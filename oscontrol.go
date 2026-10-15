//go:build windows

package main

import (
	"bytes"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/kbinani/screenshot"
	"golang.org/x/sys/windows"
)

var (
	user32             = windows.NewLazySystemDLL("user32.dll")
	shcore             = windows.NewLazySystemDLL("shcore.dll")
	procSetCursorPos   = user32.NewProc("SetCursorPos")
	procGetCursorPos   = user32.NewProc("GetCursorPos")
	procSendInput      = user32.NewProc("SendInput")
	procGetSysMetrics  = user32.NewProc("GetSystemMetrics")
	procSetForeground  = user32.NewProc("SetForegroundWindow")
	procBringToTop     = user32.NewProc("BringWindowToTop")
	procShowWindow     = user32.NewProc("ShowWindow")
	procFindWindowW    = user32.NewProc("FindWindowW")
	procEnumWindows    = user32.NewProc("EnumWindows")
	procGetWindowTextW = user32.NewProc("GetWindowTextW")
	procIsVisible      = user32.NewProc("IsWindowVisible")
	procGetWindowRect  = user32.NewProc("GetWindowRect")
	procDPIAware       = user32.NewProc("SetProcessDPIAware")
	procDPIAwareness   = shcore.NewProc("SetProcessDpiAwareness")
	procGetForeground  = user32.NewProc("GetForegroundWindow")
	procGetThread      = user32.NewProc("GetWindowThreadProcessId")
	procAttachInput    = user32.NewProc("AttachThreadInput")
	procPostMsg        = user32.NewProc("PostMessageW")
	procEnumChild      = user32.NewProc("EnumChildWindows")
	procGetClassName   = user32.NewProc("GetClassNameW")
)

const (
	wmKeyDown = 0x0100
	wmKeyUp   = 0x0101
	wmChar    = 0x0102
	wmClose   = 0x0010
)

// SendTextToWindow posts characters DIRECTLY to a window (and all its children)
// so keystrokes land in Brave even when the terminal is in front.
// Returns (charsSent, elapsedMs). This is the AI speed-race primitive.
func SendTextToWindow(substr string, text string, charDelayMs int) (int, int64, error) {
	hwnd, _, err := findWindowByTitle(substr)
	if err != nil {
		return 0, 0, err
	}
	// collect target + all descendants (Chrome routes input to renderer child)
	targets := []uintptr{hwnd}
	cb := syscall.NewCallback(func(child uintptr, _ uintptr) uintptr {
		targets = append(targets, child)
		return 1
	})
	// EnumChildWindows recurses? No — only direct children. Walk manually:
	var walk func(parent uintptr)
	walk = func(parent uintptr) {
		direct := []uintptr{}
		cb2 := syscall.NewCallback(func(child uintptr, _ uintptr) uintptr {
			direct = append(direct, child)
			return 1
		})
		procEnumChild.Call(parent, cb2, 0)
		for _, c := range direct {
			targets = append(targets, c)
			walk(c)
		}
	}
	_ = cb
	walk(hwnd)
	t0 := time.Now()
	sent := 0
	for _, r := range text {
		if r == '\n' {
			postKey(targets, 0x0D)
		} else if r < 0x10000 {
			postChar(targets, uintptr(r))
		} else {
			// supplementary: send surrogate pair as two WM_CHARs
			r -= 0x10000
			postChar(targets, uintptr(0xD800+(r>>10)))
			postChar(targets, uintptr(0xDC00+(r&0x3FF)))
			sent++ // count pair as one keystroke extra
		}
		sent++
		if charDelayMs > 0 {
			time.Sleep(time.Duration(charDelayMs) * time.Millisecond)
		}
	}
	return sent, time.Since(t0).Milliseconds(), nil
}

func postChar(targets []uintptr, ch uintptr) {
	for _, t := range targets {
		procPostMsg.Call(t, wmChar, ch, 1)
	}
}

func postKey(targets []uintptr, vk uintptr) {
	for _, t := range targets {
		procPostMsg.Call(t, wmKeyDown, vk, 1)
	}
	for _, t := range targets {
		procPostMsg.Call(t, wmKeyUp, vk, 1)
	}
}

