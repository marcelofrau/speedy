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

// Update handles incoming messages and returns the updated model plus next Cmd.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		w := progressWidth(panelWidth)
		m.dlProgress.Width = w
		m.ulProgress.Width = w
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q", "ctrl+c":
			return m, tea.Quit
		default:
			// Any key other than quit advances from the wait screen to results
			if m.phase == PhaseWaitKey {
				m.phase = PhaseDone
				return m, nil
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		dlModel, dlCmd := m.dlProgress.Update(msg)
		ulModel, ulCmd := m.ulProgress.Update(msg)
		m.dlProgress = dlModel.(progress.Model)
		m.ulProgress = ulModel.(progress.Model)
		return m, tea.Batch(dlCmd, ulCmd)

	// --- Activity log ---

	case ev.MsgLog:
		m.log = appendLog(m.log, msg, m.startTime, logVisibleRows)
		return m, nil

	// --- Speedtest messages ---

	case st.MsgUserInfo:
		m.ip = msg.IP
		m.isp = msg.ISP
		m.phase = PhaseFindingServer
		m.phaseStartTime = time.Now()
		return m, nil

	case st.MsgServerFound:
		m.server = msg.Name
		m.country = msg.Country
		m.sponsor = msg.Sponsor
		m.phase = PhasePing
		m.phaseStartTime = time.Now()
		return m, nil

	case st.MsgPingDone:
		m.pingMs = msg.PingMs
		m.jitterMs = msg.JitterMs
		m.phase = PhaseDownload
		m.phaseStartTime = time.Now()
		return m, nil

	case st.MsgDownloadProgress:
		m.dlMbps = msg.Mbps
		m.dlPercent = msg.Percent
		m.dlHistory = appendHistory(m.dlHistory, msg.Mbps, sparklineLen)
		if m.phase != PhaseDownload {
			m.phase = PhaseDownload
			m.phaseStartTime = time.Now()
		}
		cmd := m.dlProgress.SetPercent(msg.Percent)
		return m, cmd

	case st.MsgUploadProgress:
		m.ulMbps = msg.Mbps
		m.ulPercent = msg.Percent
		m.ulHistory = appendHistory(m.ulHistory, msg.Mbps, sparklineLen)
		if m.phase != PhaseUpload {
			m.phase = PhaseUpload
			m.phaseStartTime = time.Now()
		}
		cmd := m.ulProgress.SetPercent(msg.Percent)
		return m, cmd

	case st.MsgSpeedDone:
		m.speedResult = msg.Result
		m.phase = PhaseBloatBaseline
		m.phaseStartTime = time.Now()
		return m, nil

	// --- Bufferbloat messages ---

	case bb.MsgBloatBaseline:
		m.bloatBaselineMs = msg.Ms
		m.phase = PhaseBloatDownload
		m.phaseStartTime = time.Now()
		return m, nil

	case bb.MsgBloatDownloadPing:
		m.bloatDLHistory = appendHistory(m.bloatDLHistory, msg.Ms, bloatLen)
		if m.phase != PhaseBloatDownload {
			m.phase = PhaseBloatDownload
			m.phaseStartTime = time.Now()
		}
		return m, nil

	case bb.MsgBloatUploadPing:
		m.bloatULHistory = appendHistory(m.bloatULHistory, msg.Ms, bloatLen)
		if m.phase != PhaseBloatUpload {
			m.phase = PhaseBloatUpload
			m.phaseStartTime = time.Now()
		}
		return m, nil

	case bb.MsgBloatDone:
		m.bloatResult = msg.Result
		m.phase = PhaseWaitKey // pause for keypress before showing results
		m.phaseStartTime = time.Now()
		return m, nil

	case st.MsgDone:
		m.phase = PhaseDone
		return m, nil

	case st.MsgError:
		if msg.Err != nil {
			m.err = msg.Err
			m.phase = PhaseError
		}
		return m, nil
	}

	return m, nil
}
