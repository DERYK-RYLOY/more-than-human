// human-browser: see + control ANY Chromium (Brave first) like a human.
//
// One binary an AI agent drives via CLI (JSON output) or HTTP:
//
//   human-browser launch --port 9222
//   human-browser see --port 9222 --hidden --shot page.png
//   human-browser screen --out screen.png
//   human-browser tabs --port 9222
//   human-browser goto https://example.com --port 9222
//   human-browser click --x 500 --y 300 --port 9222
//   human-browser jsclick --index 12 --port 9222     (even invisible elements)
//   human-browser type --text "hello" --port 9222
//   human-browser press --key Enter --port 9222
//   human-browser eval --js "document.title" --port 9222
//   human-browser mouse --click left --x 100 --y 200
//   human-browser mouse --move --x 100 --y 200
//   human-browser mouse --drag --x1 100 --y1 200 --x2 400 --y2 500
//   human-browser mouse --scroll --clicks -3
//   human-browser key --text "hello fast typing"
//   human-browser key --press Enter
//   human-browser key --combo ctrl+l
//   human-browser windows
//   human-browser focus --title Brave
//   human-browser serve --api 8080 --port 9222
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func mustJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

// logAction appends every AI action to actions.log so the run is
// reviewable and the AI can recover after errors. Never fails the command.
func logAction(kind string, data map[string]any) {
	entry := map[string]any{
		"ts":   time.Now().Format(time.RFC3339),
		"kind": kind,
	}
	for k, v := range data {
		entry[k] = v
	}
	b, _ := json.Marshal(entry)
	f, err := os.OpenFile(filepath.Join(exeDir(), "actions.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(b, '\n'))
}

func exeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func fatal(err error) {
	out, _ := json.Marshal(map[string]any{"ok": false, "error": err.Error()})
	fmt.Println(string(out))
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	switch cmd {
	case "launch":
		cmdLaunch(args)
	case "tabs", "list":
		cmdTabs(args)
	case "new-tab":
		cmdNewTab(args)
	case "close":
		cmdClose(args)
	case "activate":
		cmdActivate(args)
	case "goto", "navigate":
		cmdGoto(args)
	case "back", "forward", "reload":
		cmdNav(args, cmd)
	case "see":
		cmdSee(args)
	case "screen":
		cmdScreen(args)
	case "elements", "els":
		cmdElements(args)
	case "click":
		cmdClick(args)
	case "ctype":
		cmdCType(args)
	case "jsclick":
		cmdJSClick(args)
	case "type":
		cmdType(args)
	case "press":
		cmdPress(args)
	case "hover":
		cmdHover(args)
	case "scroll":
		cmdScroll(args)
	case "eval":
		cmdEval(args)
	case "text":
		cmdText(args)
	case "mouse":
		cmdMouse(args)
	case "key":
		cmdKey(args)
	case "windows":
		cmdWindows(args)
	case "focus":
		cmdFocus(args)
	case "minimize":
		cmdMinimize(args)
	case "sendkeys":
		cmdSendKeys(args)
	case "autopilot":
		cmdAutopilot(args)
	case "tpass":
		cmdTPass(args)
	case "find":
		cmdFind(args)
	case "shotclean":
		cmdShotClean(args)
	case "chain":
		cmdChain(args)
	case "racetype":
		cmdRaceType(args)
	case "wait":
		cmdWait(args)
	case "waitfor":
		cmdWaitFor(args)
	case "downloads":
		cmdDownloads(args)
	case "ask":
		cmdAsk(args)
	case "adopt":
		cmdAdopt(args)
	case "quit":
		cmdQuit(args)
	case "doctor":
		cmdDoctor(args)
	case "bench":
		cmdBench(args)
	case "serve":
		cmdServe(args)
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`MORE THAN HUMAN — AI browser hands + eyes (Brave / Chrome / Edge)
Main job: control ANY browser like a human. Sectors: see/click/type OS+CDP.

EYES (see everything, incl. hidden):
  launch  --port 9222 --browser "" --url https://...   start Brave with debugging
  see     --port 9222 --tab ID --hidden --shot p.png   screenshot + ALL elements + text (main AI command)
  screen  --out screen.png                             full OS screenshot (browser chrome itself)
  elements --port 9222 --hidden --find "login"         list every element incl. invisible
  text    --port 9222                                  page innerText
  tabs    --port 9222                                  list open tabs
  windows                                              list OS windows (to find Brave)

HANDS (control everything like mouse+keyboard):
  goto    <url> --port 9222 --tab ID --new
  click   --x 500 --y 300 --port 9222                  real CDP mouse click at page coords
  ctype   --text "whole passage" --wpm 45 --port 9222   batch-type over ONE connection, self-verified
  jsclick --index 12 --port 9222                       click even invisible/covered element
  type    --text "hello" --port 9222                   type into focused field (page-level)
  press   --key Enter --port 9222                      press key (page-level)
  hover   --x 100 --y 200 --port 9222
  scroll  --dy -600 --x 400 --y 400 --port 9222
  eval    --js "document.title" --port 9222            run JS, get value
  mouse   --click left --x 100 --y 200                  OS-level real mouse (browser UI itself!)
          --move --x 100 --y 200                        (smooth glide so you can WATCH)
          --pos                                          report cursor + screen size
          --drag --x1 10 --y1 10 --x2 200 --y2 300
          --scroll --clicks -3
          --dclick --x 100 --y 200
  key     --text "typing like human"                   OS-level typing anywhere
          --press Enter
          --combo ctrl+l
  focus   --title Brave                                bring browser to front
  minimize --title "Human-like"                       get terminal out of the way for races
  sendkeys --title "Typing Lessons" --text "uuu" --gap 0   post keys DIRECT to Brave (no focus needed!)
  chain   --title "Typing Page" --text "full passage" --chunk 60 --cps 4   word-safe atomic run
  autopilot --port 9223 --tab ID --wpm 59                  perfect typing-test pass (extract+type+verify)
  autopilot --guide                                        the recorded perfect-run playbook for any AI
  tpass   --port 9223 --tab ID --wpm 50 --chars 1245       whole perfect pass in ONE call (dry with --dry)
  find    --text "Start Test" --port 9223                  exact coords of controls (pair with click)
  shotclean --out clean.png --hide "Human-like"       minimize terminal + screenshot in ONE call
  racetype --title "Typing Lessons" --text "uuu"       foreground + REAL typing at full speed, timed
  wait    --ms 1200  |  wait --for-text "Start" --port 9222 --timeout 15
  waitfor --text "Save" --port 9222 --timeout 30       wait until text/selector exists
  downloads --dir C:\dl --port 9222                   route browser downloads there
  ask     --prompt "2FA code?" --out code.txt         pause for the human, read one line
  adopt   --port 9223                                 gracefully migrate running Brave to debug mode
  quit    --port 9230                                 close ONLY the browser on that port (safe cleanup)
  doctor   [--repair]                                 diagnose this machine + AI self-repair playbook
  bench    --port 9222 [--keys 20]                     measure live CDP latency (rerunnable proof)
  back|forward|reload --port 9222
  new-tab <url> --port 9222 | close --tab ID | activate --tab ID
  serve   --api 8080 --port 9222                       HTTP API for any AI agent

Every command prints JSON. Screenshots: page.png / screen.png viewable by the AI.
Default --port is 9222 (Brave debugging).
MORE THAN HUMAN — browser control is the main job; autopilot is one sector.`)
}

// autopilotGuide is the recorded perfect-run playbook. Any AI replaying it
// on ANY typing activity must follow every rule or fail.
const autopilotGuide = `MORE THAN HUMAN — AUTOPILOT PLAYBOOK (recorded perfect passes)

MISSION: complete a typing test/activity with 0 errors at any commanded
pace — full blast (pour the whole text in one go) or human-paced. Every
recorded run lands at 100% accuracy; any pace cap ever seen was the
operator's choice, never the tool's limit.

PHASE 0 — ATTACH QUIETLY (browser control stays the main job)
- Never close the browser, never open junk windows/tabs nobody asked for.
- Prefer CDP on the ALREADY-RUNNING browser (launch with
  --remote-debugging-port + the REAL user-data-dir + --restore-last-session
  only if no debug port answers). Tabs, login and cookies must survive.
- If a flag must come after a positional arg, parseAny handles any order —
  but prefer flags-first out of habit.

PHASE 1 — READ THE SITE'S MIND (ground truth, never transcription)
- The expected text lives in the DOM. On letter-tile typing tests every
  expected character is a div.letter in order; join textContent of ALL of them.
- Separators are often NBSP (U+00A0), not U+0020 — extract raw, type 0x20
  (accepted) or match exactly; never assume, always extract.
- Cursor = element with is-act class (index). Errors = elements matching
  wrong|error|incorrect. Fresh test means cursor 0 AND errors 0.
- ExtractPassage gives {text, cursor, errors, total} in ONE call. If total=0
  the test is not loaded (intro screen?) — find and click its Continue first.
- This covers ALL future words/signs/spaces upfront. Screenshots are for the
  human owner to watch, NEVER for reading text.

PHASE 2 — REFUSE DAMAGED RUNS
- If errors>0 or cursor!=0 on arrival: STOP. Press Redo (via DOM click) for
  a fresh test and re-extract — Redo may randomize the passage, so ALWAYS
  re-extract after any Redo. Never type onto a shifted cursor: one missed
  joint space cascades every following letter into the wrong slot.
- If the run breaks mid-way (wrong slot, focus lost, page reset under you):
  STOP, assess one screenshot, Redo, restart. Chasing a broken run only
  banks errors.

PHASE 3 — TYPE (paced, atomic, exact)
- ctype sends REAL keyDown/keyUp per char over ONE CDP connection:
  letters (KeyX), Space, digits (DigitN), . , ' - " ; : ? ! are mapped;
  anything else must FAIL LOUDLY before starting, never skip silently.
- Capitals and " ? ! : + _ < > | ~ @ # $ % ^ & * ( ) ride with Shift held,
  exactly like a human (bare Quote events are marked wrong).
- 5 chars = 1 word. gap = 60s/(wpm*5). Measured pace runs ~10% under the
  setting (site timer + protocol overhead), so set ~10% above target.
- Chunk rule for OS-level typing (racetype/chain): split ONLY at spaces so
  every chunk ends with trailing space (last chunk excepted) — chain does
  this automatically. Never break mid-word, never drop a joint space.
- insertText does NOT advance keydown-driven tests; SendInput unicode works
  for OS mode but needs real foreground (racetype/chain foreground inside
  the SAME call). CDP press/ctype needs no foreground at all.
- Caps Lock toggles globally: if a caps warning appears, toggle it back
  from anywhere and dismiss the popup; unicode injection itself is
  caps-proof but the site watches the real key state.

PHASE 4 — VERIFY WITHOUT LOOKING
- Re-read cursor/errors from the DOM after the run: sent==total, cursor at
  end (or -1 = finished, results screen), errors==0.
- Read the results text from the DOM for the official WPM/accuracy.
- One screenshot at the END is for the human's victory lap, not for you.

RECOVERY CHEAT-SHEET
- Mid-run reset by anyone (even the owner) randomizes the passage: STOP,
  re-extract, restart from zero. Never pour old text into a new test.
- Background tabs accept CDP keys fine; OS SendInput needs foreground.
- Timer pauses when the window loses focus — verification pauses cost
  nothing, but mid-run pauses cost WPM. Fewer, longer runs beat many
  short ones for measured speed.
- Keep error budget in mind (<50): any run with errors banked stays red;
  Redo early rather than bleed errors across a whole page.

PHASE 5 — ONE-CALL PASSES (tpass: the whole mission in one tool call)
- tpass --port 9223 --tab ID --wpm 50 --chars 1245 [--dry] does everything
  above in ONE connection: extract ground truth, refuse damaged tests,
  validate every rune, type with an error gate every 100 keys (aborts
  loudly instead of cascading), verify cursor+errors, report effectiveWPM.
- Timed tests: --chars caps keys so nothing spills onto results screens.
- Calibrate with effectiveWPM (reported every run): measured runs ~10%
  under setting (CDP overhead ~10-15ms/key), so set ~10% above target.
- Never screenshot mid-run. Never hand-transcribe (files corrupt exotic
  chars like U+23CE through consoles). Never pour old text into a new
  test — Redo randomizes passages, always re-extract.

PHASE 6 — THE FIVE UPGRADES (shipped after the perfect passes)
- find --text: exact control coordinates from the DOM (no more eyeballing;
  pair with click --x --y). tpass/autopilot --retry N: Redo + restart
  automatically on abort/fail (self-heal). waitfor: poll for text/selector
  (captchas, spinners, loading). downloads --dir: route files. ask: pause
  for the human (2FA/captcha codes) instead of failing.
- adopt: migrates an already-running Brave into debug mode with WM_CLOSE
  (session/cookies flushed properly — never kill -9 a live browser),
  stale-lock cleanup, same-profile relaunch, tab verification.
- OS control is split per-OS (Windows full; darwin/linux stubs compile,
  same CLI). CDP core is fully cross-platform already.
`

// doctorRepair is the AI self-repair playbook, printed by `doctor` on any
// non-Windows machine (or anywhere with --repair). Small by design.
const doctorRepair = `MORE THAN HUMAN — DOCTOR REPAIR PLAYBOOK (for the AI holding this tool)

SYMPTOM: mouse/key/screen/windows/focus commands fail with
  "OS control not implemented on darwin (Windows build required)".
  That is EXPECTED outside Windows: os_other.go holds stubs with the exact
  signatures. CDP commands (tabs/see/click/type/ctype/tpass/find/serve)
  work everywhere already — if you only need page control, stop here.

TO FIX OS CONTROL ON YOUR OS (darwin or linux), implement these, in order:
1. Copy the exported function list from os_other.go into a new file
   os_darwin.go (//go:build darwin) or os_linux.go (//go:build linux).
   Keep every signature identical; only bodies change.
2. macOS: drive input with AppleScript/JXA via /usr/bin/osascript
   (mouse move/click, keystroke/key code), screenshots with /usr/sbin/
   screencapture, windows with AppleEvents + CGWindowList. No CGO needed
   if you shell out; keep calls inside Go with os/exec.
3. Linux: screenshots already work (kbinani/screenshot + xgb are in go.mod).
   Input via X11 XTEST (pure Go with github.com/jezek/xgb xtest ext) or
   shell out to xdotool; windows via EWMH (_NET_CLIENT_LIST/_NET_ACTIVE_WINDOW).
4. Extend braveCandidates[] (launcher.go) + defaultBraveProfile() with your
   OS paths (darwin: /Applications/Brave Browser.app/...; linux: PATH
   lookup brave-browser/google-chrome/chromium). Then `+"`launch`"+` just works.
5. Verify: GOOS=darwin (or linux) go build, then mouse --pos returns real
   coordinates, screen --out writes a real PNG, key --press Enter lands it.
6. If a single command misbehaves: run doctor again, read actions.log tail,
   retry once; if CDP calls fail with "not running", the browser simply
   lacks --remote-debugging-port — use adopt (graceful) or launch.

DESIGN LAWS (do not break these while repairing):
- Stubs must NEVER fake success: return the clear error, like now.
- CDP stays platform-free: no syscall imports outside os_*.go.
- One call = one foreground guarantee stays true on every OS.
- Never kill -9 a user's browser; adopt migrates gracefully or refuses.
`

func portFlag(fs *flag.FlagSet) *int {	return fs.Int("port", 9222, "CDP debugging port")
}

// parseAny lets flags appear BEFORE or AFTER positional args
// (Go's flag pkg silently ignores flags after positionals —
// that bug once sent a navigate to the wrong browser).
func parseAny(fs *flag.FlagSet, args []string) {
	var flags, pos []string
	isBool := func(name string) bool {
		f := fs.Lookup(name)
		if f == nil {
			return false
		}
		_, ok := f.Value.(boolFlag)
		return ok
	}
	i := 0
	for i < len(args) {
		a := args[i]
		if len(a) > 1 && strings.HasPrefix(a, "-") {
			name := strings.TrimLeft(a, "-")
			if eq := strings.Index(name, "="); eq >= 0 {
				name = name[:eq]
			}
			flags = append(flags, a)
			if !strings.Contains(a, "=") && !isBool(name) && fs.Lookup(name) != nil {
				i++
				if i < len(args) {
					flags = append(flags, args[i])
				}
			}
		} else {
			pos = append(pos, a)
		}
		i++
	}
	_ = fs.Parse(append(flags, pos...))
}

type boolFlag interface {
	IsBoolFlag() bool
}

// ---------- commands ----------

func cmdLaunch(args []string) {
	fs := flag.NewFlagSet("launch", flag.ExitOnError)
	port := fs.Int("port", 9222, "")
	binary := fs.String("browser", "", "path to brave/chrome/edge exe (auto-detect if empty)")
	profile := fs.String("profile", "", "user-data-dir")
	url := fs.String("url", "", "start URL")
	headless := fs.Bool("headless", false, "headless")
	parseAny(fs, args)
	bin, err := LaunchBrowser(*binary, *port, *profile, *url, *headless, nil)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "browser": bin, "port": *port})
}

