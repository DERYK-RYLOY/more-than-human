# BENCHMARKS — measured, rerunnable, no marketing numbers

Every figure below was produced by `bench` against a local headless
Brave over CDP on commodity hardware (Windows 11, 2026-10-15).
Rerun it yourself in 30 seconds:

```powershell
human-browser launch --port 9230 --headless
human-browser bench --port 9230 --keys 50
human-browser quit --port 9230
```

## Measured results (CDP round-trips, localhost)

| Operation | Result |
|---|---|
| Protocol ping (10× `Runtime.evaluate`) | **1.35 ms avg** |
| Key dispatch (50× down+up pairs) | **201 ms total = 248 keys/sec** |
| Full-passage extract (1,200 letters) | **5 ms** |
| Page screenshot (PNG) | **71 ms** |
| Tab list | **9 ms** |

What this means: the tool can physically deliver ~250 inputs/sec and read a
whole page in 5 ms. Paced runs (e.g. 45–62 WPM) are therefore 100% pacing,
0% tool overhead — every millisecond goes where the operator commands it.

## Field record (live sites, watched end-to-end)

| Run | Chars | Errors | Result |
|---|---|---|---|
| typing.com 1 Page Test (melons) | 1,213 | 0 | 41 WPM, 100% accuracy |
| typing.com 1 Page Test (Titanic) | 1,220 | 0 | **59 WPM, 100% accuracy** |
| typing.com 10-key 5-min test | 1,245 | 0 | 49 WPM, 100% accuracy |
| Mock self-heal (seeded damage) | 20 | 0 | auto-Redo + pass, attempts: 2 |

## Why it beats Selenium / Playwright / Puppeteer (architecture, not ads)

| | Selenium | Playwright | Puppeteer | MORE THAN HUMAN |
|---|---|---|---|---|
| Hops per input | test → driver HTTP → browser (3 processes) | test → Node → browser (2–3) | test → Node → browser (2–3) | **test → browser (1 direct WebSocket)** |
| Measured input rate here | — (not measured; bring your own rig) | — | — | **248 keys/sec** |
| OS-level mouse/keyboard (address bar, dialogs, menus) | No | No | No | **Yes** |
| Invisible/covered elements, first-class | No (JS escape hatches only) | Via evaluate only | Via evaluate only | **Yes (`hiddenWhy` + `jsclick`)** |
| Reads its own progress from the page | No | DIY | DIY | **Built in (cursor + error counts)** |
| Recovers itself (Redo + retry) | DIY | DIY | DIY | **Built in (`--retry`)** |
| Install weight | driver + bindings + browser | Node + browsers (~500 MB) | Node + Chrome | **One ~12 MB binary, no CGO** |
| Works headless AND headed, temp OR real profile | Headed needs display hacks | Yes | Yes | **Yes, plus graceful `adopt` of your real Brave** |

Rules of this document: our column is measured (rerun above); theirs is
architecture (process hops, install size) — checkable facts, not sampled
timings presented as universal. If anyone reproduces tighter fairly-measured
numbers for a competitor on the same rig, they belong here too.
