//go:build !windows

package main

import (
	"fmt"
	"runtime"
)

// Non-Windows stubs: the OS-control sector (mouse/keyboard/screen/windows)
// is implemented per-OS. Windows build has the full thing; darwin/linux
// report a clear error instead of silently misbehaving. CDP browser control
// (cdp.go, elements.go, launcher.go, server.go) is fully cross-platform.

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

func osErr() error { return fmt.Errorf("OS control not implemented on %s (Windows build required)", runtime.GOOS) }

func ScreenSize() (int, int)                        { return 0, 0 }
func OSScreenshot() ([]byte, error)                 { return nil, osErr() }
func OSMouseMove(x, y int) error                    { return osErr() }
func OSMouseMoveSmooth(x, y int) error              { return osErr() }
func OSMousePos() (int, int, error)                 { return 0, 0, osErr() }
func OSMouseClick(button string, x, y int) error    { return osErr() }
func OSMouseDoubleClick(x, y int) error             { return osErr() }
func OSMouseDrag(x1, y1, x2, y2 int) error          { return osErr() }
func OSMouseScroll(clicks int) error                { return osErr() }
func OSKeyPress(key string, holdMs int) error       { return osErr() }
func OSKeyPressFast(key string) error               { return osErr() }
func OSHotkey(combo string) error                   { return osErr() }
func OSTypeText(text string, cps int) error         { return osErr() }
func OSTypeTextFast(text string) error              { return osErr() }
func ListWindows() []WinInfo                        { return nil }
func FocusWindowByTitle(substr string) error        { return osErr() }
func MinimizeWindowByTitle(substr string) error     { return osErr() }
func SendTextToWindow(s, t string, d int) (int, int64, error) {
	return 0, 0, osErr()
}
func ShotClean(hideTitle string) ([]byte, error)    { return nil, osErr() }
func RaceType(substr, text string) (int64, error)   { return 0, osErr() }
func RaceTypeCps(s, t string, cps int) (int64, error) { return 0, osErr() }
func ChainType(s, t string, c, p, g int) (int, int64, error) {
	return 0, 0, osErr()
}
func AdoptBrowser(binary string, port int, profile string) error { return osErr() }
