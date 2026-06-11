package tui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	bb "github.com/marcelofrau/speedy/internal/bufferbloat"
	ev "github.com/marcelofrau/speedy/internal/event"
)

// ── Top-level View ────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.phase == PhaseError {
		return m.renderError()
	}

	var sb strings.Builder
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n\n")
	sb.WriteString(m.renderStepper())
	sb.WriteString("\n")
	sb.WriteString(m.renderStatusBar())
	sb.WriteString("\n\n")
	sb.WriteString(m.renderInfoBlock())
	sb.WriteString("\n\n")
	sb.WriteString(m.renderSpeedRow())
	sb.WriteString("\n\n")
	sb.WriteString(m.renderBloatRow())
	sb.WriteString("\n\n")
	sb.WriteString(m.renderLogPanel())

	// When done, append the results block below everything
	if m.phase == PhaseDone {
		sb.WriteString("\n\n")
		sb.WriteString(m.renderResultsBlock())
	}

	sb.WriteString("\n")
	return sb.String()
}

// ── Header ────────────────────────────────────────────────────────────────────

func (m Model) renderHeader() string {
	title := StyleTitle.Render("⚡ speedy")
	ver := StyleVersion.Render(version)
	gap := m.width - lipgloss.Width(title) - lipgloss.Width(ver) - 6
	if gap < 1 {
		gap = 1
	}
	line := title + strings.Repeat(" ", gap) + ver
	return StyleHeader.Width(m.width - 4).Render(line)
}

// ── Stepper ───────────────────────────────────────────────────────────────────

type stepDef struct {
	label  string
	phases []Phase
	done   []Phase
}

var steps = []stepDef{
	{"IP & Server",
		[]Phase{PhaseInit, PhaseFindingServer},
		[]Phase{PhasePing, PhaseDownload, PhaseUpload, PhaseBloatBaseline, PhaseBloatDownload, PhaseBloatUpload, PhaseDone}},
	{"Ping",
		[]Phase{PhasePing},
		[]Phase{PhaseDownload, PhaseUpload, PhaseBloatBaseline, PhaseBloatDownload, PhaseBloatUpload, PhaseDone}},
	{"Download",
		[]Phase{PhaseDownload},
		[]Phase{PhaseUpload, PhaseBloatBaseline, PhaseBloatDownload, PhaseBloatUpload, PhaseDone}},
	{"Upload",
		[]Phase{PhaseUpload},
		[]Phase{PhaseBloatBaseline, PhaseBloatDownload, PhaseBloatUpload, PhaseDone}},
	{"Bufferbloat",
		[]Phase{PhaseBloatBaseline, PhaseBloatDownload, PhaseBloatUpload},
		[]Phase{PhaseDone}},
}

func phaseIn(p Phase, list []Phase) bool {
	for _, v := range list {
		if p == v {
			return true
		}
	}
	return false
}

func (m Model) renderStepper() string {
	sep := StyleStepSep.Render("  ─  ")
	var parts []string
	for _, s := range steps {
		var icon, label string
		switch {
		case phaseIn(m.phase, s.done):
			icon = StyleStepDone.Render("✓")
			label = StyleStepLabelDone.Render(s.label)
		case phaseIn(m.phase, s.phases):
			icon = StyleStepActive.Render(m.spinner.View())
			label = StyleStepLabelActive.Render(s.label)
		default:
			icon = StyleStepPending.Render("○")
			label = StyleStepLabel.Render(s.label)
		}
		parts = append(parts, icon+" "+label)
	}
	return "  " + strings.Join(parts, sep)
}

// ── Status bar ────────────────────────────────────────────────────────────────

