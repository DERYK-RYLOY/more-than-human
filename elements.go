package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// UIElement is what the AI "sees". Coordinates are in CSS pixels
// relative to the page viewport (same space CDP Input uses).
type UIElement struct {
	Index      int     `json:"index"`
	Tag        string  `json:"tag"`
	Role       string  `json:"role,omitempty"`
	Type       string  `json:"type,omitempty"`
	Text       string  `json:"text,omitempty"`
	Name       string  `json:"name,omitempty"`
	Selector   string  `json:"selector"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	W          float64 `json:"w"`
	H          float64 `json:"h"`
	CX         float64 `json:"cx"`
	CY         float64 `json:"cy"`
	Visible    bool    `json:"visible"`
	InViewport bool    `json:"inViewport"`
	Clickable  bool    `json:"clickable"`
	HiddenWhy  string  `json:"hiddenWhy,omitempty"`
	Href       string  `json:"href,omitempty"`
}

// ElementsJS walks the whole DOM and reports EVERYTHING, including
// display:none / visibility:hidden / zero-size / offscreen / disabled.
const ElementsJS = `(includeHidden) => {
  const out = [];
  const els = document.querySelectorAll('*');
  const vw = window.innerWidth, vh = window.innerHeight;
  let idx = 0;
  for (const el of els) {
    const tag = (el.tagName || '').toLowerCase();
    if (tag === 'script' || tag === 'style' || tag === 'noscript' || tag === 'template') continue;
    const cs = getComputedStyle(el);
    const r = el.getBoundingClientRect();
    const w = r.width, h = r.height;
    const displayNone = cs.display === 'none';
    const visHidden = cs.visibility === 'hidden' || cs.visibility === 'collapse';
    const opacity0 = parseFloat(cs.opacity || '1') === 0;
    const zeroSize = w === 0 || h === 0;
    const offscreen = (r.bottom < 0 || r.top > vh || r.right < 0 || r.left > vw);
    const hiddenAttr = el.hasAttribute('hidden');
    const ariaHidden = el.getAttribute('aria-hidden') === 'true';
    const visible = !(displayNone || visHidden || opacity0 || hiddenAttr || ariaHidden) && !zeroSize;
    let why = '';
    if (displayNone) why = 'display:none';
    else if (visHidden) why = 'visibility:' + cs.visibility;
    else if (opacity0) why = 'opacity:0';
    else if (hiddenAttr) why = 'hidden-attr';
    else if (ariaHidden) why = 'aria-hidden';
    else if (zeroSize) why = 'zero-size';
    else if (offscreen) why = 'offscreen';
    if (!includeHidden && !visible) continue;
    // text: own + aria
    let text = (el.innerText || '').trim().replace(/\s+/g, ' ');
    if (text.length > 120) text = text.slice(0, 120);
    const aria = (el.getAttribute('aria-label') || '').trim();
    const val = (el.value !== undefined && (tag === 'input' || tag === 'textarea' || tag === 'select')) ? String(el.value).slice(0,80) : '';
    const clickable = !!el.closest('a,button,[role=button],input,select,textarea,[onclick],[tabindex]') || tag==='a' || tag==='button';
    // cheap unique selector
    let sel = tag;
    if (el.id) sel += '#' + CSS.escape(el.id);
    else {
      if (el.className && typeof el.className === 'string') {
        const c = el.className.trim().split(/\s+/).slice(0,2).map(x=>CSS.escape(x)).join('.');
        if (c) sel += '.' + c;
      }
      const sib = el.parentElement ? Array.from(el.parentElement.children).indexOf(el) : 0;
      sel += ':nth(' + sib + ')';
    }
    out.push({
      index: idx++,
      tag, role: (el.getAttribute('role')||''),
      type: (el.getAttribute('type')||''),
      text: text || val || aria || (el.getAttribute('placeholder')||'') || (el.getAttribute('alt')||'') || el.tagName,
      name: aria || (el.getAttribute('name')||''),
      selector: sel,
      x: r.x, y: r.y, w, h,
      cx: r.x + w/2, cy: r.y + h/2,
      visible, inViewport: !offscreen,
      clickable, hiddenWhy: why,
      href: (el.href && typeof el.href === 'string') ? String(el.href).slice(0,300) : ''
    });
    if (out.length >= 1500) break;
  }
  return out;
}`

func GetElements(c *Client, includeHidden bool) ([]UIElement, error) {
	js := fmt.Sprintf("(%s)(%v)", ElementsJS, boolStr(includeHidden))
	res, err := c.Send("Runtime.evaluate", map[string]any{
		"expression":    js,
		"returnByValue": true,
	})
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Result struct {
			Value []UIElement `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(res, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Result.Value, nil
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// FindElements filters by free text query (matches text/name/selector/tag).
func FindElements(els []UIElement, query string) []UIElement {
	q := strings.ToLower(query)
	var out []UIElement
	for _, e := range els {
		hay := strings.ToLower(e.Text + " " + e.Name + " " + e.Selector + " " + e.Tag + " " + e.Role + " " + e.Href)
		if strings.Contains(hay, q) {
			out = append(out, e)
		}
	}
	return out
}

// HumanClick clicks at page coordinates using real CDP Input events
// (mouseMoved + mousePressed + mouseReleased) so sites see a real mouse.
func HumanClick(c *Client, x, y float64) error {
	if _, err := c.Send("Input.dispatchMouseEvent", map[string]any{
		"type": "mouseMoved", "x": x, "y": y, "button": "none",
	}); err != nil {
		return err
	}
	if _, err := c.Send("Input.dispatchMouseEvent", map[string]any{
		"type": "mousePressed", "x": x, "y": y, "button": "left", "clickCount": 1,
	}); err != nil {
		return err
	}
	if _, err := c.Send("Input.dispatchMouseEvent", map[string]any{
		"type": "mouseReleased", "x": x, "y": y, "button": "left", "clickCount": 1,
	}); err != nil {
		return err
	}
	return nil
}

func HumanHover(c *Client, x, y float64) error {
	_, err := c.Send("Input.dispatchMouseEvent", map[string]any{
		"type": "mouseMoved", "x": x, "y": y, "button": "none",
	})
	return err
}

func HumanScroll(c *Client, x, y float64, deltaY float64) error {
	_, err := c.Send("Input.dispatchMouseEvent", map[string]any{
		"type": "mouseWheel", "x": x, "y": y, "deltaX": 0, "deltaY": deltaY,
	})
	return err
}

// JSClick works even on invisible / zero-size / covered elements.
func JSClickByIndex(c *Client, index int) error {
	js := fmt.Sprintf(`(() => {
	  const els = [...document.querySelectorAll('*')].filter(e => !['SCRIPT','STYLE','NOSCRIPT','TEMPLATE'].includes(e.tagName));
	  const el = els[%d];
	  if (!el) return 'not-found';
	  el.scrollIntoView({block:'center'});
	  el.focus && el.focus();
	  el.click();
	  // fire full mouse sequence for frameworks
	  for (const t of ['mousedown','mouseup']) el.dispatchEvent(new MouseEvent(t,{bubbles:true}));
	  return 'clicked:' + el.tagName;
	})()`, index)
	_, err := EvalValue(c, js)
	return err
}

func TypeText(c *Client, text string) error {
	// InsertText handles any unicode, like a real IME.
	_, err := c.Send("Input.insertText", map[string]any{"text": text})
	return err
}

func PressKey(c *Client, key string, code string) error {
	if code == "" {
		code = key
	}
	winCode := windowsKeyCode(key)
	if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": key, "code": code, "windowsVirtualKeyCode": winCode}); err != nil {
		return err
	}
	_, err := c.Send("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": key, "code": code})
	return err
}