func cmdTabs(args []string) {
	fs := flag.NewFlagSet("tabs", flag.ExitOnError)
	port := portFlag(fs)
	parseAny(fs, args)
	tabs, err := ListTabs(*port)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "tabs": tabs})
}

func cmdNewTab(args []string) {
	fs := flag.NewFlagSet("new-tab", flag.ExitOnError)
	port := portFlag(fs)
	parseAny(fs, args)
	url := ""
	if fs.NArg() > 0 {
		url = fs.Arg(0)
	}
	t, err := NewTab(*port, url)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "tab": t})
}

func cmdClose(args []string) {
	fs := flag.NewFlagSet("close", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "tab id")
	parseAny(fs, args)
	if err := CloseTab(*port, *tab); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true})
}

func cmdActivate(args []string) {
	fs := flag.NewFlagSet("activate", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "tab id")
	parseAny(fs, args)
	if err := ActivateTab(*port, *tab); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true})
}

func cmdGoto(args []string) {
	fs := flag.NewFlagSet("goto", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "tab id")
	isNew := fs.Bool("new", false, "open in new tab")
	parseAny(fs, args)
	if fs.NArg() == 0 {
		fatal(fmt.Errorf("usage: goto <url>"))
	}
	url := fs.Arg(0)
	if *isNew {
		t, err := NewTab(*port, url)
		if err != nil {
			fatal(err)
		}
		mustJSON(map[string]any{"ok": true, "tab": t})
		return
	}
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	if err := PageNavigate(c, url); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "navigated": url})
}