// ShotClean minimizes the terminal, waits, screenshots Brave clean — all in
// ONE call so the UI can't steal foreground mid-way. Returns PNG bytes.
func ShotClean(hideTitle string) ([]byte, error) {
	if hideTitle != "" {
		if hwnd, _, err := findWindowByTitle(hideTitle); err == nil {
			procShowWindow.Call(hwnd, 6) // SW_MINIMIZE
		}
	}
	time.Sleep(500 * time.Millisecond)
	return OSScreenshot()
}

// RaceType foregrounds the target, pauses, then types via REAL SendInput at
// full speed — one call, no focus steal mid-burst. Returns elapsed ms.
func RaceType(substr string, text string) (int64, error) {
	hwnd, _, err := findWindowByTitle(substr)
	if err != nil {
		return 0, err
	}
	procShowWindow.Call(hwnd, 9) // restore if minimized
	forceForeground(hwnd)
	procBringToTop.Call(hwnd)
	time.Sleep(350 * time.Millisecond) // let user SEE Brave snap front
	t0 := time.Now()
	// ENTER must go as VK_RETURN keydown/up, not a unicode newline.
	if text == "\n" || text == "\r" || text == "<enter>" {
		if err := OSKeyPressFast("enter"); err != nil {
			return 0, err
		}
		return time.Since(t0).Milliseconds(), nil
	}
	// <bsN> sends N backspaces (repair key) e.g. "<bs5>".
	if strings.HasPrefix(text, "<bs") {
		n := 1
		fmt.Sscanf(text, "<bs%d>", &n)
		if n < 1 || n > 200 {
			n = 1
		}
		for i := 0; i < n; i++ {
			if err := OSKeyPressFast("backspace"); err != nil {
				return 0, err
			}
			time.Sleep(15 * time.Millisecond)
		}
		return time.Since(t0).Milliseconds(), nil
	}
	if err := OSTypeTextFast(text); err != nil {
		return 0, err
	}
	return time.Since(t0).Milliseconds(), nil
}

// RaceTypeCps is the paced human mode: same foreground guarantee, but types
// at cps characters/sec (cps=4 ≈ 48-50 WPM). No screenshots needed mid-run.
func RaceTypeCps(substr string, text string, cps int) (int64, error) {
	hwnd, _, err := findWindowByTitle(substr)
	if err != nil {
		return 0, err
	}
	procShowWindow.Call(hwnd, 9)
	forceForeground(hwnd)
	procBringToTop.Call(hwnd)
	time.Sleep(350 * time.Millisecond)
	t0 := time.Now()
	if err := OSTypeText(text, cps); err != nil {
		return 0, err
	}
	return time.Since(t0).Milliseconds(), nil
}

// ChainType types a full passage atomically: splits ONLY at spaces so chunks
// always end with a trailing space (except the last) — a missing joint space
// can never cascade into shifted errors again. One call, one foreground.
func ChainType(substr string, text string, chunk int, cps int, gapMs int) (int, int64, error) {
	hwnd, _, err := findWindowByTitle(substr)
	if err != nil {
		return 0, 0, err
	}
	procShowWindow.Call(hwnd, 9)
	forceForeground(hwnd)
	procBringToTop.Call(hwnd)
	time.Sleep(350 * time.Millisecond)
	words := strings.Fields(text)
	if chunk <= 0 {
		chunk = 60
	}
	if cps <= 0 {
		cps = 4
	}
	if gapMs < 0 {
		gapMs = 0
	}
	t0 := time.Now()
	sent := 0
	cur := ""
	flush := func(s string) error {
		if s == "" {
			return nil
		}
		if err := OSTypeText(s, cps); err != nil {
			return err
		}
		sent += len(s)
		if gapMs > 0 {
			time.Sleep(time.Duration(gapMs) * time.Millisecond)
		}
		return nil
	}
	for _, w := range words {
		if len(cur)+len(w)+1 > chunk {
			if err := flush(cur); err != nil {
				return sent, time.Since(t0).Milliseconds(), err
			}
			cur = ""
		}
		cur += w + " "
	}
	// last chunk: no trailing space (passage may continue later)
	last := strings.TrimRight(cur, " ")
	if err := flush(last); err != nil {
		return sent, time.Since(t0).Milliseconds(), err
	}
	return sent, time.Since(t0).Milliseconds(), nil
}

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")
var procGetCurrentThread = kernel32.NewProc("GetCurrentThreadId")

