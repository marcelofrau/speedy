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
	sb.WriteString(m.renderMainArea())

	if m.phase == PhaseDone {
		sb.WriteString("\n\n")
		sb.WriteString(m.renderResultsBlock())
	}

	sb.WriteString("\n")
	return sb.String()
}

// renderMainArea renders the 4 metric panels + activity log.
// Wide terminals (≥wideThreshold): log goes to the right of the stacked panels.
// Narrow terminals (<wideThreshold): just the 4 panels; log is omitted (stepper
// and status bar already provide live feedback).
func (m Model) renderMainArea() string {
	leftCol := lipgloss.JoinVertical(lipgloss.Left,
		m.renderSpeedRow(),
		"\n",
		m.renderBloatRow(),
	)

	if m.width < wideThreshold {
		return leftCol
	}

	// Calculate log width from remaining space
	leftW := lipgloss.Width(leftCol)
	logW := m.width - leftW - 4 - 4 // 4 gap + 4 outer margin
	if logW < logMinWidth {
		return leftCol
	}

	logH := lipgloss.Height(leftCol)
	logPanel := m.renderLogPanelSized(logW, logH)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftCol, "    ", logPanel)
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
	phases []Phase // active during these phases
	done   []Phase // completed during these phases
}

var steps = []stepDef{
	{"IP & Server",
		[]Phase{PhaseInit, PhaseFindingServer},
		[]Phase{PhasePing, PhaseDownload, PhaseUpload, PhaseBloatBaseline, PhaseBloatDownload, PhaseBloatUpload, PhaseWaitKey, PhaseDone}},
	{"Ping",
		[]Phase{PhasePing},
		[]Phase{PhaseDownload, PhaseUpload, PhaseBloatBaseline, PhaseBloatDownload, PhaseBloatUpload, PhaseWaitKey, PhaseDone}},
	{"Download",
		[]Phase{PhaseDownload},
		[]Phase{PhaseUpload, PhaseBloatBaseline, PhaseBloatDownload, PhaseBloatUpload, PhaseWaitKey, PhaseDone}},
	{"Upload",
		[]Phase{PhaseUpload},
		[]Phase{PhaseBloatBaseline, PhaseBloatDownload, PhaseBloatUpload, PhaseWaitKey, PhaseDone}},
	{"Bufferbloat",
		[]Phase{PhaseBloatBaseline, PhaseBloatDownload, PhaseBloatUpload},
		[]Phase{PhaseWaitKey, PhaseDone}},
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
	case PhaseWaitKey:
		msg = StyleWaitKey.Render("✓  All tests complete  •  Press any key to see results")
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

	pingVal, jitterVal := dash, dash
	if m.pingMs > 0 {
		pingVal = StyleSpeedPing.Render(fmt.Sprintf("%.0f ms", m.pingMs))
		jitterVal = StyleSpeedPing.Render(fmt.Sprintf("%.0f ms", m.jitterMs))
	}

	return strings.Join([]string{
		"  " + StyleLabel.Render("Server") + bullet + "  " + serverVal,
		"  " + StyleLabel.Render("IP") + bullet + "  " + ipVal,
		"  " + StyleLabel.Render("Ping") + bullet + "  " + pingVal + StyleMuted.Render("    Jitter  ") + jitterVal,
	}, "\n")
}

// ── Speed panels ──────────────────────────────────────────────────────────────

func (m Model) renderSpeedRow() string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderDownloadPanel(),
		"  ",
		m.renderUploadPanel(),
	)
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
		spark = renderSparklineMultiRow(m.dlHistory, arcWidth, sparkRows, colorNeonBlue)
	} else {
		speed = StylePlaceholder.Render("— Mbps")
		arcBar = renderArcEmpty(arcWidth, colorMuted)
		gauge = renderEmptyBar(arcWidth)
		spark = renderSparklineMultiRowEmpty(arcWidth, sparkRows)
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
		spark = renderSparklineMultiRow(m.ulHistory, arcWidth, sparkRows, colorNeonGreen)
	} else {
		speed = StylePlaceholder.Render("— Mbps")
		arcBar = renderArcEmpty(arcWidth, colorMuted)
		gauge = renderEmptyBar(arcWidth)
		spark = renderSparklineMultiRowEmpty(arcWidth, sparkRows)
	}

	content := strings.Join([]string{heading, "", arcBar, speed, "", gauge, "", spark}, "\n")
	return style.Width(panelWidth).Render(content)
}

