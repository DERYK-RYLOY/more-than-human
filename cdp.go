package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

func urlQueryEscape(s string) string { return url.QueryEscape(s) }

// ---------- CDP tab discovery (HTTP) ----------

type TabInfo struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	Type                 string `json:"type"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func cdpBase(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func ListTabs(port int) ([]TabInfo, error) {
	resp, err := http.Get(cdpBase(port) + "/json/list")
	if err != nil {
		return nil, fmt.Errorf("is browser running with --remote-debugging-port=%d? %w", port, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tabs []TabInfo
	if err := json.Unmarshal(body, &tabs); err != nil {
		return nil, err
	}
	return tabs, nil
}

func NewTab(port int, url string) (*TabInfo, error) {
	endpoint := cdpBase(port) + "/json/new"
	if url != "" {
		endpoint += "?url=" + urlQueryEscape(url)
	}
	// CDP expects PUT
	req, _ := http.NewRequest(http.MethodPut, endpoint, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var t TabInfo
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("new tab: %s: %w", string(body), err)
	}
	return &t, nil
}

func CloseTab(port int, id string) error {
	endpoint := cdpBase(port) + "/json/close/" + id
	req, _ := http.NewRequest(http.MethodGet, endpoint, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func ActivateTab(port int, id string) error {
	endpoint := cdpBase(port) + "/json/activate/" + id
	resp, err := http.Get(endpoint)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func ResolveWSTarget(port int, tabID string) (string, error) {
	tabs, err := ListTabs(port)
	if err != nil {
		return "", err
	}
	if tabID == "" {
		// first page tab
		for _, t := range tabs {
			if t.Type == "page" {
				return t.WebSocketDebuggerURL, nil
			}
		}
		if len(tabs) > 0 {
			return tabs[0].WebSocketDebuggerURL, nil
		}
		return "", fmt.Errorf("no tabs found")
	}
	for _, t := range tabs {
		if t.ID == tabID {
			return t.WebSocketDebuggerURL, nil
		}
	}
	return "", fmt.Errorf("tab %q not found", tabID)
}

// ---------- CDP WebSocket client ----------

type cdpMsg struct {
	ID     int64           `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *cdpError       `json:"error,omitempty"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type Client struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	nextID  int64
	pending map[int64]chan cdpMsg
}

func DialWS(wsURL string) (*Client, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	c := &Client{conn: conn, nextID: 1, pending: map[int64]chan cdpMsg{}}
	go c.readLoop()
	return c, nil
}

func (c *Client) readLoop() {
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var m cdpMsg
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		if m.ID == 0 {
			continue // browser event, ignore for now
		}
		c.mu.Lock()
		ch, ok := c.pending[m.ID]
		c.mu.Unlock()
		if ok {
			ch <- m
		}
	}
}

func (c *Client) Send(method string, params any) (json.RawMessage, error) {
	var raw json.RawMessage
	if params != nil {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		raw = b
	}
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	ch := make(chan cdpMsg, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	req := map[string]any{"id": id, "method": method}
	if raw != nil {
		req["params"] = json.RawMessage(raw)
	}
	if err := c.conn.WriteJSON(req); err != nil {
		return nil, err
	}
	select {
	case m := <-ch:
		if m.Error != nil {
			return nil, fmt.Errorf("cdp %s: %s (code %d)", method, m.Error.Message, m.Error.Code)
		}
		return m.Result, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("cdp %s timed out", method)
	}
}

func (c *Client) Close() { c.conn.Close() }

func DialTab(port int, tabID string) (*Client, error) {
	wsURL, err := ResolveWSTarget(port, tabID)
	if err != nil {
		return nil, err
	}
	return DialWS(wsURL)
}

// DialBrowser connects to the browser-level endpoint (for Browser domain).
func DialBrowser(port int) (*Client, error) {
	resp, err := http.Get(cdpBase(port) + "/json/version")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var v struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	if v.WebSocketDebuggerURL == "" {
		return nil, fmt.Errorf("no browser debugger url (port %d)", port)
	}
	return DialWS(v.WebSocketDebuggerURL)
}

// SetDownloadDir routes all browser downloads into dir.
func SetDownloadDir(port int, dir string) error {
	c, err := DialBrowser(port)
	if err != nil {
		return err
	}
	defer c.Close()
	_, err = c.Send("Browser.setDownloadBehavior", map[string]any{
		"behavior": "allow", "downloadPath": dir,
	})
	return err
}

// ---------- High level page ops ----------

func PageNavigate(c *Client, url string) error {
	_, err := c.Send("Page.navigate", map[string]any{"url": url})
	return err
}

func PageReload(c *Client) error {
	_, err := c.Send("Page.reload", map[string]any{})
	return err
}

func PageBack(c *Client) error {
	_, err := c.Send("Runtime.evaluate", map[string]any{"expression": "history.back()", "awaitPromise": false})
	return err
}

func PageForward(c *Client) error {
	_, err := c.Send("Runtime.evaluate", map[string]any{"expression": "history.forward()", "awaitPromise": false})
	return err
}

func PageScreenshot(c *Client, fullPage bool) ([]byte, error) {
	params := map[string]any{"format": "png"}
	if fullPage {
		params["captureBeyondViewport"] = true
		params["fromSurface"] = true
	}
	res, err := c.Send("Page.captureScreenshot", params)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(res, &out); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(out.Data)
}

func RuntimeEval(c *Client, js string, awaitPromise bool) (json.RawMessage, error) {
	return c.Send("Runtime.evaluate", map[string]any{
		"expression":    js,
		"returnByValue": true,
		"awaitPromise":  awaitPromise,
	})
}

// EvalValue evaluates JS and returns the .value decoded into any.
func EvalValue(c *Client, js string) (any, error) {
	res, err := RuntimeEval(c, js, false)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(res, &wrapper); err != nil {
		return nil, err
	}
	var v any
	if len(wrapper.Result.Value) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(wrapper.Result.Value, &v); err != nil {
		return string(wrapper.Result.Value), nil
	}
	return v, nil
}

func savePNG(data []byte, path string) error {
	if path == "" {
		return nil
	}
	return os.WriteFile(path, data, 0644)
}