func (m Model) renderStatusBar() string {
	elapsed := time.Since(m.phaseStartTime).Round(time.Second)

	var msg string
	switch m.phase {
	case PhaseInit:
		msg = "Connecting to Speedtest.net..."
	case PhaseFindingServer:
		msg = fmt.Sprintf("Fetching server list and measuring latency to nearby servers...  %s",
			StyleDim.Render(elapsed.String()))
	case PhasePing:
		srv := m.server
		if srv == "" {
			srv = "server"
		}
		msg = fmt.Sprintf("Measuring ping and jitter to %s...  %s",
			StyleStatusHighlight.Render(srv),
			StyleDim.Render(elapsed.String()))
	case PhaseDownload:
		msg = fmt.Sprintf("Measuring download speed — saturating link with multiple streams...  %s",
			StyleDim.Render(elapsed.String()))
	case PhaseUpload:
		msg = fmt.Sprintf("Measuring upload speed — saturating link with multiple streams...  %s",
			StyleDim.Render(elapsed.String()))
	case PhaseBloatBaseline:
		msg = fmt.Sprintf("Bufferbloat: measuring idle baseline ping (10 probes)...  %s",
			StyleDim.Render(elapsed.String()))
	case PhaseBloatDownload:
		remaining := 10 - int(elapsed.Seconds())
		if remaining < 0 {
			remaining = 0
		}
		msg = fmt.Sprintf("Bufferbloat: saturating download while pinging every 200 ms...  %s remaining",
			StyleStatusHighlight.Render(fmt.Sprintf("~%ds", remaining)))
	case PhaseBloatUpload:
		remaining := 10 - int(elapsed.Seconds())
		if remaining < 0 {
			remaining = 0
		}
		msg = fmt.Sprintf("Bufferbloat: saturating upload while pinging every 200 ms...  %s remaining",
			StyleStatusHighlight.Render(fmt.Sprintf("~%ds", remaining)))
	case PhaseDone:
		total := time.Since(m.startTime).Round(time.Second)
		msg = fmt.Sprintf("All tests complete  •  %s", StyleStatusHighlight.Render(total.String()))
	}

	return "  " + StyleStatusBar.Render(msg)
}

// ── Info block ────────────────────────────────────────────────────────────────

func (m Model) renderInfoBlock() string {
	bullet := StyleBullet.String()
	dash := StyleDim.Render("—")

	serverVal := dash
	if m.server != "" {
		loc := m.server
		if m.country != "" {
			loc += ", " + m.country
		}
		if m.sponsor != "" {
			loc += "  (" + m.sponsor + ")"
		}
		serverVal = StyleValue.Render(loc)
	}

	ipVal := dash
	if m.ip != "" {
		ip := m.ip
		if m.isp != "" {
			ip += "  " + bullet + "  " + m.isp
		}
		ipVal = StyleValue.Render(ip)
	}

	pingVal := dash
	jitterVal := dash
	if m.pingMs > 0 {
		pingVal = StyleSpeedPing.Render(fmt.Sprintf("%.0f ms", m.pingMs))
		jitterVal = StyleSpeedPing.Render(fmt.Sprintf("%.0f ms", m.jitterMs))
	}

	serverLine := "  " + StyleLabel.Render("Server") + bullet + "  " + serverVal
	ipLine := "  " + StyleLabel.Render("IP") + bullet + "  " + ipVal
	pingLine := "  " + StyleLabel.Render("Ping") + bullet + "  " + pingVal +
		StyleMuted.Render("    Jitter  ") + jitterVal

	return strings.Join([]string{serverLine, ipLine, pingLine}, "\n")
}

// ── Speed panels ──────────────────────────────────────────────────────────────

func (m Model) renderSpeedRow() string {
	left := m.renderDownloadPanel()
	right := m.renderUploadPanel()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m Model) renderDownloadPanel() string {
	active := m.phase == PhaseDownload
	done := phaseIn(m.phase, steps[2].done)
	style := StylePanel
	if active {
		style = StylePanelActive
	}

	heading := StyleSectionDown.Render("↓  DOWNLOAD")

	var speed, arcBar, gauge, spark string
	if m.dlMbps > 0 || active || done {
		speed = renderSpeedLine(m.dlMbps, StyleSpeedDown)
		arcBar = renderArc(m.dlPercent, arcWidth, colorNeonBlue, colorMuted)
		gauge = m.dlProgress.View()
		spark = renderSparkline(m.dlHistory, arcWidth)
	} else {
		speed = StylePlaceholder.Render("— Mbps")
		arcBar = renderArcEmpty(arcWidth, colorMuted)
		gauge = renderEmptyBar(arcWidth)
		spark = StylePlaceholder.Render(strings.Repeat("▁", arcWidth))
	}

	content := strings.Join([]string{heading, "", arcBar, speed, "", gauge, "", spark}, "\n")
	return style.Width(panelWidth).Render(content)
}

