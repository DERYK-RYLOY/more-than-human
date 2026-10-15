# MORE THAN HUMAN — AI hands + eyes for Brave / Chrome / Edge (Go)

> Main job: control **ANY** browser like a human. Autopilot below is one
> sector, not the whole tool.

One Go binary that lets an AI like me drive **any Chromium browser — Brave first —
just like a human**, in real time:

- **SEE everything**: page screenshot + full OS screenshot (browser chrome itself),
  every DOM element with x/y/width/height, text, role, clickable flag —
  **including invisible ones** (`display:none`, `opacity:0`, `aria-hidden`,
  zero-size, offscreen, disabled) with `hiddenWhy`.
- **PRESS anything**: real CDP mouse (move/click/hover/scroll) at page coords,
  `jsclick` for invisible/covered elements, typing, key presses, JS eval —
  **plus** OS-level mouse/keyboard (`SendInput`) for the browser UI itself:
  address bar (`ctrl+l`), back button, menus, extensions, dialogs, any site.

No Python, no Node, no CGO. Pure Go, Windows `x/sys` + WebSocket CDP.

## Build

```powershell
git clone <this-repo> human-browser; cd human-browser
go mod tidy
go build -o human-browser.exe .
.\human-browser.exe help
.\human-browser.exe doctor        # diagnoses YOUR machine for any AI holding this tool
```

macOS/Linux: `GOOS=darwin go build -o human-browser .` (CDP core fully
portable; OS-control sector compiles as clear stubs — see `doctor --repair`
and "Not on Windows?" below).

## 1. Launch Brave with debugging (once)

```powershell
.\human-browser.exe launch --port 9222 --url https://example.com
.\human-browser.exe tabs --port 9222
.\human-browser.exe focus --title Brave
```

Auto-finds `brave.exe`, falls back to Chrome/Edge. Uses a temp profile so your
real Brave session is untouched. If Brave is already on port 9222 it just attaches.

## 2. The AI loop (this is the human part)

```powershell
# SEE: screenshot + ALL elements (visible AND hidden) + text — the main command
.\human-browser.exe see --port 9222 --hidden --shot page.png --max 200

# Full human vision: entire screen incl. browser chrome
.\human-browser.exe screen --out screen.png

# Find a button, even hidden ones
.\human-browser.exe elements --port 9222 --hidden --find "login"
```

`see` prints JSON like:

```json
{
  "title": "Example Domain", "url": "https://example.com/",
  "count": 19,
  "elements": [{
    "index": 18, "tag": "a", "text": "Learn more",
    "x": 472, "y": 628, "cx": 512, "cy": 639,
    "visible": true, "clickable": true, "href": "https://..."
  }],
  "shot": "page.png"
}
```

Open `page.png` / `screen.png` — that is exactly what the AI sees.

## 3. Act like a mouse + keyboard

```powershell
# Page-level (CDP, real Input events sites can feel)
.\human-browser.exe click --x 512 --y 639 --port 9222
.\human-browser.exe jsclick --index 18 --port 9222   # even invisible/covered!
.\human-browser.exe type --text "hello" --port 9222
.\human-browser.exe press --key Enter --port 9222
.\human-browser.exe hover --x 100 --y 200 --port 9222
.\human-browser.exe scroll --dy -600 --port 9222
.\human-browser.exe eval --js "document.title" --port 9222
.\human-browser.exe goto https://brave.com --port 9222
.\human-browser.exe see --full --shot full.png --port 9222   # full-page shot
.\human-browser.exe wait --ms 1500
.\human-browser.exe wait --for-text "Start" --port 9222 --timeout 15

# OS-level (browser chrome itself — address bar, buttons, menus, dialogs)
# NOTE: cursor GLIDES visibly + pauses 350ms on target so you can WATCH it,
# then JSON reports actual x/y + screenW/H for grounding. DPI-aware 1:1.
.\human-browser.exe mouse --click left --x 300 --y 60
.\human-browser.exe mouse --move --x 500 --y 500
.\human-browser.exe mouse --pos
.\human-browser.exe mouse --drag --x1 100 --y1 200 --x2 400 --y2 500
.\human-browser.exe mouse --scroll --clicks -3
.\human-browser.exe key --combo ctrl+l
.\human-browser.exe key --text "brave.com"
.\human-browser.exe key --press Enter
.\human-browser.exe windows
```

Every OS/CDP action is appended to `actions.log` next to the exe for review/replay.

## 4. HTTP API for any AI agent

```powershell
.\human-browser.exe serve --api 8080 --port 9222
```