// charKey maps a passage rune to CDP (key, code, vk). Covers everything a
// typing test throws: letters, space, digits and . , ' -
// Returns ok=false for anything exotic (caller skips or fails loudly).
func charKey(r rune) (key, code string, vk int, ok bool) {
	switch {
	case r >= 'a' && r <= 'z':
		u := string(r - 'a' + 'A')
		return string(r), "Key" + u, int(r - 'a' + 'A'), true
	case r >= 'A' && r <= 'Z':
		return string(r), "Key" + string(r), int(r), true
	case r >= '0' && r <= '9':
		return string(r), "Digit" + string(r), int(r), true
	case r == ' ':
		return " ", "Space", 32, true
	case r == ' ': // U+00A0 NO-BREAK SPACE (typing.com separators)
		return " ", "Space", 32, true
	case r == '.':
		return ".", "Period", 190, true
	case r == ',':
		return ",", "Comma", 188, true
	case r == '\'':
		return "'", "Quote", 222, true
	case r == '-':
		return "-", "Minus", 189, true
	case r == '/':
		return "/", "Slash", 191, true
	case r == '+':
		return "+", "Equal", 187, true
	case r == '*':
		return "*", "Digit8", 56, true
	case r == '=':
		return "=", "Equal", 187, true
	case r == '(':
		return "(", "Digit9", 57, true
	case r == ')':
		return ")", "Digit0", 48, true
	case r == '"':
		return "\"", "Quote", 222, true
	case r == ';':
		return ";", "Semicolon", 186, true
	case r == ':':
		return ":", "Semicolon", 186, true
	case r == '?':
		return "?", "Slash", 191, true
	case r == '!':
		return "!", "Digit1", 49, true
	}
	return "", "", 0, false
}