func (m Model) renderUploadPanel() string {
	active := m.phase == PhaseUpload
	done := phaseIn(m.phase, steps[3].done)
	style := StylePanel
	if active {
		style = StylePanelActive
	}

	heading := StyleSectionUp.Render("↑  UPLOAD")

	var speed, arcBar, gauge, spark string
	if m.ulMbps > 0 || active || done {
		speed = renderSpeedLine(m.ulMbps, StyleSpeedUp)
		arcBar = renderArc(m.ulPercent, arcWidth, colorNeonGreen, colorMuted)
		gauge = m.ulProgress.View()
		spark = renderSparkline(m.ulHistory, arcWidth)
	} else {
		speed = StylePlaceholder.Render("— Mbps")
		arcBar = renderArcEmpty(arcWidth, colorMuted)
		gauge = renderEmptyBar(arcWidth)
		spark = StylePlaceholder.Render(strings.Repeat("▁", arcWidth))
	}

	content := strings.Join([]string{heading, "", arcBar, speed, "", gauge, "", spark}, "\n")
	return style.Width(panelWidth).Render(content)
}

// ── Bufferbloat panels ────────────────────────────────────────────────────────

func (m Model) renderBloatRow() string {
	left := m.renderBloatDownloadPanel()
	right := m.renderBloatUploadPanel()
	return lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
}

func (m Model) renderBloatDownloadPanel() string {
	active := m.phase == PhaseBloatDownload
	style := StylePanel
	if active {
		style = StylePanelActive
	}

	heading := StyleSectionDown.Render("↓  BUFFERBLOAT — DL")

	baselineLine := "Baseline    " + StylePlaceholder.Render("— ms")
	if m.bloatBaselineMs > 0 {
		baselineLine = "Baseline    " + StyleSpeedPing.Render(fmt.Sprintf("%.0f ms", m.bloatBaselineMs))
	}

	currentLine := "Under load  " + StylePlaceholder.Render("— ms")
	spark := StylePlaceholder.Render(strings.Repeat("▁", arcWidth))
	if len(m.bloatDLHistory) > 0 {
		last := m.bloatDLHistory[len(m.bloatDLHistory)-1]
		currentLine = "Under load  " + renderLatencyValue(last, m.bloatBaselineMs)
		spark = renderLatencySparkline(m.bloatDLHistory, arcWidth, colorNeonBlue)
	}

	// Show grade if done
	gradeStr := ""
	if m.phase == PhaseDone && m.bloatResult.DLGrade != "" {
		gradeStr = "  " + renderGradeBadge(m.bloatResult.DLGrade)
	}

	content := strings.Join([]string{heading + gradeStr, "", baselineLine, currentLine, "", spark}, "\n")
	return style.Width(panelWidth).Render(content)
}

func (m Model) renderBloatUploadPanel() string {
	active := m.phase == PhaseBloatUpload
	style := StylePanel
	if active {
		style = StylePanelActive
	}

	heading := StyleSectionUp.Render("↑  BUFFERBLOAT — UL")

	baselineLine := "Baseline    " + StylePlaceholder.Render("— ms")
	if m.bloatBaselineMs > 0 {
		baselineLine = "Baseline    " + StyleSpeedPing.Render(fmt.Sprintf("%.0f ms", m.bloatBaselineMs))
	}

	currentLine := "Under load  " + StylePlaceholder.Render("— ms")
	spark := StylePlaceholder.Render(strings.Repeat("▁", arcWidth))
	if len(m.bloatULHistory) > 0 {
		last := m.bloatULHistory[len(m.bloatULHistory)-1]
		currentLine = "Under load  " + renderLatencyValue(last, m.bloatBaselineMs)
		spark = renderLatencySparkline(m.bloatULHistory, arcWidth, colorNeonGreen)
	}

	gradeStr := ""
	if m.phase == PhaseDone && m.bloatResult.ULGrade != "" {
		gradeStr = "  " + renderGradeBadge(m.bloatResult.ULGrade)
	}

	content := strings.Join([]string{heading + gradeStr, "", baselineLine, currentLine, "", spark}, "\n")
	return style.Width(panelWidth).Render(content)
}

// ── Activity Log panel ────────────────────────────────────────────────────────