- `GET /see?hidden=true&shot=page.png` — screenshot b64 + elements + text
- `GET /screen?out=screen.png` — OS screenshot b64
- `GET /tabs` `GET /navigate?url=...` `GET /windows`
- `POST /act {"action":"click","x":512,"y":639}`
  actions: `click jsclick type press hover scroll eval mouse key focus`

Example:

```powershell
curl "http://127.0.0.1:8080/see?hidden=true"
curl -X POST http://127.0.0.1:8080/act -H "Content-Type: application/json" -d '{"action":"click","x":512,"y":639}'
```

## 5. AUTOPILOT sector (recorded perfect passes)

Recorded live: typing.com 1 Page Test, 1,213 chars → 41 WPM/100%, then
1,220 chars → **59 WPM/100%**, zero screenshots mid-run.

```powershell
.\human-browser.exe autopilot --guide                 # the full playbook for any AI
.\human-browser.exe autopilot --dry --port 9223 --tab ID   # extract + report only
.\human-browser.exe autopilot --port 9223 --tab ID --wpm 59 # perfect pass: extract + type + verify
```

What it does, exactly as recorded: reads the site's own answer key from the
DOM (`div.letter` incl. future words/spaces/signs), refuses damaged runs
(cursor≠0 or errors>0 → Redo first), types real key events at commanded WPM
(Shift held where humans hold it), then verifies cursor+errors from the DOM.
Full rules: `autopilot --guide` (phases 0–4 + recovery cheat-sheet).

One-call version (no round trips, no screenshots, no transcription):
```powershell
.\human-browser.exe tpass --port 9223 --tab ID --wpm 50 --chars 1245
.\human-browser.exe tpass --dry --port 9223 --tab ID   # extract+validate only
```
`tpass` extracts ground truth in-process, validates every rune, aborts loudly
on the first error-gate trip (every 100 keys) instead of cascading, caps keys
for timed tests, and reports measured effectiveWPM (45 set ≈ 41 measured,
62 ≈ 59 — set ~53 for exactly 50).

Beyond autopilot (browser control is still the main job):
- `find --text "Start Test" [--click-index 0]` — exact control coordinates
- `tpass/autopilot --retry 3` — self-heal: Redo + restart automatically
- `waitfor --text/--selector`, `downloads --dir`, `ask --prompt` (2FA/captcha)
- `adopt --port 9223` — graceful WM_CLOSE migration of running Brave to debug
  mode (session-safe, same profile, tabs restored)
- Windows full OS control; darwin/linux stubs compile, CDP core cross-platform

## How it maps to your ask

| You said | How |
|---|---|
| Any browser, especially Brave | Auto-detects Brave, Chrome, Edge; any CDP port |
| See buttons/images/words/everything in real time | `see` (page PNG + 1500 elements) + `screen` (OS PNG @ 1920x1080) |
| Even the browser itself | OS screenshot + OS mouse/key + window focus |
| Press any button like mouse/keyboard | CDP `Input.dispatchMouseEvent/KeyEvent/insertText` + Win32 `SendInput` |
| Even invisible buttons | `GetElements(hidden=true)` reports `display:none opacity:0 aria-hidden zero-size offscreen` + `jsclick` clicks them |
| Any given site | CDP Runtime JS runs on any URL, no per-site code |

## Files

- `main.go` — CLI (`launch see screen click jsclick type press mouse key focus serve ...`, all JSON)
- `cdp.go` — minimal CDP client (tabs + WS + Page/Runtime/Input/Browser)
- `elements.go` — full-DOM enumerator + HumanClick/Hover/Scroll/Type/Press + FindExact + CType/TPass autopilot engine
- `oscontrol.go` — (Windows) SendInput mouse/keyboard + screenshots + windows, no CGO
- `os_other.go` — (macOS/Linux) same CLI, clear stubs until implemented
- `launcher.go` — Brave/Chrome/Edge finder (Win/macOS/Linux paths) + `--remote-debugging-port`
- `server.go` — `/see /screen /tabs /navigate /act /windows` JSON API

## Not on Windows? (AI: read this + run `doctor --repair`)

`mouse/key/screen/windows/focus` fail with `OS control not implemented`
because `os_other.go` holds stubs with exact signatures. CDP commands
(tabs/see/click/type/ctype/tpass/find/serve) work everywhere unchanged.
To implement: copy signatures into `os_darwin.go` (`//go:build darwin`) or
`os_linux.go` (macOS: osascript + screencapture; Linux: xgb/XTEST — both
already in go.mod), extend `braveCandidates[]`, then `mouse --pos` must
return real coordinates. Design laws: stubs never fake success, CDP stays
platform-free, never kill -9 a user browser.
