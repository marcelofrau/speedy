# speedy — Agent Instructions

## Build & Run

- **Build:** `go build -o speedy .`
- **Run:** `./speedy` (requires internet — uses Speedtest.net)
- **No tests exist** — do not search for test files or test commands
- **No lint/typecheck config** — `go vet ./...` is standard, no additional tooling

## Architecture

- **Bubble Tea (Elm Architecture):** `internal/tui/model.go` (Model/Init), `internal/tui/update.go` (Update), `internal/tui/view.go` (View). Styles in `styles.go`.
- **Global `send` pattern (glue code):** `main.go` calls `tui.SetSend(func(msg tea.Msg) { p.Send(msg) })` *before* `p.Run()`. This is required because `tea.NewProgram` owns an internal copy of the model — goroutines in `speedtest.Run()` and `bufferbloat.Run()` use the global `send` to deliver progress messages back to the TUI.
- **State machine phases:** `PhaseInit → FindingServer → Ping → Download → Upload → BloatBaseline → BloatDownload → BloatUpload → WaitKey → Done` (defined in `internal/tui/model.go:17-29`).
- **Event package:** `internal/event/event.go` defines shared message types (`MsgLog`, `LogLevel`). Zero external dependencies — exist solely to prevent circular imports between `tui`, `speedtest`, and `bufferbloat`.
- **Speed runner:** `internal/speedtest/runner.go` — sequential: user info → server selection → ping → download (8 streams) → upload (4 streams). Uses `github.com/showwin/speedtest-go`.
- **Bufferbloat runner:** `internal/bufferbloat/runner.go` — baseline ping (10 probes) → saturate download + ping (8 workers, 10s) → saturate upload + ping (4 workers, 10s). Pure stdlib `net/http`, no extra deps.

## Key Details

- **Go 1.26.3** (per go.mod) — recent toolchain required
- **Controls:** `q`/`Q`/`Ctrl+C` quits; any key after tests complete advances to results
- **No Makefile, no CI, no pre-commit, no task runner** — this is a minimal single-binary project
- **No generated code, no migrations, no build artifacts** — all source is hand-written Go
- **Style:** Synthwave/Tokyo Night palette in `internal/tui/styles.go` — all colors as hex constants