func cmdNav(args []string, what string) {
	fs := flag.NewFlagSet(what, flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	parseAny(fs, args)
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	switch what {
	case "back":
		err = PageBack(c)
	case "forward":
		err = PageForward(c)
	default:
		err = PageReload(c)
	}
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "action": what})
}

// see = THE human-vision command: screenshot + all elements (visible AND hidden) + text
func cmdSee(args []string) {
	fs := flag.NewFlagSet("see", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "tab id (default: active)")
	hidden := fs.Bool("hidden", true, "include invisible elements")
	full := fs.Bool("full", false, "full-page screenshot beyond viewport")
	shot := fs.String("shot", "", "save screenshot PNG path")
	find := fs.String("find", "", "filter elements by text")
	maxEls := fs.Int("max", 200, "max elements printed (0=all)")
	b64 := fs.Bool("b64", false, "print screenshot base64 in JSON (big!)")
	parseAny(fs, args)

	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()

	png, err := PageScreenshot(c, *full)
	if err != nil {
		fatal(err)
	}
	if *shot != "" {
		if err := savePNG(png, *shot); err != nil {
			fatal(err)
		}
	}
	els, err := GetElements(c, *hidden)
	if err != nil {
		fatal(err)
	}
	if *find != "" {
		els = FindElements(els, *find)
	}
	title, _ := EvalValue(c, "document.title")
	pageURL, _ := EvalValue(c, "location.href")
	text, _ := PageText(c)

	shown := els
	truncated := false
	if *maxEls > 0 && len(els) > *maxEls {
		shown = els[:*maxEls]
		truncated = true
	}
	out := map[string]any{
		"ok": true, "title": title, "url": pageURL,
		"text": truncate(fmt.Sprint(text), 6000),
		"count": len(els), "truncated": truncated,
		"elements": shown,
		"shot":     *shot,
	}
	if *b64 {
		out["shotB64"] = base64.StdEncoding.EncodeToString(png)
	}
	mustJSON(out)
}

