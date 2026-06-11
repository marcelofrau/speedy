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
// Wide terminals (≥wideThreshold): log goes to the right of the stacked panels
// with a 70/30 split (panels get 70%, log gets 30%).
// Narrow terminals (<wideThreshold): just the 4 panels; log is omitted (stepper
// and status bar already provide live feedback).
func (m Model) renderMainArea() string {
	if m.width < wideThreshold {
		leftCol := lipgloss.JoinVertical(lipgloss.Left,
			m.renderSpeedRow(panelWidth),
			"\n",
			m.renderBloatRow(panelWidth),
		)
		return leftCol
	}

	// Wide: split available space 70/30 between panels and log
	avail := m.width - 4 // reserve 4 chars for the inter-column gap
	panelsTotal := avail * 70 / 100
	logW := avail - panelsTotal

	if logW < logMinWidth {
		logW = logMinWidth
		panelsTotal = avail - logW
	}

	// Two panels per row with "  " gap → each panel width = (panelsTotal - 2) / 2
	pw := (panelsTotal - 2) / 2
	if pw < 30 {
		pw = 30
	}

	leftCol := lipgloss.JoinVertical(lipgloss.Left,
		m.renderSpeedRow(pw),
		"\n",
		m.renderBloatRow(pw),
	)

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

func (m Model) renderSpeedRow(pw int) string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderDownloadPanel(pw),
		"  ",
		m.renderUploadPanel(pw),
	)
}

func (m Model) renderDownloadPanel(pw int) string {
	aw := pw - 6 // inner width = panelWidth - border(2) - padding(4)
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
		arcBar = renderArc(m.dlPercent, aw, colorNeonBlue, colorMuted)
		m.dlProgress.Width = aw
		gauge = m.dlProgress.View()
		spark = renderSparklineMultiRow(m.dlHistory, aw, sparkRows, colorNeonBlue)
	} else {
		speed = StylePlaceholder.Render("— Mbps")
		arcBar = renderArcEmpty(aw, colorMuted)
		gauge = renderEmptyBar(aw)
		spark = renderSparklineMultiRowEmpty(aw, sparkRows)
	}

	content := strings.Join([]string{heading, "", arcBar, speed, "", gauge, "", spark}, "\n")
	return style.Width(pw).Render(content)
}

func (m Model) renderUploadPanel(pw int) string {
	aw := pw - 6
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
		arcBar = renderArc(m.ulPercent, aw, colorNeonGreen, colorMuted)
		m.ulProgress.Width = aw
		gauge = m.ulProgress.View()
		spark = renderSparklineMultiRow(m.ulHistory, aw, sparkRows, colorNeonGreen)
	} else {
		speed = StylePlaceholder.Render("— Mbps")
		arcBar = renderArcEmpty(aw, colorMuted)
		gauge = renderEmptyBar(aw)
		spark = renderSparklineMultiRowEmpty(aw, sparkRows)
	}

	content := strings.Join([]string{heading, "", arcBar, speed, "", gauge, "", spark}, "\n")
	return style.Width(pw).Render(content)
}

// ── Bufferbloat panels ────────────────────────────────────────────────────────

func (m Model) renderBloatRow(pw int) string {
	return lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderBloatDownloadPanel(pw),
		"  ",
		m.renderBloatUploadPanel(pw),
	)
}

func (m Model) renderBloatDownloadPanel(pw int) string {
	aw := pw - 6
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
		spark = renderLatencySparklineMultiRow(m.bloatDLHistory, aw, sparkRows, colorNeonBlue)
	} else {
		spark = renderSparklineMultiRowEmpty(aw, sparkRows)
	}

	// Grade on its own line to avoid heading overflow
	rows := []string{heading, "", baselineLine, currentLine, ""}
	if (m.phase == PhaseDone || m.phase == PhaseWaitKey) && m.bloatResult.DLGrade != "" {
		rows = append(rows, "Grade  "+renderGradeBadge(m.bloatResult.DLGrade))
		rows = append(rows, "")
	}
	rows = append(rows, spark)

	content := strings.Join(rows, "\n")
	return style.Width(pw).Render(content)
}

func (m Model) renderBloatUploadPanel(pw int) string {
	aw := pw - 6
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
		spark = renderLatencySparklineMultiRow(m.bloatULHistory, aw, sparkRows, colorNeonGreen)
	} else {
		spark = renderSparklineMultiRowEmpty(aw, sparkRows)
	}

	rows := []string{heading, "", baselineLine, currentLine, ""}
	if (m.phase == PhaseDone || m.phase == PhaseWaitKey) && m.bloatResult.ULGrade != "" {
		rows = append(rows, "Grade  "+renderGradeBadge(m.bloatResult.ULGrade))
		rows = append(rows, "")
	}
	rows = append(rows, spark)

	content := strings.Join(rows, "\n")
	return style.Width(pw).Render(content)
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

// ── Results block ─────────────────────────────────────────────────────────────