// ── Bufferbloat panels ────────────────────────────────────────────────────────

func (m Model) renderBloatRow() string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderBloatDownloadPanel(),
		"  ",
		m.renderBloatUploadPanel(),
	)
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
	var spark string
	if len(m.bloatDLHistory) > 0 {
		last := m.bloatDLHistory[len(m.bloatDLHistory)-1]
		currentLine = "Under load  " + renderLatencyValue(last, m.bloatBaselineMs)
		spark = renderLatencySparklineMultiRow(m.bloatDLHistory, arcWidth, sparkRows, colorNeonBlue)
	} else {
		spark = renderSparklineMultiRowEmpty(arcWidth, sparkRows)
	}

	gradeStr := ""
	if (m.phase == PhaseDone || m.phase == PhaseWaitKey) && m.bloatResult.DLGrade != "" {
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
	var spark string
	if len(m.bloatULHistory) > 0 {
		last := m.bloatULHistory[len(m.bloatULHistory)-1]
		currentLine = "Under load  " + renderLatencyValue(last, m.bloatBaselineMs)
		spark = renderLatencySparklineMultiRow(m.bloatULHistory, arcWidth, sparkRows, colorNeonGreen)
	} else {
		spark = renderSparklineMultiRowEmpty(arcWidth, sparkRows)
	}

	gradeStr := ""
	if (m.phase == PhaseDone || m.phase == PhaseWaitKey) && m.bloatResult.ULGrade != "" {
		gradeStr = "  " + renderGradeBadge(m.bloatResult.ULGrade)
	}

	content := strings.Join([]string{heading + gradeStr, "", baselineLine, currentLine, "", spark}, "\n")
	return style.Width(panelWidth).Render(content)
}

// ── Activity Log panel ────────────────────────────────────────────────────────

// renderLogPanelSized renders the log at a specific width and height.
func (m Model) renderLogPanelSized(width, height int) string {
	// inner content height = height - 2 (borders) - 2 (title + divider)
	innerH := height - 4
	if innerH < 1 {
		innerH = 1
	}

	title := StyleLogTitle.Render("Activity Log")
	innerW := width - 4 // subtract border+padding
	if innerW < 4 {
		innerW = 4
	}
	divider := StyleDim.Render(strings.Repeat("─", innerW))

	// Collect last innerH entries
	entries := m.log
	if len(entries) > innerH {
		entries = entries[len(entries)-innerH:]
	}

	lines := make([]string, 0, innerH)
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
		// truncate to fit inner width
		line := ts + "  " + text
		lines = append(lines, line)
	}

	// Pad to innerH so panel height is stable
	for len(lines) < innerH {
		lines = append(lines, "")
	}

	content := title + "\n" + divider + "\n" + strings.Join(lines, "\n")
	return StyleLogPanel.Width(width).Height(height).Render(content)
}

func fmtElapsed(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", m, s)
}

// ── Results block ─────────────────────────────────────────────────────────────

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
		renderLatencySparklineMultiRow(m.bloatDLHistory, 32, 2, bb.GradeColor(br.DLGrade))
	ulSpark := StyleMuted.Render("ul ping  ") +
		renderLatencySparklineMultiRow(m.bloatULHistory, 32, 2, bb.GradeColor(br.ULGrade))
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
	return box + "\n\n  " + StyleHint.Render("Press q to quit.")
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

// ── Multi-row sparkline ───────────────────────────────────────────────────────
//
// Each column represents one data point. The bar grows from the bottom up.
// For each row (top=rows, bottom=1) and each column:
//   - if normalized(value) >= row/rows → fill character (bright)
//   - else → empty character (very dim)
//
// This creates a proper bar-chart effect with `rows` lines of height.

