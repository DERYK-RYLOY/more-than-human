package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func splitCombo(s string) []string {
	var out []string
	cur := ""
	for _, ch := range s {
		if ch == '+' || ch == ',' || ch == ' ' {
			if cur != "" {
				out = append(out, strings.ToLower(strings.TrimSpace(cur)))
				cur = ""
			}
			continue
		}
		cur += string(ch)
	}
	if cur != "" {
		out = append(out, strings.ToLower(strings.TrimSpace(cur)))
	}
	return out
}

func braveCandidates() []string {
	cands := []string{
		`C:\Program Files\BraveSoftware\Brave-Browser\Application\brave.exe`,
		`C:\Program Files (x86)\BraveSoftware\Brave-Browser\Application\brave.exe`,
		filepath.Join(os.Getenv("LOCALAPPDATA"), `BraveSoftware\Brave-Browser\Application\brave.exe`),
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`/Applications/Brave Browser.app/Contents/MacOS/Brave Browser`,
		`/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`,
		`/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge`,
		`/usr/bin/brave-browser`,
		`/usr/bin/google-chrome`,
		`/usr/bin/chromium`,
	}
	return cands
}

func findBrowser(explicit string) (string, error) {
	if explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, nil
		}
		// maybe on PATH
		if p, err := exec.LookPath(explicit); err == nil {
			return p, nil
		}
		return "", fmt.Errorf("browser binary not found: %s", explicit)
	}
	for _, c := range braveCandidates() {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	for _, name := range []string{"brave", "brave.exe", "chrome", "chrome.exe", "msedge", "msedge.exe", "google-chrome", "chromium", "chromium-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("no Chromium browser found (Brave/Chrome/Edge)")
}

func isDebuggingUp(port int) bool {
	client := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/json/version", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// LaunchBrowser starts Brave (or any Chromium) with remote debugging.
// Returns when the debugger endpoint answers.
func LaunchBrowser(binary string, port int, userDataDir string, startURL string, headless bool, extraArgs []string) (string, error) {
	bin, err := findBrowser(binary)
	if err != nil {
		return "", err
	}
	if isDebuggingUp(port) {
		return bin, nil // already running
	}
	if userDataDir == "" {
		userDataDir = filepath.Join(os.TempDir(), fmt.Sprintf("human-browser-profile-%d", port))
	}
	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--remote-allow-origins=*",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-hang-monitor",
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
	}
	if headless {
		args = append(args, "--headless=new")
	}
	args = append(args, extraArgs...)
	if startURL != "" {
		args = append(args, startURL)
	} else {
		args = append(args, "about:blank")
	}
	cmd := exec.Command(bin, args...)
	// detach: don't wait
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if err := cmd.Start(); err != nil {
		return "", err
	}
	// wait up to 20s
	for i := 0; i < 40; i++ {
		if isDebuggingUp(port) {
			return bin, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return bin, fmt.Errorf("browser started but debugger port %d never answered", port)
}

// defaultBraveProfile is the real everyday profile (history, logins, tabs).
// adopt uses it so the relaunched Brave IS the user's Brave.
func defaultBraveProfile() string {
	if p := os.Getenv("LOCALAPPDATA"); p != "" {
		cand := filepath.Join(p, `BraveSoftware\Brave-Browser\User Data`)
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		for _, cand := range []string{
			filepath.Join(h, `Library`, `Application Support`, `BraveSoftware`, `Brave-Browser`),
			filepath.Join(h, `.config`, `BraveSoftware`, `Brave-Browser`),
		} {
			if st, err := os.Stat(cand); err == nil && st.IsDir() {
				return cand
			}
		}
	}
	return ""
}