// gradeRank maps letter grades to a numeric rank (lower = better).
var gradeRank = map[string]int{
	"A+": 0, "A": 1, "B": 2, "C": 3, "D": 4, "F": 5,
}

// gradeDesc returns a short description for a given overall grade.
func gradeDesc(grade string) string {
	switch grade {
	case "A+":
		return "Latency is excellent under load.\nIdeal for all real-time applications."
	case "A":
		return "Latency increased slightly under load.\nMost applications will work great."
	case "B":
		return "Noticeable latency increase under load.\nSome real-time apps may be affected."
	case "C":
		return "Significant bufferbloat detected.\nVideo calls and gaming will suffer."
	case "D":
		return "Severe bufferbloat detected.\nReal-time apps will be heavily degraded."
	default:
		return "Extreme bufferbloat detected.\nYour connection is impaired under load."
	}
}

// activityStatus returns the display icon + style for an activity
// given the overall grade rank.
func activityStatus(overallRank, minRank int) string {
	switch {
	case overallRank <= minRank:
		return StyleTableOK.Render("✓")
	case overallRank == minRank+1:
		return StyleTableWarn.Render("⚠")
	default:
		return StyleTableFail.Render("✗")
	}
}

func (m Model) renderResultsBlock() string {
	// Column width: split available width in two, minus gap and outer margin
	colW := (m.width - 4 - 2 - 4) / 2 // 4 outer margin, 2 border, 4 gap
	if colW < 30 {
		colW = 30
	}

	left := m.renderSpeedCol(colW)
	right := m.renderBloatSummaryCol(colW)

	cols := lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	return cols + "\n\n  " + StyleHint.Render("Press q to quit.")
}

// renderSpeedCol renders the left results column: speed metrics + sparklines.
func (m Model) renderSpeedCol(w int) string {
	r := m.speedResult
	br := m.bloatResult
	innerW := w - 6 // border (2) + padding (2*2=4)

	sec := func(title string) string {
		return StyleResultsTitle.Render(title) + "\n" +
			StyleDim.Render(strings.Repeat("─", innerW))
	}

	row := func(k, v string, vs lipgloss.Style) string {
		return StyleResultKey.Render(k) + vs.Render(v)
	}

	// ── SPEED TEST section ──
	speedSec := sec("SPEED TEST")
	pingRow := row("Ping", fmt.Sprintf("%.0f ms", r.PingMs), StyleSpeedPing) +
		"  " + StyleResultKey.Render("Jitter") + StyleSpeedPing.Render(fmt.Sprintf("%.0f ms", r.JitterMs))

	// DL / UL side-by-side inside a sub-panel
	subW := (innerW - 3) / 2 // -3 for separator and spacing
	dlSubContent := StyleSectionDown.Render("↓  DOWNLOAD") + "\n\n" +
		StyleSpeedDown.Render(fmt.Sprintf("%.1f", r.DownloadMbps)) + " " + StyleUnit.Render("Mbps")
	ulSubContent := StyleSectionUp.Render("↑  UPLOAD") + "\n\n" +
		StyleSpeedUp.Render(fmt.Sprintf("%.1f", r.UploadMbps)) + " " + StyleUnit.Render("Mbps")

	dlBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorNeonBlue)).
		Padding(0, 1).
		Width(subW).
		Render(dlSubContent)
	ulBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorNeonGreen)).
		Padding(0, 1).
		Width(subW).
		Render(ulSubContent)
	speedBoxes := lipgloss.JoinHorizontal(lipgloss.Top, dlBox, " ", ulBox)

	// ── CONNECTION section ──
	connSec := sec("CONNECTION")
	serverRow := row("Server", r.ServerName+", "+r.Country, StyleResultVal)
	sponsorRow := row("Sponsor", r.Sponsor, StyleResultVal)
	ipRow := row("IP", r.IP, StyleResultVal)
	ispRow := row("ISP", r.ISP, StyleResultVal)

	// ── LATENCY HISTORY section ──
	sparkSec := sec("LATENCY HISTORY")
	sparkW := innerW
	if sparkW < 8 {
		sparkW = 8
	}
	dlSpark := StyleMuted.Render("dl load") + "\n" +
		renderLatencySparklineMultiRow(m.bloatDLHistory, sparkW, 3, bb.GradeColor(br.DLGrade))
	ulSpark := StyleMuted.Render("ul load") + "\n" +
		renderLatencySparklineMultiRow(m.bloatULHistory, sparkW, 3, bb.GradeColor(br.ULGrade))

	elapsed := time.Since(m.startTime).Round(time.Second)
	totalRow := StyleDim.Render("Completed in  ") + StyleResultVal.Render(elapsed.String())

	content := strings.Join([]string{
		speedSec, "",
		pingRow, "",
		speedBoxes,
		"",
		connSec, "",
		serverRow,
		sponsorRow,
		ipRow,
		ispRow,
		"",
		sparkSec, "",
		dlSpark, "",
		ulSpark, "",
		totalRow,
	}, "\n")

	return StyleResultsColLeft.Width(w).Render(content)
}