// needsShift reports chars a human types with Shift held. The site checks
// shift state for these (a bare Quote event for `"` is marked wrong).
func needsShift(r rune) bool {
	if r >= 'A' && r <= 'Z' {
		return true
	}
	switch r {
	case '"', '?', '!', ':', '+', '_', '<', '>', '|', '~', '@', '#', '$', '%', '^', '&', '*', '(', ')':
		return true
	}
	return false
}

// CTypeResult is the self-verifying report: cursor index + error count read
// back from the DOM, so no screenshot is ever needed to confirm.
type CTypeResult struct {
	Sent   int   `json:"sent"`
	Ms     int64 `json:"ms"`
	Cursor int   `json:"cursor"`
	Errors int   `json:"errors"`
}

// CTypeText presses a whole passage over ONE CDP connection at wpm speed,
// then reads back cursor + errors from div.letter markers.
// Capitals ride on key value alone (no Shift dance needed for e.key checks).
// maxChars > 0 stops after that many runes (timed tests: never spill keys
// onto the results screen after the timer ends the test).
func CTypeText(c *Client, text string, wpm int) (CTypeResult, error) {
	return CTypeTextMax(c, text, wpm, 0)
}

func CTypeTextMax(c *Client, text string, wpm int, maxChars int) (CTypeResult, error) {
	var res CTypeResult
	if wpm <= 0 {
		wpm = 45
	}
	gap := time.Second * 60 / (time.Duration(wpm) * 5) // 5 chars = 1 word
	t0 := time.Now()
	for _, r := range text {
		if maxChars > 0 && res.Sent >= maxChars {
			break
		}
		// ⏎ (U+23CE) is the site's "press ENTER for next line" marker.
		if r == '⏎' {
			if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
				"type": "keyDown", "key": "Enter", "code": "Enter", "windowsVirtualKeyCode": 13,
			}); err != nil {
				return res, err
			}
			if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
				"type": "keyUp", "key": "Enter", "code": "Enter",
			}); err != nil {
				return res, err
			}
			res.Sent++
			time.Sleep(gap)
			continue
		}
		key, code, vk, ok := charKey(r)
		if !ok {
			return res, fmt.Errorf("ctype: unsupported char %q", string(r))
		}
		if needsShift(r) {
			if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
				"type": "keyDown", "key": "Shift", "code": "ShiftLeft", "windowsVirtualKeyCode": 16,
			}); err != nil {
				return res, err
			}
		}
		if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
			"type": "keyDown", "key": key, "code": code, "windowsVirtualKeyCode": vk,
		}); err != nil {
			return res, err
		}
		if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
			"type": "keyUp", "key": key, "code": code,
		}); err != nil {
			return res, err
		}
		if needsShift(r) {
			if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
				"type": "keyUp", "key": "Shift", "code": "ShiftLeft",
			}); err != nil {
				return res, err
			}
		}
		res.Sent++
		time.Sleep(gap)
	}
	res.Ms = time.Since(t0).Milliseconds()
	cur, cerr := LetterCursor(c)
	if cerr != nil {
		return res, cerr
	}
	res.Cursor = cur.Index
	res.Errors = cur.Errors
	return res, nil
}