func (m Model) renderLogPanel() string {
	title := StyleLogTitle.Render("Activity Log")

	// Build visible lines — last logVisibleRows entries
	entries := m.log
	if len(entries) > logVisibleRows {
		entries = entries[len(entries)-logVisibleRows:]
	}

	lines := make([]string, 0, logVisibleRows)
	for _, e := range entries {
		ts := StyleLogTimestamp.Render(fmtElapsed(e.Elapsed))
		var text string
		switch e.Level {
		case ev.LogSuccess:
			text = StyleLogSuccess.Render(e.Text)
		case ev.LogData:
			text = StyleLogData.Render(e.Text)
		case ev.LogWarn:
			text = StyleLogWarn.Render(e.Text)
		default:
			text = StyleLogInfo.Render(e.Text)
		}
		lines = append(lines, ts+"  "+text)
	}

	// Pad to logVisibleRows so the panel height is stable
	for len(lines) < logVisibleRows {
		lines = append(lines, "")
	}

	innerW := m.width - 8 // account for panel padding + border
	if innerW < 20 {
		innerW = 20
	}

	content := title + "\n" + strings.Repeat("─", innerW) + "\n" +
		strings.Join(lines, "\n")

	return StyleLogPanel.Width(m.width - 4).Render(content)
}

func fmtElapsed(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

// ── Results block (appended in-place when PhaseDone) ─────────────────────────

func (m Model) renderResultsBlock() string {
	r := m.speedResult
	br := m.bloatResult

	row := func(k, v string, vs lipgloss.Style) string {
		return StyleResultKey.Render(k) + vs.Render(v)
	}

	speedTitle := StyleResultsTitle.Width(54).Render("SPEED TEST")
	sdiv := StyleDim.Render(strings.Repeat("─", 54))
	pingRow := row("Ping", fmt.Sprintf("%.0f ms", r.PingMs), StyleSpeedPing) +
		"   " + StyleResultKey.Render("Jitter") + StyleSpeedPing.Render(fmt.Sprintf("%.0f ms", r.JitterMs))
	dlRow := row("Download", fmt.Sprintf("%.1f Mbps", r.DownloadMbps), StyleSpeedDown) +
		"   " + StyleResultKey.Render("Upload") + StyleSpeedUp.Render(fmt.Sprintf("%.1f Mbps", r.UploadMbps))
	serverRow := row("Server", r.ServerName+", "+r.Country+" ("+r.Sponsor+")", StyleResultVal)
	ipRow := row("IP", r.IP, StyleResultVal) +
		"   " + StyleResultKey.Render("ISP") + StyleResultVal.Render(r.ISP)

	bloatTitle := StyleResultsTitle.Width(54).Render("BUFFERBLOAT")
	bdiv := StyleDim.Render(strings.Repeat("─", 54))
	baselineRow := row("Baseline", fmt.Sprintf("%.0f ms  idle", br.BaselineMs), StyleSpeedPing)
	dlBloatRow := StyleResultKey.Render("Download") +
		renderGradeBadge(br.DLGrade) +
		StyleMuted.Render(fmt.Sprintf("  peak %.0f ms  (+%.0f ms)", br.DLPeakMs, br.DLDegradationMs))
	ulBloatRow := StyleResultKey.Render("Upload") +
		renderGradeBadge(br.ULGrade) +
		StyleMuted.Render(fmt.Sprintf("  peak %.0f ms  (+%.0f ms)", br.ULPeakMs, br.ULDegradationMs))
	overallRow := StyleResultKey.Render("Overall") + renderGradeBadge(br.OverallGrade)
	dlSpark := StyleMuted.Render("dl ping  ") +
		renderLatencySparkline(m.bloatDLHistory, 32, bb.GradeColor(br.DLGrade))
	ulSpark := StyleMuted.Render("ul ping  ") +
		renderLatencySparkline(m.bloatULHistory, 32, bb.GradeColor(br.ULGrade))
	total := row("Completed", time.Since(m.startTime).Round(time.Second).String(), StyleDim)

	content := strings.Join([]string{
		speedTitle, sdiv, "",
		pingRow, dlRow, "",
		serverRow, ipRow, "",
		bloatTitle, bdiv, "",
		baselineRow, "",
		dlBloatRow, dlSpark, "",
		ulBloatRow, ulSpark, "",
		overallRow, "",
		total,
	}, "\n")

	box := StyleResultsBox.Width(m.width - 4).Render(content)
	hint := "\n  " + StyleHint.Render("Press q to quit.")
	return box + hint
}

// ── Error screen ──────────────────────────────────────────────────────────────

func (m Model) renderError() string {
	var sb strings.Builder
	sb.WriteString(m.renderHeader())
	sb.WriteString("\n\n  ")
	sb.WriteString(StyleError.Render(fmt.Sprintf("Error: %v", m.err)))
	sb.WriteString("\n\n  ")
	sb.WriteString(StyleHint.Render("Press q to quit."))
	sb.WriteString("\n")
	return sb.String()
}

// ── Drawing helpers ───────────────────────────────────────────────────────────

func renderSpeedLine(mbps float64, style lipgloss.Style) string {
	return style.Render(fmt.Sprintf("%.1f", mbps)) + " " + StyleUnit.Render("Mbps")
}

func renderArc(percent float64, width int, fillColor, emptyColor string) string {
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	filled := int(math.Round(float64(width) * percent))
	empty := width - filled
	bar := lipgloss.NewStyle().Foreground(lipgloss.Color(fillColor)).Render(strings.Repeat(ArcFilled, filled)) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor)).Render(strings.Repeat(ArcEmpty, empty))
	labelPad := width - 1 - len("1000 Mbps")
	if labelPad < 1 {
		labelPad = 1
	}
	labels := lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor)).Render("0") +
		strings.Repeat(" ", labelPad) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor)).Render("1000 Mbps")
	return bar + "\n" + labels
}