func init() {
	// Make OS coordinates match screenshot pixels 1:1 even on scaled displays.
	// Try Win8.1+ per-monitor awareness first, fall back to legacy.
	if procDPIAwareness.Find() == nil {
		procDPIAwareness.Call(uintptr(2)) // PROCESS_PER_MONITOR_DPI_AWARE
	} else if procDPIAware.Find() == nil {
		procDPIAware.Call()
	}
}

type pointRaw struct {
	X, Y int32
}

func OSMousePos() (int, int, error) {
	var p pointRaw
	r1, _, _ := procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if r1 == 0 {
		return 0, 0, fmt.Errorf("GetCursorPos failed")
	}
	return int(p.X), int(p.Y), nil
}

const (
	inputMouse    = 0
	inputKeyboard = 1

	mouseMove      = 0x0001
	mouseLeftDown  = 0x0002
	mouseLeftUp    = 0x0004
	mouseRightDown = 0x0008
	mouseRightUp   = 0x0010
	mouseMidDown   = 0x0020
	mouseMidUp     = 0x0040
	mouseWheel     = 0x0800

	keyUp     = 0x0002
	keyUnicode = 0x0004

	smCXScreen = 0
	smCYScreen = 1
)

type mouseInput struct {
	Dx          int32
	Dy          int32
	MouseData   uint32
	DwFlags     uint32
	Time        uint32
	ExtraInfo   uintptr
}

type keybdInput struct {
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	ExtraInfo uintptr
}

type inputUnion struct {
	Mi mouseInput
	// KEYBDINPUT overlays the same 32 bytes (it is smaller); no extra pad.
	// Total INPUT size must be 40 bytes on 64-bit: 4 type + 4 pad + 32 union.
}

type winInput struct {
	Type uint32
	_    uint32 // padding for 8-byte alignment on 64-bit
	U    inputUnion
}

func sendMouse(flags uint32, data uint32) error {
	in := winInput{Type: inputMouse}
	in.U.Mi.DwFlags = flags
	in.U.Mi.MouseData = data
	r1, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	if r1 == 0 {
		return fmt.Errorf("SendInput mouse failed: %v", err)
	}
	return nil
}

func sendKey(vk uint16, scan uint16, flags uint32) error {
	// reinterpret union as keyboard: MOUSEINPUT and KEYBDINPUT share layout size
	type kbdRaw struct {
		Vk        uint16
		Scan      uint16
		Flags     uint32
		Time      uint32
		ExtraInfo uintptr
	}
	in := winInput{Type: inputKeyboard}
	k := (*kbdRaw)(unsafe.Pointer(&in.U.Mi))
	k.Vk = vk
	k.Scan = scan
	k.Flags = flags
	r1, _, err := procSendInput.Call(1, uintptr(unsafe.Pointer(&in)), unsafe.Sizeof(in))
	if r1 == 0 {
		return fmt.Errorf("SendInput key failed: %v", err)
	}
	return nil
}

// ---------- screen ----------

func ScreenSize() (int, int) {
	w, _, _ := procGetSysMetrics.Call(smCXScreen)
	h, _, _ := procGetSysMetrics.Call(smCYScreen)
	return int(w), int(h)
}

// OSScreenshot captures ALL monitors stitched (display 0 union) or primary.
// Returns PNG bytes.
func OSScreenshot() ([]byte, error) {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		return nil, fmt.Errorf("no displays")
	}
	// stitch all displays: compute union
	bounds := screenshot.GetDisplayBounds(0)
	for i := 1; i < n; i++ {
		b := screenshot.GetDisplayBounds(i)
		bounds = bounds.Union(b)
	}
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ---------- mouse ----------

func OSMouseMove(x, y int) error {
	r1, _, _ := procSetCursorPos.Call(uintptr(x), uintptr(y))
	if r1 == 0 {
		return fmt.Errorf("SetCursorPos(%d,%d) failed", x, y)
	}
	return nil
}

// OSMouseMoveSmooth glides visibly like a human so the user can WATCH it.
func OSMouseMoveSmooth(x, y int) error {
	cx, cy, err := OSMousePos()
	if err != nil {
		return OSMouseMove(x, y)
	}
	dx := x - cx
	dy := y - cy
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	dist := dx + dy
	steps := dist / 25
	if steps < 8 {
		steps = 8
	}
	if steps > 40 {
		steps = 40
	}
	for i := 1; i <= steps; i++ {
		xi := cx + (x-cx)*i/steps
		yi := cy + (y-cy)*i/steps
		procSetCursorPos.Call(uintptr(xi), uintptr(yi))
		time.Sleep(10 * time.Millisecond)
	}
	return OSMouseMove(x, y)
}