func cmdScreen(args []string) {
	fs := flag.NewFlagSet("screen", flag.ExitOnError)
	out := fs.String("out", "screen.png", "save path")
	b64 := fs.Bool("b64", false, "include base64")
	parseAny(fs, args)
	shot, err := OSScreenshot()
	if err != nil {
		fatal(err)
	}
	if *out != "" {
		if err := savePNG(shot, *out); err != nil {
			fatal(err)
		}
	}
	w, h := ScreenSize()
	res := map[string]any{"ok": true, "w": w, "h": h, "shot": *out}
	if *b64 {
		res["shotB64"] = base64.StdEncoding.EncodeToString(shot)
	}
	mustJSON(res)
}

func cmdElements(args []string) {
	fs := flag.NewFlagSet("elements", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	hidden := fs.Bool("hidden", true, "")
	find := fs.String("find", "", "")
	maxEls := fs.Int("max", 300, "")
	parseAny(fs, args)
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	els, err := GetElements(c, *hidden)
	if err != nil {
		fatal(err)
	}
	if *find != "" {
		els = FindElements(els, *find)
	}
	if *maxEls > 0 && len(els) > *maxEls {
		els = els[:*maxEls]
	}
	mustJSON(map[string]any{"ok": true, "count": len(els), "elements": els})
}

func cmdClick(args []string) {
	fs := flag.NewFlagSet("click", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	x := fs.Float64("x", -1, "page x (from `see`)")
	y := fs.Float64("y", -1, "page y")
	parseAny(fs, args)
	if *x < 0 || *y < 0 {
		fatal(fmt.Errorf("usage: click --x 500 --y 300 (get coords from `see`)"))
	}
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	if err := HumanClick(c, *x, *y); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "clicked": []float64{*x, *y}})
}

func cmdCType(args []string) {
	fs := flag.NewFlagSet("ctype", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "tab id (default: active)")
	text := fs.String("text", "", "exact passage (ground truth from DOM)")
	file := fs.String("file", "", "read passage from UTF-8 file (immune to shell quoting)")
	max := fs.Int("chars", 0, "type only first N chars then stop+report (for timed tests)")
	wpm := fs.Int("wpm", 45, "pace, ≤50 to stay human")
	parseAny(fs, args)
	if *text == "" && *file != "" {
		b, err := os.ReadFile(*file)
		if err != nil {
			fatal(err)
		}
		*text = string(b)
	}
	if *text == "" && fs.NArg() > 0 {
		*text = strings.Join(fs.Args(), " ")
	}
	if *text == "" {
		fatal(fmt.Errorf("usage: ctype --text \"exact passage\" [--file passage.txt] --wpm 45 --port 9223"))
	}
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	res, err := CTypeTextMax(c, *text, *wpm, *max)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "sent": res.Sent, "ms": res.Ms, "cursor": res.Cursor, "errors": res.Errors})
	logAction("ctype", map[string]any{"sent": res.Sent, "ms": res.Ms, "cursor": res.Cursor, "errors": res.Errors})
}