// renderGradeBig renders a large double-bordered grade badge.
func renderGradeBig(grade string) string {
	color := bb.GradeColor(grade)
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#0d0d1a")).
		Background(lipgloss.Color(color)).
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color(color)).
		Padding(0, 2).
		Render(grade)
}

// renderBloatSummaryCol renders the right results column: bufferbloat summary.
func (m Model) renderBloatSummaryCol(w int) string {
	br := m.bloatResult
	innerW := w - 6 // border (2) + padding (2*2=4)

	sec := func(title string) string {
		return StyleResultsTitle.Render(title) + "\n" +
			StyleDim.Render(strings.Repeat("─", innerW))
	}

	// ── Grade section ──
	gradeSec := sec("BUFFERBLOAT GRADE")
	gradeBig := renderGradeBig(br.OverallGrade)
	desc := gradeDesc(br.OverallGrade)
	var descLines []string
	for _, l := range strings.Split(desc, "\n") {
		descLines = append(descLines, StyleResultsGradeDesc.Render(l))
	}

	// ── Latency section ──
	latSec := sec("LATENCY")
	unloadedRow := StyleResultKey.Render("Unloaded") +
		StyleSpeedPing.Render(fmt.Sprintf("%.0f ms", br.BaselineMs))
	dlLatRow := StyleResultKey.Render("Under DL") +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(bb.GradeColor(br.DLGrade))).
			Render(fmt.Sprintf("+%.0f ms", br.DLDegradationMs)) +
		"  " + StyleDim.Render("["+br.DLGrade+"]")
	ulLatRow := StyleResultKey.Render("Under UL") +
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(bb.GradeColor(br.ULGrade))).
			Render(fmt.Sprintf("+%.0f ms", br.ULDegradationMs)) +
		"  " + StyleDim.Render("["+br.ULGrade+"]")

	// ── Your Connection table with borders ──
	connSec := sec("YOUR CONNECTION")

	type activity struct {
		name    string
		minRank int
	}
	activities := []activity{
		{"Web Browsing", 4},
		{"Audio Calls", 3},
		{"4K Streaming", 2},
		{"Video Conf.", 2},
		{"Low-lat. Gaming", 1},
	}

	overallRank := gradeRank[br.OverallGrade]

	// Column widths — nameW is dynamic so table fits innerW exactly
	// table total = nameW + 2 idealW + 3 separators (│) + 2 outer (│) = nameW + 2*idealW + 5
	idealW := 7
	nameW := innerW - 2*idealW - 5
	if nameW < 12 {
		nameW = 12
	}

	// border chars
	h, v := "─", "│"
	tl, tm, tr := "┌", "┬", "┐"
	ml, mm, mr := "├", "┼", "┤"
	bl, bm, br2 := "└", "┴", "┘"
	dim := func(s string) string { return StyleDim.Render(s) }

	hName := strings.Repeat(h, nameW+2)
	hIdeal := strings.Repeat(h, idealW+2)
	hNow := strings.Repeat(h, idealW+2)

	topBorder := dim(tl + hName + tm + hIdeal + tm + hNow + tr)
	midBorder := dim(ml + hName + mm + hIdeal + mm + hNow + mr)
	botBorder := dim(bl + hName + bm + hIdeal + bm + hNow + br2)

	cell := func(s string, cw int) string {
		return " " + lipgloss.NewStyle().Width(cw).Render(s) + " "
	}
	dv := dim(v)

	headerRow := dv + cell(StyleTableHeader.Render(""), nameW) +
		dv + cell(StyleTableHeader.Render("Ideal"), idealW) +
		dv + cell(StyleTableHeader.Render("Now"), idealW) + dv

	var actRows []string
	for _, a := range activities {
		idealIcon := StyleTableOK.Render("✓")
		nowIcon := activityStatus(overallRank, a.minRank)
		actRows = append(actRows,
			dv+cell(StyleMuted.Render(a.name), nameW)+
				dv+cell(idealIcon, idealW)+
				dv+cell(nowIcon, idealW)+dv,
		)
	}

	// ── Overall ──
	overallLine := StyleResultKey.Render("Overall") + renderGradeBig(br.OverallGrade)

	tableRows := []string{topBorder, headerRow, midBorder}
	for i, r := range actRows {
		tableRows = append(tableRows, r)
		if i < len(actRows)-1 {
			tableRows = append(tableRows, midBorder)
		}
	}
	tableRows = append(tableRows, botBorder)

	rows := []string{gradeSec, "", gradeBig, ""}
	rows = append(rows, descLines...)
	rows = append(rows, "",
		latSec, "",
		unloadedRow, dlLatRow, ulLatRow,
		"",
		connSec, "",
	)
	rows = append(rows, tableRows...)
	rows = append(rows, "", overallLine)

	content := strings.Join(rows, "\n")
	return StyleResultsColRight.Width(w).Render(content)
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
