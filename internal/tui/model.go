package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	bb "github.com/marcelofrau/speedy/internal/bufferbloat"
	ev "github.com/marcelofrau/speedy/internal/event"
	st "github.com/marcelofrau/speedy/internal/speedtest"
)

// Phase represents the current stage of the full test sequence.
type Phase int

const (
	PhaseInit Phase = iota
	PhaseFindingServer
	PhasePing
	PhaseDownload
	PhaseUpload
	PhaseBloatBaseline
	PhaseBloatDownload
	PhaseBloatUpload
	PhaseDone
	PhaseError
)

const (
	version        = "v0.1.0"
	sparklineLen   = 30
	bloatLen       = 50
	panelWidth     = 38
	arcWidth       = 34
	logVisibleRows = 12 // visible lines in the activity log panel
)

// globalSend is set from main.go via SetSend() before p.Run().
var globalSend func(tea.Msg)

// SetSend must be called from main.go with p.Send before p.Run().
func SetSend(fn func(tea.Msg)) {
	globalSend = fn
}

// Model is the Bubble Tea model for speedy.
type Model struct {
	phase          Phase
	phaseStartTime time.Time
	startTime      time.Time

	// user / server info
	ip      string
	isp     string
	server  string
	country string
	sponsor string

	// ping results
	pingMs   float64
	jitterMs float64

	// live download
	dlMbps    float64
	dlPercent float64
	dlHistory []float64

	// live upload
	ulMbps    float64
	ulPercent float64
	ulHistory []float64

	// speed test final result
	speedResult st.Result

	// bufferbloat
	bloatBaselineMs float64
	bloatDLHistory  []float64
	bloatULHistory  []float64
	bloatResult     bb.Result

	// activity log
	log []ev.MsgLog

	err error

	// UI components
	spinner    spinner.Model
	dlProgress progress.Model
	ulProgress progress.Model

	// terminal dimensions
	width  int
	height int
}

// NewModel creates a fresh Model ready to run.
func NewModel() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = StyleSpinner

	initialBarWidth := arcWidth

	dlProg := progress.New(
		progress.WithGradient(colorNeonBlue, colorNeonCyan),
		progress.WithoutPercentage(),
		progress.WithWidth(initialBarWidth),
	)
	ulProg := progress.New(
		progress.WithGradient(colorNeonGreen, colorNeonCyan),
		progress.WithoutPercentage(),
		progress.WithWidth(initialBarWidth),
	)

	now := time.Now()
	return Model{
		phase:          PhaseInit,
		phaseStartTime: now,
		startTime:      now,
		spinner:        sp,
		dlProgress:     dlProg,
		ulProgress:     ulProg,
		dlHistory:      make([]float64, 0, sparklineLen),
		ulHistory:      make([]float64, 0, sparklineLen),
		bloatDLHistory: make([]float64, 0, bloatLen),
		bloatULHistory: make([]float64, 0, bloatLen),
		log:            make([]ev.MsgLog, 0, logVisibleRows*2),
		width:          80,
		height:         24,
	}
}

// SetProgram is kept for compatibility but is now a no-op.
func (m *Model) SetProgram(_ interface{}) {}

// Init starts the spinner and kicks off the full test sequence.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		st.Run(func(msg tea.Msg) {
			if globalSend != nil {
				globalSend(msg)
			}
		}),
	)
}

// appendHistory appends a value to a sparkline slice, capping at maxLen.
func appendHistory(history []float64, val float64, maxLen int) []float64 {
	history = append(history, val)
	if len(history) > maxLen {
		history = history[len(history)-maxLen:]
	}
	return history
}

// appendLog appends a log entry, stamping Elapsed from startTime, capping at cap.
func appendLog(log []ev.MsgLog, entry ev.MsgLog, startTime time.Time, cap int) []ev.MsgLog {
	entry.Elapsed = time.Since(startTime).Round(time.Second)
	log = append(log, entry)
	if len(log) > cap*2 {
		log = log[len(log)-cap*2:]
	}
	return log
}

// progressWidth returns the width to use for the progress bar component.
func progressWidth(panelW int) int {
	w := panelW - 4
	if w < 10 {
		return 10
	}
	return w
}