func cmdJSClick(args []string) {	fs := flag.NewFlagSet("jsclick", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	index := fs.Int("index", -1, "element index from `see`/`elements`")
	parseAny(fs, args)
	if *index < 0 {
		fatal(fmt.Errorf("usage: jsclick --index 12"))
	}
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	if err := JSClickByIndex(c, *index); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "jsclicked": *index})
}

func cmdType(args []string) {
	fs := flag.NewFlagSet("type", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	text := fs.String("text", "", "text to type")
	parseAny(fs, args)
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	if err := TypeText(c, *text); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "typed": *text})
}

func cmdPress(args []string) {
	fs := flag.NewFlagSet("press", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	key := fs.String("key", "Enter", "Enter|Tab|Escape|Backspace|ArrowLeft|...")
	code := fs.String("code", "", "optional DOM code")
	parseAny(fs, args)
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	if err := PressKey(c, *key, *code); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "pressed": *key})
}

func cmdHover(args []string) {
	fs := flag.NewFlagSet("hover", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	x := fs.Float64("x", 0, "")
	y := fs.Float64("y", 0, "")
	parseAny(fs, args)
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	if err := HumanHover(c, *x, *y); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true})
}

func cmdScroll(args []string) {
	fs := flag.NewFlagSet("scroll", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	x := fs.Float64("x", 400, "")
	y := fs.Float64("y", 400, "")
	dy := fs.Float64("dy", -600, "negative = down")
	parseAny(fs, args)
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	if err := HumanScroll(c, *x, *y, *dy); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "dy": *dy})
}

func cmdEval(args []string) {
	fs := flag.NewFlagSet("eval", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	js := fs.String("js", "", "JavaScript expression")
	parseAny(fs, args)
	if *js == "" && fs.NArg() > 0 {
		*js = strings.Join(fs.Args(), " ")
	}
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	v, err := EvalValue(c, *js)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "value": v})
}

func cmdText(args []string) {
	fs := flag.NewFlagSet("text", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	parseAny(fs, args)
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	t, err := PageText(c)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "text": t})
}

func cmdMouse(args []string) {
	fs := flag.NewFlagSet("mouse", flag.ExitOnError)
	click := fs.String("click", "", "left|right|middle (requires --x --y)")
	move := fs.Bool("move", false, "")
	pos := fs.Bool("pos", false, "print current cursor position")
	dclick := fs.Bool("dclick", false, "")
	drag := fs.Bool("drag", false, "")
	scroll := fs.Bool("scroll", false, "")
	x := fs.Int("x", 0, "")
	y := fs.Int("y", 0, "")
	x1 := fs.Int("x1", 0, "")
	y1 := fs.Int("y1", 0, "")
	x2 := fs.Int("x2", 0, "")
	y2 := fs.Int("y2", 0, "")
	clicks := fs.Int("clicks", -3, "wheel clicks for --scroll")
	parseAny(fs, args)
	var err error
	action := ""
	switch {
	case *pos:
		cx, cy, e := OSMousePos()
		if e != nil {
			fatal(e)
		}
		w, h := ScreenSize()
		mustJSON(map[string]any{"ok": true, "x": cx, "y": cy, "screenW": w, "screenH": h})
		logAction("mouse-pos", map[string]any{"x": cx, "y": cy})
		return
	case *drag:
		action = "drag"
		err = OSMouseDrag(*x1, *y1, *x2, *y2)
	case *scroll:
		action = "scroll"
		err = OSMouseScroll(*clicks)
	case *dclick:
		action = "dclick"
		err = OSMouseDoubleClick(*x, *y)
	case *move:
		action = "move"
		err = OSMouseMoveSmooth(*x, *y)
	case *click != "":
		action = "click-" + *click
		err = OSMouseClick(*click, *x, *y)
	default:
		fatal(fmt.Errorf("usage: mouse --click left --x 100 --y 200 | --move --x 100 --y 200 | --pos | --drag | --scroll | --dclick"))
	}
	if err != nil {
		fatal(err)
	}
	cx, cy, _ := OSMousePos()
	w, h := ScreenSize()
	mustJSON(map[string]any{"ok": true, "action": action, "x": cx, "y": cy, "screenW": w, "screenH": h})
	logAction("mouse-"+action, map[string]any{"x": *x, "y": *y, "actualX": cx, "actualY": cy})
}

func cmdKey(args []string) {
	fs := flag.NewFlagSet("key", flag.ExitOnError)
	text := fs.String("text", "", "type text at OS level")
	press := fs.String("press", "", "press key at OS level")
	combo := fs.String("combo", "", "hotkey e.g. ctrl+l")
	fast := fs.Bool("fast", false, "no inter-key delay (race mode)")
	parseAny(fs, args)
	var err error
	kind := ""
	delay := 0
	if *fast {
		delay = -1 // OSTypeText: negative = zero delay
	}
	switch {
	case *combo != "":
		kind = "hotkey:" + *combo
		err = OSHotkey(*combo)
	case *press != "":
		kind = "press:" + *press
		if *fast {
			err = OSKeyPressFast(*press)
		} else {
			err = OSKeyPress(*press, 0)
		}
	case *text != "":
		kind = "type-os"
		if delay < 0 {
			err = OSTypeTextFast(*text)
		} else {
			err = OSTypeText(*text, 0)
		}
	default:
		fatal(fmt.Errorf("usage: key --text \"hi\" | --press Enter | --combo ctrl+l"))
	}
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "key": kind})
	logAction("key", map[string]any{"kind": kind})
}