func renderArcEmpty(width int, emptyColor string) string {
	bar := lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor)).Render(strings.Repeat(ArcEmpty, width))
	labelPad := width - 1 - len("1000 Mbps")
	if labelPad < 1 {
		labelPad = 1
	}
	labels := lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor)).Render("0") +
		strings.Repeat(" ", labelPad) +
		lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor)).Render("1000 Mbps")
	return bar + "\n" + labels
}

func renderEmptyBar(width int) string {
	return StylePlaceholder.Render(strings.Repeat("░", width))
}

func renderSparkline(history []float64, width int) string {
	if len(history) == 0 {
		return StyleDim.Render(strings.Repeat("▁", width))
	}
	maxVal := maxSlice(history)
	if maxVal == 0 {
		maxVal = 1
	}
	data := padOrTrim(history, width)
	var sb strings.Builder
	for _, v := range data {
		sb.WriteString(SparkChars[clampIdx(int(math.Round(v/maxVal*float64(len(SparkChars)-1))), len(SparkChars))])
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted)).Render(sb.String())
}

func renderLatencySparkline(history []float64, width int, fillColor string) string {
	if len(history) == 0 {
		return StyleDim.Render(strings.Repeat("▁", width))
	}
	maxVal := maxSlice(history)
	if maxVal == 0 {
		maxVal = 1
	}
	data := padOrTrim(history, width)
	var sb strings.Builder
	for _, v := range data {
		sb.WriteString(SparkChars[clampIdx(int(math.Round(v/maxVal*float64(len(SparkChars)-1))), len(SparkChars))])
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(fillColor)).Render(sb.String())
}

func renderLatencyValue(ms, baselineMs float64) string {
	deg := ms - baselineMs
	color := bb.GradeColor(bb.Grade(deg))
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).
		Render(fmt.Sprintf("%.0f ms", ms))
}

func renderGradeBadge(grade string) string {
	color := bb.GradeColor(grade)
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#0d0d1a")).
		Background(lipgloss.Color(color)).
		Padding(0, 1).
		Render(grade)
}

func padOrTrim(data []float64, width int) []float64 {
	if len(data) < width {
		padded := make([]float64, width-len(data))
		return append(padded, data...)
	}
	if len(data) > width {
		return data[len(data)-width:]
	}
	return data
}

func clampIdx(idx, max int) int {
	if idx < 0 {
		return 0
	}
	if idx >= max {
		return max - 1
	}
	return idx
}

func maxSlice(s []float64) float64 {
	m := 0.0
	for _, v := range s {
		if v > m {
			m = v
		}
	}
	return m
}