func renderSparklineMultiRow(history []float64, width, rows int, fillColor string) string {
	data := padOrTrim(history, width)
	maxVal := maxSlice(data)
	if maxVal == 0 {
		maxVal = 1
	}
	normalized := make([]float64, len(data))
	for i, v := range data {
		normalized[i] = v / maxVal
	}
	return buildMultiRowSpark(normalized, width, rows,
		fillColor, colorDim, colorDim)
}

func renderLatencySparklineMultiRow(history []float64, width, rows int, fillColor string) string {
	data := padOrTrim(history, width)
	maxVal := maxSlice(data)
	if maxVal == 0 {
		maxVal = 1
	}
	normalized := make([]float64, len(data))
	for i, v := range data {
		normalized[i] = v / maxVal
	}
	return buildMultiRowSpark(normalized, width, rows,
		fillColor, colorDim, colorDim)
}

func renderSparklineMultiRowEmpty(width, rows int) string {
	lines := make([]string, rows)
	empty := StylePlaceholder.Render(strings.Repeat("░", width))
	for i := range lines {
		lines[i] = empty
	}
	return strings.Join(lines, "\n")
}

// buildMultiRowSpark constructs a multi-row bar chart string.
// normalized: slice of values 0.0–1.0, one per column.
// rows: number of terminal lines tall.
// fillColor: color of filled bars.
// fillDimColor: color of the slightly dimmer top row (depth effect).
// emptyColor: color of empty cells.
func buildMultiRowSpark(normalized []float64, width, rows int, fillColor, fillDimColor, emptyColor string) string {
	fillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(fillColor))
	fillDimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(fillDimColor))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor))

	lines := make([]string, rows)
	for row := rows; row >= 1; row-- {
		// threshold: fraction of the bar that must be filled to light up this row
		threshold := float64(row-1) / float64(rows)
		var sb strings.Builder
		for col := 0; col < width; col++ {
			v := 0.0
			if col < len(normalized) {
				v = normalized[col]
			}
			if v >= threshold+1.0/float64(rows) {
				// fully filled row — use dim style at very top for depth
				if row == rows && v < 1.0 {
					sb.WriteString(fillDimStyle.Render("█"))
				} else {
					sb.WriteString(fillStyle.Render("█"))
				}
			} else if v > threshold {
				// partial fill — use a fractional block char
				frac := (v - threshold) * float64(rows)
				idx := clampIdx(int(math.Round(frac*float64(len(SparkChars)-1))), len(SparkChars))
				sb.WriteString(fillStyle.Render(SparkChars[idx]))
			} else {
				sb.WriteString(emptyStyle.Render("░"))
			}
		}
		lines[rows-row] = sb.String()
	}
	return strings.Join(lines, "\n")
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

	fillStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(fillColor))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor))

	// Two rows: top row slightly dimmer for depth
	topFilled := int(math.Round(float64(filled) * 0.85)) // slightly shorter top
	bar1 := fillStyle.Render(strings.Repeat("█", topFilled)) +
		emptyStyle.Render(strings.Repeat("░", width-topFilled))
	bar2 := fillStyle.Render(strings.Repeat("█", filled)) +
		emptyStyle.Render(strings.Repeat("░", empty))

	labelPad := width - 1 - len("1000 Mbps")
	if labelPad < 1 {
		labelPad = 1
	}
	labels := emptyStyle.Render("0") +
		strings.Repeat(" ", labelPad) +
		emptyStyle.Render("1000 Mbps")

	return bar1 + "\n" + bar2 + "\n" + labels
}

func renderArcEmpty(width int, emptyColor string) string {
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(emptyColor))
	bar := emptyStyle.Render(strings.Repeat("░", width))
	labelPad := width - 1 - len("1000 Mbps")
	if labelPad < 1 {
		labelPad = 1
	}
	labels := emptyStyle.Render("0") +
		strings.Repeat(" ", labelPad) +
		emptyStyle.Render("1000 Mbps")
	return bar + "\n" + bar + "\n" + labels
}

func renderEmptyBar(width int) string {
	return StylePlaceholder.Render(strings.Repeat("░", width))
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
