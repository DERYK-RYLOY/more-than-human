package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type apiResp struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func ok(w http.ResponseWriter, data any) {
	writeJSON(w, 200, apiResp{OK: true, Data: data})
}

func fail(w http.ResponseWriter, err error) {
	writeJSON(w, 500, apiResp{OK: false, Error: err.Error()})
}

func qInt(r *http.Request, key string, def int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// ServeAPI exposes the whole tool over HTTP so ANY AI agent
// (Python, JS, another LLM harness) can drive the browser.
func ServeAPI(port int, cdpPort int) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ok(w, map[string]any{"cdpPort": cdpPort})
	})

	// GET /see?tab=&hidden=true&shot=page.png  -> screenshot + elements + text + url
	mux.HandleFunc("/see", func(w http.ResponseWriter, r *http.Request) {
		tab := r.URL.Query().Get("tab")
		hidden := r.URL.Query().Get("hidden") != "false"
		c, err := DialTab(cdpPort, tab)
		if err != nil {
			fail(w, err)
			return
		}
		defer c.Close()
		shot, err := PageScreenshot(c, false)
		if err != nil {
			fail(w, err)
			return
		}
		els, err := GetElements(c, hidden)
		if err != nil {
			fail(w, err)
			return
		}
		title, _ := EvalValue(c, "document.title")
		url, _ := EvalValue(c, "location.href")
		text, _ := PageText(c)
		out := r.URL.Query().Get("shot")
		if out != "" {
			savePNG(shot, out)
		}
		ok(w, map[string]any{
			"title":    title,
			"url":      url,
			"text":     truncate(str(fmt.Sprint(text)), 8000),
			"elements": els,
			"count":    len(els),
			"shotB64":  base64.StdEncoding.EncodeToString(shot),
		})
	})

	// GET /screen?out=screen.png -> OS screenshot (browser chrome + everything AI sees as human)
	mux.HandleFunc("/screen", func(w http.ResponseWriter, r *http.Request) {
		shot, err := OSScreenshot()
		if err != nil {
			fail(w, err)
			return
		}
		if out := r.URL.Query().Get("out"); out != "" {
			savePNG(shot, out)
		}
		sw, sh := ScreenSize()
		ok(w, map[string]any{"w": sw, "h": sh, "shotB64": base64.StdEncoding.EncodeToString(shot)})
	})

	mux.HandleFunc("/tabs", func(w http.ResponseWriter, r *http.Request) {
		tabs, err := ListTabs(cdpPort)
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, tabs)
	})

	mux.HandleFunc("/navigate", func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Query().Get("url")
		tab := r.URL.Query().Get("tab")
		c, err := DialTab(cdpPort, tab)
		if err != nil {
			fail(w, err)
			return
		}
		defer c.Close()
		if err := PageNavigate(c, url); err != nil {
			fail(w, err)
			return
		}
		ok(w, map[string]any{"navigated": url})
	})

	// POST /act  {"action":"click","x":..,"y":..} etc.
	mux.HandleFunc("/act", func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			fail(w, err)
			return
		}
		res, err := doAction(cdpPort, req)
		if err != nil {
			fail(w, err)
			return
		}
		ok(w, res)
	})

	mux.HandleFunc("/windows", func(w http.ResponseWriter, r *http.Request) {
		ok(w, ListWindows())
	})

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	fmt.Printf("AI API listening on http://%s  (cdp=%d)\n", addr, cdpPort)
	fmt.Println("  GET  /see?hidden=true&shot=page.png")
	fmt.Println("  GET  /screen?out=screen.png")
	fmt.Println("  GET  /tabs   /navigate?url=...   /windows")
	fmt.Println("  POST /act   {\"action\":\"click\",\"x\":100,\"y\":200}")
	return http.ListenAndServe(addr, mux)
}

func doAction(cdpPort int, req map[string]any) (any, error) {
	action, _ := req["action"].(string)
	tab, _ := req["tab"].(string)
	num := func(k string) float64 {
		switch v := req[k].(type) {
		case float64:
			return v
		case int:
			return float64(v)
		case string:
			f, _ := strconv.ParseFloat(v, 64)
			return f
		}
		return 0
	}
	str := func(k string) string {
		if s, ok := req[k].(string); ok {
			return s
		}
		return ""
	}
	switch strings.ToLower(action) {
	case "click":
		c, err := DialTab(cdpPort, tab)
		if err != nil {
			return nil, err
		}
		defer c.Close()
		return map[string]any{"clicked": []float64{num("x"), num("y")}}, HumanClick(c, num("x"), num("y"))
	case "jsclick":
		c, err := DialTab(cdpPort, tab)
		if err != nil {
			return nil, err
		}
		defer c.Close()
		return map[string]any{"jsclick": int(num("index"))}, JSClickByIndex(c, int(num("index")))
	case "type":
		c, err := DialTab(cdpPort, tab)
		if err != nil {
			return nil, err
		}
		defer c.Close()
		return map[string]any{"typed": str("text")}, TypeText(c, str("text"))
	case "press":
		c, err := DialTab(cdpPort, tab)
		if err != nil {
			return nil, err
		}
		defer c.Close()
		return map[string]any{"pressed": str("key")}, PressKey(c, str("key"), str("code"))
	case "hover":
		c, err := DialTab(cdpPort, tab)
		if err != nil {
			return nil, err
		}
		defer c.Close()
		return map[string]any{"hover": []float64{num("x"), num("y")}}, HumanHover(c, num("x"), num("y"))
	case "scroll":
		c, err := DialTab(cdpPort, tab)
		if err != nil {
			return nil, err
		}
		defer c.Close()
		dy := num("dy")
		if dy == 0 {
			dy = -600
		}
		return map[string]any{"scrolled": dy}, HumanScroll(c, num("x"), num("y"), dy)
	case "eval":
		c, err := DialTab(cdpPort, tab)
		if err != nil {
			return nil, err
		}
		defer c.Close()
		v, err := EvalValue(c, str("js"))
		return map[string]any{"value": v}, err
	case "mouse":
		sub := str("sub")
		switch sub {
		case "move":
			return map[string]any{"moved": []float64{num("x"), num("y")}}, OSMouseMoveSmooth(int(num("x")), int(num("y")))
		case "pos":
			cx, cy, err := OSMousePos()
			if err != nil {
				return nil, err
			}
			w, h := ScreenSize()
			return map[string]any{"x": cx, "y": cy, "screenW": w, "screenH": h}, nil
		case "drag":
			return map[string]any{"dragged": true}, OSMouseDrag(int(num("x1")), int(num("y1")), int(num("x2")), int(num("y2")))
		case "scroll":
			return map[string]any{"scrolled": int(num("clicks"))}, OSMouseScroll(int(num("clicks")))
		default:
			btn := str("button")
			if btn == "" {
				btn = "left"
			}
			err := OSMouseClick(btn, int(num("x")), int(num("y")))
			cx, cy, _ := OSMousePos()
			return map[string]any{"osclick": btn, "x": cx, "y": cy}, err
		}
	case "key":
		if str("combo") != "" {
			return map[string]any{"hotkey": str("combo")}, OSHotkey(str("combo"))
		}
		if str("text") != "" {
			return map[string]any{"typed": str("text")}, OSTypeText(str("text"), 0)
		}
		return map[string]any{"pressed": str("key")}, OSKeyPress(str("key"), 0)
	case "focus":
		return map[string]any{"focused": str("title")}, FocusWindowByTitle(str("title"))
	default:
		return nil, fmt.Errorf("unknown action %q (click/jsclick/type/press/hover/scroll/eval/mouse/key/focus)", action)
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func str(v string) string { return v }