func cmdWindows(args []string) {
	_ = args
	mustJSON(map[string]any{"ok": true, "windows": ListWindows()})
}

func cmdFocus(args []string) {
	fs := flag.NewFlagSet("focus", flag.ExitOnError)
	title := fs.String("title", "Brave", "window title substring")
	parseAny(fs, args)
	if err := FocusWindowByTitle(*title); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "focused": *title})
	logAction("focus", map[string]any{"title": *title})
}

func cmdMinimize(args []string) {
	fs := flag.NewFlagSet("minimize", flag.ExitOnError)
	title := fs.String("title", "", "window title substring")
	parseAny(fs, args)
	if *title == "" && fs.NArg() > 0 {
		*title = fs.Arg(0)
	}
	if *title == "" {
		fatal(fmt.Errorf("usage: minimize --title \"Human-like\""))
	}
	if err := MinimizeWindowByTitle(*title); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "minimized": *title})
	logAction("minimize", map[string]any{"title": *title})
}

func cmdSendKeys(args []string) {
	fs := flag.NewFlagSet("sendkeys", flag.ExitOnError)
	title := fs.String("title", "Typing Lessons", "target window title substring")
	text := fs.String("text", "", "characters to post directly to the window")
	gap := fs.Int("gap", 0, "ms between chars (0 = fastest)")
	parseAny(fs, args)
	if *text == "" && fs.NArg() > 0 {
		*text = strings.Join(fs.Args(), " ")
	}
	if *text == "" {
		fatal(fmt.Errorf("usage: sendkeys --title \"Typing Lessons\" --text \"uuu\""))
	}
	sent, ms, err := SendTextToWindow(*title, *text, *gap)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "sent": sent, "ms": ms, "to": *title})
	logAction("sendkeys", map[string]any{"to": *title, "sent": sent, "ms": ms})
}

func cmdChain(args []string) {
	fs := flag.NewFlagSet("chain", flag.ExitOnError)
	title := fs.String("title", "Typing Page", "target window title substring")
	text := fs.String("text", "", "FULL passage; auto-split at spaces, atomic run")
	chunk := fs.Int("chunk", 60, "max chars per burst")
	cps := fs.Int("cps", 4, "chars/sec (~4 = 50 WPM)")
	gap := fs.Int("gap", 250, "ms pause between bursts")
	parseAny(fs, args)
	if *text == "" && fs.NArg() > 0 {
		*text = strings.Join(fs.Args(), " ")
	}
	if *text == "" {
		fatal(fmt.Errorf("usage: chain --title \"Typing Page\" --text \"full passage...\""))
	}
	sent, ms, err := ChainType(*title, *text, *chunk, *cps, *gap)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "sent": sent, "ms": ms})
	logAction("chain", map[string]any{"to": *title, "sent": sent, "ms": ms})
}

// cmdAutopilot is the MORE THAN HUMAN sector: a recorded, perfect,
// self-verifying typing-test pass any AI can replay on any activity.
// --guide prints the full playbook (no browser touched).
func cmdAutopilot(args []string) {
	fs := flag.NewFlagSet("autopilot", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "tab id (default: active)")
	wpm := fs.Int("wpm", 59, "pace for the pass")
	guide := fs.Bool("guide", false, "print the perfect-run playbook and exit")
	dry := fs.Bool("dry", false, "extract + report only, do not type")
	parseAny(fs, args)
	if *guide {
		fmt.Print(autopilotGuide)
		return
	}
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	p, err := ExtractPassage(c)
	if err != nil {
		fatal(err)
	}
	if p.Total == 0 {
		fatal(fmt.Errorf("no test loaded: open a typing test in the tab first, then rerun"))
	}
	if p.Errors > 0 || p.Cursor != 0 {
		fatal(fmt.Errorf("test already started/damaged (cursor=%d errors=%d): press Redo for a fresh test, then rerun", p.Cursor, p.Errors))
	}
	for _, r := range p.Text {
		if _, _, _, ok := charKey(r); !ok {
			fatal(fmt.Errorf("autopilot: unsupported char %q — extend charKey first", string(r)))
		}
	}
	head := p.Text
	if len(head) > 60 {
		head = head[:60]
	}
	if *dry {
		mustJSON(map[string]any{"ok": true, "dry": true, "total": p.Total, "cursor": p.Cursor, "errors": p.Errors, "head": head})
		return
	}
	res, err := CTypeText(c, p.Text, *wpm)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": res.Errors == 0, "sent": res.Sent, "ms": res.Ms, "cursor": res.Cursor, "errors": res.Errors, "total": p.Total})
	logAction("autopilot", map[string]any{"sent": res.Sent, "ms": res.Ms, "cursor": res.Cursor, "errors": res.Errors})
}

// cmdTPass is the one-call perfect pass: extract + refuse-if-damaged +
// validate + type with error gates + verify. Replaces ~10 round trips.
func cmdTPass(args []string) {
	fs := flag.NewFlagSet("tpass", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "tab id (default: active)")
	wpm := fs.Int("wpm", 50, "pace")
	chars := fs.Int("chars", 0, "type only first N chars (timed tests)")
	dry := fs.Bool("dry", false, "extract + validate only, do not type")
	retry := fs.Int("retry", 0, "self-heal: Redo + restart up to N times on abort/fail")
	parseAny(fs, args)
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	var rep TPassReport
	for attempt := 0; ; attempt++ {
		rep, err = TPass(c, *wpm, *chars, *dry)
		if err != nil {
			fatal(err)
		}
		ok := !rep.Aborted && rep.Errors == 0
		if ok || *dry || attempt >= *retry {
			mustJSON(map[string]any{
				"ok": ok, "dry": *dry, "attempts": attempt + 1,
				"sent": rep.Sent, "ms": rep.Ms, "effectiveWPM": rep.EffectiveWPM,
				"cursor": rep.Cursor, "errors": rep.Errors, "total": rep.Total,
				"aborted": rep.Aborted, "abortWhy": rep.AbortWhy,
			})
			logAction("tpass", map[string]any{"sent": rep.Sent, "ms": rep.Ms, "errors": rep.Errors, "aborted": rep.Aborted, "attempts": attempt + 1})
			return
		}
		if err := RedoTest(c); err != nil {
			fatal(fmt.Errorf("self-heal redo failed: %w", err))
		}
		time.Sleep(2500 * time.Millisecond)
	}
}