// Passage is the site's own mind: every expected char (including future
// words), the live cursor index and the live error count. No screenshots,
// no transcription, no guessing.
type Passage struct {
	Text   string `json:"text"`
	Cursor int    `json:"cursor"`
	Errors int    `json:"errors"`
	Total  int    `json:"total"`
}

func ExtractPassage(c *Client) (Passage, error) {
	var p Passage
	res, err := c.Send("Runtime.evaluate", map[string]any{
		"expression": `(() => { const ds=[...document.querySelectorAll('div.letter')]; let cur=-1,err=0; const t=ds.map(d=>{ const cl=String(d.className); return d.textContent; }); ds.forEach((d,i)=>{ const cl=String(d.className); if(/is-act/i.test(cl))cur=i; if(/wrong|error|incorrect/i.test(cl))err++; }); return {text:t.join(''),cur,err,n:ds.length}; })()`,
		"returnByValue": true,
	})
	if err != nil {
		return p, err
	}
	var w struct {
		Result struct {
			Value struct {
				Text string `json:"text"`
				Cur  int    `json:"cur"`
				Err  int    `json:"err"`
				N    int    `json:"n"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(res, &w); err != nil {
		return p, err
	}
	p.Text, p.Cursor, p.Errors, p.Total = w.Result.Value.Text, w.Result.Value.Cur, w.Result.Value.Err, w.Result.Value.N
	return p, nil
}

// TPassReport is the single-call verdict: what was typed, measured pace,
// and DOM-verified cursor/errors. Future AIs: one call replaces ~10.
type TPassReport struct {
	Sent         int     `json:"sent"`
	Ms           int64   `json:"ms"`
	EffectiveWPM float64 `json:"effectiveWPM"`
	Cursor       int     `json:"cursor"`
	Errors       int     `json:"errors"`
	Total        int     `json:"total"`
	Aborted      bool    `json:"aborted"`
	AbortWhy     string  `json:"abortWhy,omitempty"`
}

// TPass runs a whole perfect pass in ONE connection: extract ground truth,
// refuse damaged tests, validate every rune, type at wpm with an error gate
// every 100 keys (aborts loudly instead of cascading), verify at the end.
// maxChars>0 stops early (timed tests). dry=true extracts+validates only.
func TPass(c *Client, wpm int, maxChars int, dry bool) (TPassReport, error) {
	var rep TPassReport
	p, err := ExtractPassage(c)
	if err != nil {
		return rep, err
	}
	rep.Total, rep.Cursor, rep.Errors = p.Total, p.Cursor, p.Errors
	if p.Total == 0 {
		return rep, fmt.Errorf("tpass: no test loaded (intro screen? click Continue first)")
	}
	if p.Cursor != 0 || p.Errors != 0 {
		rep.Aborted = true
		rep.AbortWhy = fmt.Sprintf("test not fresh (cursor=%d errors=%d): Redo first", p.Cursor, p.Errors)
		return rep, nil
	}
	runes := []rune(p.Text)
	if maxChars > 0 && len(runes) > maxChars {
		runes = runes[:maxChars]
	}
	for _, r := range runes {
		if r == '⏎' || r == '\n' {
			continue // supported (Enter branch below handles it)
		}
		if _, _, _, ok := charKey(r); !ok {
			return rep, fmt.Errorf("tpass: unsupported char %q — extend charKey first", string(r))
		}
	}
	if dry {
		return rep, nil
	}
	if wpm <= 0 {
		wpm = 50
	}
	gap := time.Second * 60 / (time.Duration(wpm) * 5)
	t0 := time.Now()
	sent := 0
	for i, r := range runes {
		if r == '⏎' || r == '\n' {
			if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
				"type": "keyDown", "key": "Enter", "code": "Enter", "windowsVirtualKeyCode": 13,
			}); err != nil {
				return rep, err
			}
			if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
				"type": "keyUp", "key": "Enter", "code": "Enter",
			}); err != nil {
				return rep, err
			}
		} else {
			key, code, vk, _ := charKey(r)
			if needsShift(r) {
				if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
					"type": "keyDown", "key": "Shift", "code": "ShiftLeft", "windowsVirtualKeyCode": 16,
				}); err != nil {
					return rep, err
				}
			}
			if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
				"type": "keyDown", "key": key, "code": code, "windowsVirtualKeyCode": vk,
			}); err != nil {
				return rep, err
			}
			if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
				"type": "keyUp", "key": key, "code": code,
			}); err != nil {
				return rep, err
			}
			if needsShift(r) {
				if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{
					"type": "keyUp", "key": "Shift", "code": "ShiftLeft",
				}); err != nil {
					return rep, err
				}
			}
		}
		sent = i + 1
		time.Sleep(gap)
		if sent%100 == 0 {
			st, err := LetterCursor(c)
			if err != nil {
				return rep, err
			}
			if st.Errors > 0 {
				rep.Sent, rep.Ms, rep.Cursor, rep.Errors = sent, time.Since(t0).Milliseconds(), st.Index, st.Errors
				rep.Aborted, rep.AbortWhy = true, fmt.Sprintf("error gate tripped at key %d (errors=%d): Redo and restart", sent, st.Errors)
				return rep, nil
			}
		}
	}
	rep.Sent, rep.Ms = sent, time.Since(t0).Milliseconds()
	if rep.Ms > 0 {
		rep.EffectiveWPM = float64(sent) / 5 / (float64(rep.Ms) / 60000)
	}
	st, err := LetterCursor(c)
	if err != nil {
		return rep, err
	}
	rep.Cursor, rep.Errors = st.Index, st.Errors
	return rep, nil
}

// FoundEl is an exact-coordinate answer: no more eyeballing screenshots.
type FoundEl struct {
	Text    string  `json:"text"`
	Tag     string  `json:"tag"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	CX      float64 `json:"cx"`
	CY      float64 `json:"cy"`
	Visible bool    `json:"visible"`
}

// FindExact asks the page where matching controls are (text/aria/placeholder)
// and returns their viewport boxes. Pair with click --x --y for pixel truth.
func FindExact(c *Client, query string) ([]FoundEl, error) {
	qb, _ := json.Marshal(query)
	res, err := c.Send("Runtime.evaluate", map[string]any{
		"expression": `((q) => { const out=[]; const els=document.querySelectorAll('a,button,input,select,textarea,[role=button],[onclick],[tabindex],h1,h2,h3'); const ql=q.toLowerCase(); for (const el of els) { const t=((el.innerText||'')+' '+(el.getAttribute('aria-label')||'')+' '+(el.getAttribute('placeholder')||'')+' '+(el.value||'')).trim().replace(/\s+/g,' '); if (!t.toLowerCase().includes(ql)) continue; const r=el.getBoundingClientRect(); if (r.width===0&&r.height===0) continue; out.push({text:t.slice(0,80), tag:(el.tagName||'').toLowerCase(), x:r.x, y:r.y, w:r.width, h:r.height, cx:r.x+r.width/2, cy:r.y+r.height/2, visible:getComputedStyle(el).display!=='none'}); if (out.length>=50) break; } return out; })(` + string(qb) + `)`,
		"returnByValue": true,
	})
	if err != nil {
		return nil, err
	}
	var w struct {
		Result struct {
			Value []FoundEl `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(res, &w); err != nil {
		return nil, err
	}
	return w.Result.Value, nil
}

// RedoTest clicks the test's Redo button (fresh start for self-heal).
func RedoTest(c *Client) error {
	v, err := EvalValue(c, `(() => { const b=[...document.querySelectorAll('button,a')].find(e=>(e.innerText||'').trim()==='Redo'); if(!b) return 'NOREDO'; b.click(); return 'redo-ok'; })()`)
	if err != nil {
		return err
	}
	if s, _ := v.(string); s != "redo-ok" {
		return fmt.Errorf("redo button not found")
	}
	return nil
}

// BenchReport holds rerunnable live measurements (no estimates, no vibes).
type BenchReport struct {
	PingMs    float64 `json:"pingMsAvg"`
	KeysPerS  float64 `json:"keysPerSec"`
	KeysMs    int64   `json:"keysMs"`
	Keys      int     `json:"keys"`
	Letters   int     `json:"letters"`
	ExtractMs int64   `json:"extractMs"`
	ShotMs    int64   `json:"shotMs"`
	TabsMs    int64   `json:"tabsMs"`
}

// BenchRun measures real CDP round-trips against a live tab. Rerun it
// yourself: launch --headless, load any page, bench --port. Same method
// every time, so the numbers in BENCHMARKS.md are checkable, not marketing.
func BenchRun(port int, tab string, keys int) (BenchReport, error) {
	var r BenchReport
	if keys <= 0 {
		keys = 20
	}
	r.Keys = keys
	t0 := time.Now()
	tabs, err := ListTabs(port)
	if err != nil {
		return r, err
	}
	_ = tabs
	r.TabsMs = time.Since(t0).Milliseconds()
	c, err := DialTab(port, tab)
	if err != nil {
		return r, err
	}
	defer c.Close()
	t0 = time.Now()
	for i := 0; i < 10; i++ {
		if _, err := c.Send("Runtime.evaluate", map[string]any{"expression": "1", "returnByValue": true}); err != nil {
			return r, err
		}
	}
	r.PingMs = float64(time.Since(t0).Microseconds()) / 10 / 1000
	t0 = time.Now()
	for i := 0; i < keys; i++ {
		if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "a", "code": "KeyA", "windowsVirtualKeyCode": 65}); err != nil {
			return r, err
		}
		if _, err := c.Send("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": "a", "code": "KeyA"}); err != nil {
			return r, err
		}
	}
	r.KeysMs = time.Since(t0).Milliseconds()
	if r.KeysMs > 0 {
		r.KeysPerS = float64(keys) * 1000 / float64(r.KeysMs)
	}
	t0 = time.Now()
	p, err := ExtractPassage(c)
	if err != nil {
		return r, err
	}
	r.ExtractMs = time.Since(t0).Milliseconds()
	r.Letters = p.Total
	t0 = time.Now()
	if _, err := PageScreenshot(c, false); err != nil {
		return r, err
	}
	r.ShotMs = time.Since(t0).Milliseconds()
	return r, nil
}

// LetterState reads typing-test progress straight from the DOM.
type LetterState struct {
	Index  int `json:"cursor"`
	Errors int `json:"errors"`
	Total  int `json:"total"`
}

func LetterCursor(c *Client) (LetterState, error) {
	var st LetterState
	res, err := c.Send("Runtime.evaluate", map[string]any{
		"expression":    `(() => { const ds=[...document.querySelectorAll('div.letter')]; let cur=-1,err=0; ds.forEach((d,i)=>{ const cl=String(d.className); if(/is-act/i.test(cl))cur=i; if(/wrong|error|incorrect/i.test(cl))err++; }); return {cur,err,n:ds.length}; })()`,
		"returnByValue": true,
	})
	if err != nil {
		return st, err
	}
	var w struct {
		Result struct {
			Value struct {
				Cur int `json:"cur"`
				Err int `json:"err"`
				N   int `json:"n"`
			} `json:"value"`
		} `json:"result"`
	}
	if err := json.Unmarshal(res, &w); err != nil {
		return st, err
	}
	st.Index, st.Errors, st.Total = w.Result.Value.Cur, w.Result.Value.Err, w.Result.Value.N
	return st, nil
}

func windowsKeyCode(key string) int {	switch key {
	case "Enter":
		return 13
	case "Tab":
		return 9
	case "Escape":
		return 27
	case "Backspace":
		return 8
	case "Delete":
		return 46
	case "ArrowLeft":
		return 37
	case "ArrowUp":
		return 38
	case "ArrowRight":
		return 39
	case "ArrowDown":
		return 40
	case " ":
		return 32
	default:
		if len(key) == 1 {
			ch := key[0]
			if ch >= 'a' && ch <= 'z' {
				return int(ch - 'a' + 'A')
			}
			if ch >= 'A' && ch <= 'Z' {
				return int(ch)
			}
			if ch >= '0' && ch <= '9' {
				return int(ch)
			}
		}
		return 0
	}
}

func PageText(c *Client) (string, error) {
	v, err := EvalValue(c, "document.body ? document.body.innerText.slice(0,20000) : ''")
	if err != nil {
		return "", err
	}
	if s, ok := v.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", v), nil
}
