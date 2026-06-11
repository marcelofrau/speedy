<div align="center">

# ⚡ speedy

**A modern, beautiful terminal speedtest — built with Go + Bubble Tea**

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-bb9af7?logo=opensourceinitiative&logoColor=white)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/marcelofrau/speedy)](https://goreportcard.com/report/github.com/marcelofrau/speedy)
[![Release](https://img.shields.io/github/v/release/marcelofrau/speedy?color=f7768e)](https://github.com/marcelofrau/speedy/releases)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macos%20%7C%20windows-565f89)](https://github.com/marcelofrau/speedy/releases)

</div>

---

`speedy` is a terminal speedtest TUI with live gauges, arc speedometers, and sparkline graphs — powered by the [Speedtest.net](https://speedtest.net) network.

```
╭──────────────────────────────────────────────────────────╮
│  ⚡ speedy                                      v0.1.0   │
╰──────────────────────────────────────────────────────────╯

  Server  •  São Paulo, BR  (Claro)
  IP      •  177.82.x.x  •  Vivo Fibra
  Ping    •  12 ms  •  Jitter  2 ms

╭──────────────────────────╮  ╭──────────────────────────╮
│  ↓  DOWNLOAD             │  │  ↑  UPLOAD               │
│                          │  │                          │
│  ████████████░░░░░░░░░░  │  │  ██████░░░░░░░░░░░░░░░░  │
│  0                1000   │  │  0                1000   │
│  342.1 Mbps              │  │  180.5 Mbps              │
│                          │  │                          │
│  ████████████████░░░░░   │  │  ██████████░░░░░░░░░░░   │
│                          │  │                          │
│  ▁▂▃▄▅▆▇█▇▆▅▇██▇█▇█     │  │  ▁▁▂▃▄▅▆▇██▇▆▅▄▃▂        │
╰──────────────────────────╯  ╰──────────────────────────╯
```

## Features

- **Live arc speedometer** — animated Unicode block bar showing current speed vs 1 Gbps max
- **Animated progress gauge** — smooth fill bar with gradient color (blue for download, green for upload)
- **Live sparkline** — rolling history graph of speed variation during the test
- **Ping + Jitter** — measured before the speed test
- **Server auto-selection** — picks the closest / lowest-latency Speedtest.net server
- **IP + ISP info** — displays your public IP and internet provider
- **Tokyo Night color palette** — purple, blue, green, pink on a dark background
- **Single binary** — no runtime dependencies, no Docker, no Python

## Install

### Pre-built binary (recommended)

Download the latest binary for your platform from the [Releases](https://github.com/marcelofrau/speedy/releases) page.

```sh
# Linux / macOS
chmod +x speedy
sudo mv speedy /usr/local/bin/speedy

# Windows: move speedy.exe to a directory in your PATH
```

### Go install

```sh
go install github.com/marcelofrau/speedy@latest
```

### Build from source

```sh
git clone https://github.com/marcelofrau/speedy.git
cd speedy
go build -o speedy .
./speedy
```

Requires **Go 1.22+**.

## Usage

```sh
speedy
```

That's it. No flags needed.

| Key | Action |
|-----|--------|
| `q` / `Q` / `Ctrl+C` | Quit |

## Metrics

| Metric | Description |
|--------|-------------|
| **Ping** | Round-trip latency to the test server (ms) |
| **Jitter** | Variation in ping latency (ms) |
| **Download** | Measured download speed (Mbps) |
| **Upload** | Measured upload speed (Mbps) |
| **Server** | Name, country, and sponsor of the selected test server |
| **IP / ISP** | Your public IP address and internet service provider |

## Bufferbloat

After the speed test, `speedy` automatically runs a **bufferbloat test** — the most useful metric most speed tests skip.

**What is bufferbloat?**
When your connection is fully loaded (streaming, uploading, gaming), your router's buffers fill up and your latency explodes — even if your raw speed is high. This ruins video calls, gaming, and VoIP even on fast connections.

**How speedy measures it:**
1. Measures a baseline idle ping to the test server
2. Saturates the download path with 8 parallel streams while pinging every 200ms for 10 seconds
3. Repeats for upload with 4 parallel streams
4. Calculates latency degradation (peak-under-load − baseline) and assigns a grade

**Grading scale** (same as [Waveform's test](https://www.waveform.com/tools/bufferbloat)):

| Grade | Degradation |
|-------|-------------|
| **A+** | < 5 ms — excellent |
| **A** | < 30 ms — good |
| **B** | < 60 ms — acceptable |
| **C** | < 200 ms — poor |
| **D** | < 400 ms — bad |
| **F** | ≥ 400 ms — unusable |

**Example results screen:**

```
╭──────────────────────────────────────────────────────╮
│                    BUFFERBLOAT                       │
│  ────────────────────────────────────────────────    │
│                                                      │
│  Baseline    12 ms                                   │
│                                                      │
│  Download    A   peak 41 ms  (+29 ms)                │
│  dl latency  ▁▁▂▂▃▃▄▄▅▅▆▆▇▇▆▅▄▃▂▁▁▂▃▄▅▄▃▂▁▁         │
│                                                      │
│  Upload      B   peak 68 ms  (+56 ms)                │
│  ul latency  ▁▁▂▃▄▅▆▇█▇▆▅▄▃▄▅▆▇█▇▆▅▄▃▂▁▁▂▃▄▅         │
│                                                      │
│  Overall     B                                       │
╰──────────────────────────────────────────────────────╯
```

## Architecture

```
speedy/
├── main.go                        # Entrypoint — tea.NewProgram
├── internal/
│   ├── speedtest/
│   │   └── runner.go              # Speedtest sequence: ping → download → upload
│   │                              # then hands off to bufferbloat runner
│   ├── bufferbloat/
│   │   └── runner.go              # Baseline ping → saturate download + ping
│   │                              # → saturate upload + ping → grade result
│   └── tui/
│       ├── model.go               # Model struct, Init(), phase constants
│       ├── update.go              # Update() — state machine transitions
│       ├── view.go                # View() — renders arc, gauge, sparkline,
│       │                          #          bufferbloat panels and results
│       └── styles.go              # Lip Gloss palette (Tokyo Night / Dracula)
```

The TUI follows the [Elm Architecture](https://guide.elm-lang.org/architecture/) via [Bubble Tea](https://github.com/charmbracelet/bubbletea). Both runners execute in the same goroutine (sequentially) and send progress messages to the program via `program.Send()`, driving state transitions and live re-renders.

**State machine:**
```
Idle → Initializing → FindingServer → Ping
     → Download → Upload
     → BloatBaseline → BloatDownload → BloatUpload
     → Done
```

## Dependencies

| Library | Purpose |
|---------|---------|
| [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) | TUI framework (Elm Architecture) |
| [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) | Progress bar, spinner components |
| [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) | Styling, borders, layout |
| [showwin/speedtest-go](https://github.com/showwin/speedtest-go) | Speedtest.net network client (unofficial) |

The bufferbloat runner has **no additional dependencies** — it uses only Go's standard `net/http` library.

## Roadmap

- [ ] Semicircle speedometer with animated needle
- [ ] `--server <id>` flag to select a specific test server
- [ ] `--json` output for scripting
- [ ] Save results history to `~/.speedy/history.json`
- [ ] Homebrew tap
- [ ] `--no-bloat` flag to skip bufferbloat test

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Commit your changes with a clear message
4. Open a pull request

Please open an issue first for large changes.

## License

[MIT](LICENSE) © 2025 Marcelo Frau