func sleepHuman(minMs, maxMs int) {
	if maxMs <= minMs {
		time.Sleep(time.Duration(minMs) * time.Millisecond)
		return
	}
	time.Sleep(time.Duration(minMs) * time.Millisecond)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func OSMouseClick(button string, x, y int) error {
	// Glide so the human owner can SEE the cursor travel, then pause on target.
	if err := OSMouseMoveSmooth(x, y); err != nil {
		return err
	}
	ax, ay, _ := OSMousePos()
	if abs(ax-x) > 3 || abs(ay-y) > 3 {
		// Cursor didn't land (DPI / multi-monitor) — force placement once more.
		if err := OSMouseMove(x, y); err != nil {
			return err
		}
	}
	time.Sleep(350 * time.Millisecond) // let user SEE the hover before click
	switch button {
	case "right":
		if err := sendMouse(mouseRightDown, 0); err != nil {
			return err
		}
		sleepHuman(40, 40)
		return sendMouse(mouseRightUp, 0)
	case "middle":
		if err := sendMouse(mouseMidDown, 0); err != nil {
			return err
		}
		sleepHuman(40, 40)
		return sendMouse(mouseMidUp, 0)
	default: // left
		if err := sendMouse(mouseLeftDown, 0); err != nil {
			return err
		}
		sleepHuman(40, 40)
		return sendMouse(mouseLeftUp, 0)
	}
}

func OSMouseDoubleClick(x, y int) error {
	if err := OSMouseClick("left", x, y); err != nil {
		return err
	}
	sleepHuman(60, 60)
	return OSMouseClick("left", x, y)
}

func OSMouseDrag(x1, y1, x2, y2 int) error {
	if err := OSMouseMove(x1, y1); err != nil {
		return err
	}
	sleepHuman(60, 60)
	if err := sendMouse(mouseLeftDown, 0); err != nil {
		return err
	}
	// smooth-ish drag in 10 steps so hover states track like a human
	steps := 12
	for i := 1; i <= steps; i++ {
		xi := x1 + (x2-x1)*i/steps
		yi := y1 + (y2-y1)*i/steps
		OSMouseMove(xi, yi)
		time.Sleep(8 * time.Millisecond)
	}
	sleepHuman(40, 40)
	return sendMouse(mouseLeftUp, 0)
}

func OSMouseScroll(clicks int) error {
	// 1 click = WHEEL_DELTA 120. Negative = scroll down.
	data := uint32(int32(clicks * 120))
	return sendMouse(mouseWheel, data)
}

// ---------- keyboard ----------

var vkMap = map[string]uint16{
	"enter": 0x0D, "tab": 0x09, "esc": 0x1B, "escape": 0x1B,
	"backspace": 0x08, "delete": 0x2E, "space": 0x20,
	"up": 0x26, "down": 0x28, "left": 0x25, "right": 0x27,
	"home": 0x24, "end": 0x23, "pgup": 0x21, "pgdn": 0x22,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74,
	"f6": 0x75, "f7": 0x76, "f8": 0x77, "f9": 0x78, "f10": 0x79,
	"f11": 0x7A, "f12": 0x7B,
	"shift": 0x10, "ctrl": 0x11, "alt": 0x12, "win": 0x5B,
	"capslock": 0x14, "insert": 0x2D,
}

func vkFor(name string) (uint16, bool) {
	if len(name) == 1 {
		ch := name[0]
		if ch >= 'a' && ch <= 'z' {
			ch = ch - 'a' + 'A'
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			return uint16(ch), true
		}
	}
	vk, ok := vkMap[name]
	return vk, ok
}

func OSKeyPress(key string, holdMs int) error {
	if vk, ok := vkFor(key); ok {
		if err := sendKey(vk, 0, 0); err != nil {
			return err
		}
		if holdMs > 0 {
			time.Sleep(time.Duration(holdMs) * time.Millisecond)
		} else {
			sleepHuman(30, 30)
		}
		return sendKey(vk, 0, keyUp)
	}
	// fallback: type as unicode char
	return OSTypeText(key, 0)
}

// OSHotkey e.g. ctrl+l, ctrl+shift+t, alt+tab
func OSHotkey(combo string) error {
	parts := splitCombo(combo)
	var mods []uint16
	var main string
	for _, p := range parts {
		if p == "ctrl" || p == "shift" || p == "alt" || p == "win" {
			vk, _ := vkFor(p)
			mods = append(mods, vk)
		} else {
			main = p
		}
	}
	for _, m := range mods {
		sendKey(m, 0, 0)
		time.Sleep(25 * time.Millisecond)
	}
	if main != "" {
		if vk, ok := vkFor(main); ok {
			sendKey(vk, 0, 0)
			time.Sleep(40 * time.Millisecond)
			sendKey(vk, 0, keyUp)
		} else {
			OSTypeText(main, 0)
		}
	}
	time.Sleep(25 * time.Millisecond)
	for i := len(mods) - 1; i >= 0; i-- {
		sendKey(mods[i], 0, keyUp)
		time.Sleep(25 * time.Millisecond)
	}
	return nil
}

// OSTypeText types unicode text human-like with per-char delay.
// Handles BMP + supplementary planes (emoji) via UTF-16 surrogates.
func OSTypeText(text string, cps int) error {
	delay := time.Duration(0)
	if cps > 0 {
		delay = time.Second / time.Duration(cps)
	} else {
		delay = 12 * time.Millisecond
	}
	for _, r := range text {
		var units []uint16
		if r < 0x10000 {
			units = []uint16{uint16(r)}
		} else {
			r -= 0x10000
			units = []uint16{uint16(0xD800 + (r >> 10)), uint16(0xDC00 + (r & 0x3FF))}
		}
		for _, u := range units {
			if err := sendKey(0, u, keyUnicode); err != nil {
				return err
			}
			if err := sendKey(0, u, keyUnicode|keyUp); err != nil {
				return err
			}
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}
	return nil
}

// OSTypeTextFast types with zero delay — race mode. Watch the lesson fly.
func OSTypeTextFast(text string) error {
	for _, r := range text {
		var units []uint16
		if r < 0x10000 {
			units = []uint16{uint16(r)}
		} else {
			r -= 0x10000
			units = []uint16{uint16(0xD800 + (r >> 10)), uint16(0xDC00 + (r & 0x3FF))}
		}
		for _, u := range units {
			if err := sendKey(0, u, keyUnicode); err != nil {
				return err
			}
			if err := sendKey(0, u, keyUnicode|keyUp); err != nil {
				return err
			}
		}
	}
	return nil
}

// OSKeyPressFast presses with minimal hold for max WPM.
func OSKeyPressFast(key string) error {
	if vk, ok := vkFor(key); ok {
		if err := sendKey(vk, 0, 0); err != nil {
			return err
		}
		return sendKey(vk, 0, keyUp)
	}
	return OSTypeTextFast(key)
}

// ---------- windows ----------

type WinInfo struct {
	HWND  uint64 `json:"hwnd"`
	Title string `json:"title"`
	Rect  struct {
		Left   int `json:"left"`
		Top    int `json:"top"`
		Right  int `json:"right"`
		Bottom int `json:"bottom"`
	} `json:"rect"`
}

type rectRaw struct {
	Left, Top, Right, Bottom int32
}

func getWindowText(hwnd uintptr) string {
	buf := make([]uint16, 512)
	r1, _, _ := procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r1 == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func ListWindows() []WinInfo {
	var out []WinInfo
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		vis, _, _ := procIsVisible.Call(hwnd)
		if vis == 0 {
			return 1
		}
		title := getWindowText(hwnd)
		if title == "" {
			return 1
		}
		var rc rectRaw
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&rc)))
		// skip tiny/invisible
		if rc.Right-rc.Left < 50 || rc.Bottom-rc.Top < 50 {
			return 1
		}
		var w WinInfo
		w.HWND = uint64(hwnd)
		w.Title = title
		w.Rect.Left = int(rc.Left)
		w.Rect.Top = int(rc.Top)
		w.Rect.Right = int(rc.Right)
		w.Rect.Bottom = int(rc.Bottom)
		out = append(out, w)
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return out
}