// cmdFind returns exact viewport coordinates for controls matching text.
// Feed an index back with --click-index to click it with pixel truth.
func cmdFind(args []string) {
	fs := flag.NewFlagSet("find", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "tab id (default: active)")
	text := fs.String("text", "", "match against text/aria-label/placeholder")
	clickIdx := fs.Int("click-index", -1, "click match N at its exact center")
	parseAny(fs, args)
	if *text == "" && fs.NArg() > 0 {
		*text = strings.Join(fs.Args(), " ")
	}
	if *text == "" {
		fatal(fmt.Errorf("usage: find --text \"Start Test\" [--click-index 0] --port 9223"))
	}
	c, err := DialTab(*port, *tab)
	if err != nil {
		fatal(err)
	}
	defer c.Close()
	m, err := FindExact(c, *text)
	if err != nil {
		fatal(err)
	}
	clicked := -1
	if *clickIdx >= 0 && *clickIdx < len(m) {
		if err := HumanClick(c, m[*clickIdx].CX, m[*clickIdx].CY); err != nil {
			fatal(err)
		}
		clicked = *clickIdx
	}
	mustJSON(map[string]any{"ok": true, "matches": m, "clicked": clicked})
	logAction("find", map[string]any{"text": *text, "n": len(m), "clicked": clicked})
}

func cmdShotClean(args []string) {	fs := flag.NewFlagSet("shotclean", flag.ExitOnError)
	out := fs.String("out", "clean.png", "save path")
	hide := fs.String("hide", "Human-like", "terminal title to minimize first")
	parseAny(fs, args)
	shot, err := ShotClean(*hide)
	if err != nil {
		fatal(err)
	}
	if *out != "" {
		if err := savePNG(shot, *out); err != nil {
			fatal(err)
		}
	}
	w, h := ScreenSize()
	mustJSON(map[string]any{"ok": true, "shot": *out, "w": w, "h": h})
}

func cmdRaceType(args []string) {
	fs := flag.NewFlagSet("racetype", flag.ExitOnError)
	title := fs.String("title", "Typing Lessons", "target window")
	text := fs.String("text", "", "text to type at FULL speed")
	cps := fs.Int("cps", 0, "paced mode chars/sec: 4 ≈ 50 WPM (0 = fastest)")
	parseAny(fs, args)
	if *text == "" && fs.NArg() > 0 {
		*text = strings.Join(fs.Args(), " ")
	}
	if *text == "" {
		fatal(fmt.Errorf("usage: racetype --title \"Typing Lessons\" --text \"uuu\" [--cps 4]"))
	}
	var ms int64
	var err error
	if *cps > 0 {
		ms, err = RaceTypeCps(*title, *text, *cps)
	} else {
		ms, err = RaceType(*title, *text)
	}
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "typed": *text, "chars": len(*text), "ms": ms, "cps": *cps})
	logAction("racetype", map[string]any{"to": *title, "chars": len(*text), "ms": ms, "cps": *cps})
}

func cmdWait(args []string) {	fs := flag.NewFlagSet("wait", flag.ExitOnError)
	port := fs.Int("port", 9222, "")
	ms := fs.Int("ms", 0, "plain sleep milliseconds")
	forText := fs.String("for-text", "", "wait until page text contains this (CDP)")
	timeout := fs.Int("timeout", 15, "seconds for --for-text")
	parseAny(fs, args)
	if *forText != "" {
		deadline := time.Now().Add(time.Duration(*timeout) * time.Second)
		for {
			c, err := DialTab(*port, "")
			if err != nil {
				fatal(err)
			}
			t, err := PageText(c)
			c.Close()
			if err == nil && strings.Contains(strings.ToLower(t), strings.ToLower(*forText)) {
				mustJSON(map[string]any{"ok": true, "found": *forText})
				logAction("wait-for-text", map[string]any{"text": *forText})
				return
			}
			if time.Now().After(deadline) {
				fatal(fmt.Errorf("timeout waiting for text %q", *forText))
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	if *ms <= 0 {
		*ms = 1000
	}
	time.Sleep(time.Duration(*ms) * time.Millisecond)
	mustJSON(map[string]any{"ok": true, "waitedMs": *ms})
}

// cmdWaitFor polls the page until text (or selector) exists. Patience,
// built in: captchas/codes/loading spinners stop blocking the AI.
func cmdWaitFor(args []string) {
	fs := flag.NewFlagSet("waitfor", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "")
	text := fs.String("text", "", "wait until page innerText contains this")
	sel := fs.String("selector", "", "or: wait until CSS selector matches")
	timeout := fs.Int("timeout", 30, "seconds")
	parseAny(fs, args)
	if *text == "" && *sel == "" {
		fatal(fmt.Errorf("usage: waitfor --text \"Save\" [--timeout 30] --port 9223"))
	}
	deadline := time.Now().Add(time.Duration(*timeout) * time.Second)
	for {
		c, err := DialTab(*port, *tab)
		if err != nil {
			fatal(err)
		}
		found := false
		if *sel != "" {
			v, err := EvalValue(c, `document.querySelector(`+jsStr(*sel)+`) ? 1 : 0`)
			c.Close()
			if err == nil {
				if n, ok := v.(float64); ok && n == 1 {
					found = true
				}
			}
		} else {
			t, err := PageText(c)
			c.Close()
			if err == nil && strings.Contains(strings.ToLower(t), strings.ToLower(*text)) {
				found = true
			}
		}
		if found {
			mustJSON(map[string]any{"ok": true, "found": *text + *sel})
			logAction("waitfor", map[string]any{"found": *text + *sel})
			return
		}
		if time.Now().After(deadline) {
			fatal(fmt.Errorf("waitfor timeout (%ds): %q", *timeout, *text+*sel))
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func jsStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func cmdDownloads(args []string) {
	fs := flag.NewFlagSet("downloads", flag.ExitOnError)
	port := portFlag(fs)
	dir := fs.String("dir", "", "download directory (created if needed)")
	parseAny(fs, args)
	if *dir == "" && fs.NArg() > 0 {
		*dir = fs.Arg(0)
	}
	if *dir == "" {
		fatal(fmt.Errorf("usage: downloads --dir C:\\dl --port 9223"))
	}
	if err := os.MkdirAll(*dir, 0755); err != nil {
		fatal(err)
	}
	if err := SetDownloadDir(*port, *dir); err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "dir": *dir})
	logAction("downloads", map[string]any{"dir": *dir})
}

// cmdAsk pauses for the human (2FA codes, captchas): prints the prompt,
// reads ONE line from stdin, echoes JSON. The AI waits instead of failing.
func cmdAsk(args []string) {
	fs := flag.NewFlagSet("ask", flag.ExitOnError)
	prompt := fs.String("prompt", "", "what to ask the human")
	out := fs.String("out", "", "optional file to save the answer")
	parseAny(fs, args)
	if *prompt == "" && fs.NArg() > 0 {
		*prompt = strings.Join(fs.Args(), " ")
	}
	if *prompt == "" {
		*prompt = "input needed"
	}
	fmt.Println("ASK-HUMAN: " + *prompt)
	reader := bufio.NewReader(os.Stdin)
	raw, _ := reader.ReadString('\n')
	line := strings.TrimSpace(raw)
	if *out != "" && line != "" {
		_ = os.WriteFile(*out, []byte(line), 0600)
	}
	mustJSON(map[string]any{"ok": line != "", "answer": line})
	logAction("ask", map[string]any{"prompt": *prompt, "answered": line != ""})
}

// cmdAdopt gracefully migrates the ALREADY-RUNNING Brave into debug mode:
// WM_CLOSE (session/cookies flushed, unlike kill -9), stale lock cleanup,
// relaunch same profile with --restore-last-session, verify tabs are back.
func cmdAdopt(args []string) {
	fs := flag.NewFlagSet("adopt", flag.ExitOnError)
	port := fs.Int("port", 9223, "debug port for the relaunched Brave")
	profile := fs.String("profile", defaultBraveProfile(), "user-data-dir to keep")
	binary := fs.String("browser", "", "brave/chrome/edge exe (auto)")
	parseAny(fs, args)
	if err := AdoptBrowser(*binary, *port, *profile); err != nil {
		fatal(err)
	}
	tabs, err := ListTabs(*port)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{"ok": true, "port": *port, "tabs": len(tabs)})
	logAction("adopt", map[string]any{"port": *port, "tabs": len(tabs)})
}

// cmdBench runs the rerunnable live benchmark (see docs/BENCHMARKS.md).
func cmdBench(args []string) {
	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	port := portFlag(fs)
	tab := fs.String("tab", "", "tab id (default: active)")
	keys := fs.Int("keys", 20, "key events to time")
	parseAny(fs, args)
	rep, err := BenchRun(*port, *tab, *keys)
	if err != nil {
		fatal(err)
	}
	mustJSON(map[string]any{
		"ok": true, "pingMsAvg": rep.PingMs, "keysPerSec": rep.KeysPerS,
		"keysMs": rep.KeysMs, "keys": rep.Keys, "letters": rep.Letters,
		"extractMs": rep.ExtractMs, "shotMs": rep.ShotMs, "tabsMs": rep.TabsMs,
	})
	logAction("bench", map[string]any{"keysPerSec": rep.KeysPerS, "pingMs": rep.PingMs})
}

// cmdQuit closes ONLY the browser instance on the given debug port
// (Browser.close). Unlike taskkill it can never touch your real browser.
func cmdQuit(args []string) {
	fs := flag.NewFlagSet("quit", flag.ExitOnError)
	port := portFlag(fs)
	parseAny(fs, args)
	c, err := DialBrowser(*port)
	if err != nil {
		fatal(err)
	}
	_, _ = c.Send("Browser.close", map[string]any{})
	c.Close()
	mustJSON(map[string]any{"ok": true, "quit": *port})
	logAction("quit", map[string]any{"port": *port})
}

// cmdDoctor diagnoses this machine for any AI holding the tool: what works,
// what is stubbed, live debug-port scan, and (on non-Windows or --repair)
// the exact self-repair playbook.
func cmdDoctor(args []string) {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	repair := fs.Bool("repair", false, "print full self-repair playbook")
	parseAny(fs, args)
	ports := []int{}
	for _, p := range []int{9222, 9223, 9224} {
		if isDebuggingUp(p) {
			ports = append(ports, p)
		}
	}
	osOK := runtime.GOOS == "windows"
	mustJSON(map[string]any{
		"ok": true, "tool": "MORE THAN HUMAN", "goos": runtime.GOOS, "goarch": runtime.GOARCH,
		"cdpCore": "portable (works everywhere)", "osControl": map[string]any{"full": osOK, "mode": map[bool]string{true: "full Windows SendInput/screenshot/window API", false: "stubs — see repair playbook"}[osOK]},
		"debugPorts": ports,
	})
	if !osOK || *repair {
		fmt.Print(doctorRepair)
	}
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	api := fs.Int("api", 8080, "HTTP API port")
	port := portFlag(fs)
	parseAny(fs, args)
	if err := ServeAPI(*api, *port); err != nil {
		fatal(err)
	}
}