func FocusWindowByTitle(substr string) error {
	hwnd, title, err := findWindowByTitle(substr)
	if err != nil {
		return err
	}
	procShowWindow.Call(hwnd, 9) // SW_RESTORE
	forceForeground(hwnd)
	procBringToTop.Call(hwnd)
	time.Sleep(400 * time.Millisecond) // let compositor/z-order settle before clicking
	_ = title
	return nil
}

// MinimizeWindowByTitle gets the terminal (or any window) out of the way so
// Brave becomes truly foreground and keystrokes land in the lesson, not here.
func MinimizeWindowByTitle(substr string) error {
	hwnd, _, err := findWindowByTitle(substr)
	if err != nil {
		return err
	}
	procShowWindow.Call(hwnd, 6) // SW_MINIMIZE
	time.Sleep(300 * time.Millisecond)
	return nil
}

func findWindowByTitle(substr string) (uintptr, string, error) {
	var found uintptr
	var foundTitle string
	lower := lowerStr(substr)
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		vis, _, _ := procIsVisible.Call(hwnd)
		if vis == 0 {
			return 1
		}
		title := getWindowText(hwnd)
		if title == "" {
			return 1
		}
		if containsLower(title, lower) {
			found = hwnd
			foundTitle = title
			return 0 // stop
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	if found == 0 {
		return 0, "", fmt.Errorf("window %q not found", substr)
	}
	return found, foundTitle, nil
}

// forceForeground works around Windows' foreground lock: a background process
// can't SetForegroundWindow unless it attaches its input thread first.
func forceForeground(hwnd uintptr) {
	fg, _, _ := procGetForeground.Call()
	var pid uint32
	var fgThread uintptr
	if fg != 0 {
		fgThread, _, _ = procGetThread.Call(fg, uintptr(unsafe.Pointer(&pid)))
	}
	curThread, _, _ := procGetCurrentThread.Call()
	if fg != 0 && fgThread != 0 {
		procAttachInput.Call(curThread, fgThread, 1)
	}
	procSetForeground.Call(hwnd)
	if fg != 0 && fgThread != 0 {
		procAttachInput.Call(curThread, fgThread, 0)
	}
}

func getWindowClass(hwnd uintptr) string {
	buf := make([]uint16, 256)
	r1, _, _ := procGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r1 == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

// chromiumWindows lists visible Chromium top-level windows (Brave/Chrome/Edge
// all share Chrome_WidgetWin_1), regardless of title.
func chromiumWindows() []uintptr {
	var out []uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		vis, _, _ := procIsVisible.Call(hwnd)
		if vis == 0 {
			return 1
		}
		if getWindowClass(hwnd) == "Chrome_WidgetWin_1" && getWindowText(hwnd) != "" {
			out = append(out, hwnd)
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return out
}

// AdoptBrowser gracefully migrates the running Chromium into debug mode:
// WM_CLOSE lets it flush session/cookies (unlike kill -9, which drops logins),
// then stale Singleton locks are cleared and it relaunches on port with the
// SAME profile + restored tabs. Returns when the debugger answers.
func AdoptBrowser(binary string, port int, profile string) error {
	if isDebuggingUp(port) {
		return nil
	}
	wins := chromiumWindows()
	for _, w := range wins {
		procPostMsg.Call(w, wmClose, 0, 0)
	}
	for i := 0; i < 60 && len(chromiumWindows()) > 0; i++ {
		time.Sleep(500 * time.Millisecond)
	}
	if len(chromiumWindows()) > 0 {
		return fmt.Errorf("adopt: browser would not close gracefully; refusing kill -9 (would drop your login)")
	}
	if profile != "" {
		for _, n := range []string{"SingletonLock", "SingletonSocket", "SingletonCookie"} {
			_ = os.Remove(filepath.Join(profile, n))
		}
	}
	if _, err := LaunchBrowser(binary, port, profile, "", false, []string{"--restore-last-session"}); err != nil {
		return fmt.Errorf("adopt relaunch: %w", err)
	}
	return nil
}

func lowerStr(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func containsLower(hay, needleLower string) bool {
	h := lowerStr(hay)
	for i := 0; i+len(needleLower) <= len(h); i++ {
		if h[i:i+len(needleLower)] == needleLower {
			return true
		}
	}
	return len(needleLower) == 0
}
