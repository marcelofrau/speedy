package speedtest

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	bb "github.com/marcelofrau/speedy/internal/bufferbloat"
	ev "github.com/marcelofrau/speedy/internal/event"
	stlib "github.com/showwin/speedtest-go/speedtest"
)

// Result holds the final measurement values.
type Result struct {
	ServerName   string
	Country      string
	Sponsor      string
	ServerURL    string
	IP           string
	ISP          string
	PingMs       float64
	JitterMs     float64
	DownloadMbps float64
	UploadMbps   float64
}

// --- Messages emitted to Bubble Tea ---

type MsgUserInfo struct {
	IP  string
	ISP string
}

type MsgServerFound struct {
	Name    string
	Country string
	Sponsor string
}

type MsgPingDone struct {
	PingMs   float64
	JitterMs float64
}

type MsgDownloadProgress struct {
	Mbps    float64
	Percent float64
}

type MsgUploadProgress struct {
	Mbps    float64
	Percent float64
}

type MsgSpeedDone struct {
	Result Result
}

type MsgDone struct {
	Result      Result
	BloatResult bb.Result
}

type MsgError struct {
	Err error
}

const maxSpeedMbps = 1000.0

func serverBaseURL(rawURL string) string {
	parts := strings.SplitN(rawURL, "/", 4)
	if len(parts) >= 3 {
		return parts[0] + "//" + parts[2]
	}
	return rawURL
}

// log emits a MsgLog via send.
func log(send func(tea.Msg), text string, level ev.LogLevel) {
	send(ev.MsgLog{Text: text, Level: level})
}

// Run returns a tea.Cmd that executes the full test sequence.
func Run(send func(tea.Msg)) tea.Cmd {
	return func() tea.Msg {
		client := stlib.New()

		// 1. User info
		log(send, "Connecting to Speedtest.net...", ev.LogInfo)
		user, err := client.FetchUserInfo()
		if err != nil {
			return MsgError{Err: err}
		}
		send(MsgUserInfo{IP: user.IP, ISP: user.Isp})
		log(send, fmt.Sprintf("IP: %s  •  ISP: %s", user.IP, user.Isp), ev.LogData)

		// 2. Server list
		log(send, "Fetching server list...", ev.LogInfo)
		servers, err := client.FetchServers()
		if err != nil {
			return MsgError{Err: err}
		}
		log(send, fmt.Sprintf("Found %d servers, pinging all in parallel...", len(servers)), ev.LogInfo)

		targets, err := servers.FindServer([]int{})
		if err != nil {
			return MsgError{Err: err}
		}
		server := targets[0]
		send(MsgServerFound{
			Name:    server.Name,
			Country: server.Country,
			Sponsor: server.Sponsor,
		})
		log(send,
			fmt.Sprintf("Best server: %s, %s (%s)", server.Name, server.Country, server.Sponsor),
			ev.LogSuccess)

		// 3. Ping
		log(send, "Running ping test...", ev.LogInfo)
		if err := server.PingTest(nil); err != nil {
			return MsgError{Err: err}
		}
		pingMs := float64(server.Latency.Milliseconds())
		jitterMs := float64(server.Jitter.Milliseconds())
		send(MsgPingDone{PingMs: pingMs, JitterMs: jitterMs})
		log(send, fmt.Sprintf("Ping: %.0f ms  •  Jitter: %.0f ms", pingMs, jitterMs), ev.LogData)

		// 4. Download — throttle log to ~1 per second, or >=10% change
		log(send, "Starting download test (8 parallel streams)...", ev.LogInfo)
		var lastLogDLTime time.Time
		var lastLogDLMbps float64
		client.SetCallbackDownload(func(rate stlib.ByteRate) {
			mbps := rate.Mbps()
			pct := mbps / maxSpeedMbps
			if pct > 1.0 {
				pct = 1.0
			}
			send(MsgDownloadProgress{Mbps: mbps, Percent: pct})
			// throttle log: only if >=1s elapsed or >=10% change
			delta := math.Abs(mbps-lastLogDLMbps) / math.Max(lastLogDLMbps, 1)
			if time.Since(lastLogDLTime) >= time.Second || delta >= 0.10 {
				log(send, fmt.Sprintf("↓  %.1f Mbps", mbps), ev.LogData)
				lastLogDLTime = time.Now()
				lastLogDLMbps = mbps
			}
		})
		if err := server.DownloadTest(); err != nil {
			return MsgError{Err: err}
		}
		dlMbps := server.DLSpeed.Mbps()
		send(MsgDownloadProgress{Mbps: dlMbps, Percent: dlMbps / maxSpeedMbps})
		log(send, fmt.Sprintf("Download complete: %.1f Mbps", dlMbps), ev.LogSuccess)

		// 5. Upload — same throttle
		log(send, "Starting upload test (4 parallel streams)...", ev.LogInfo)
		var lastLogULTime time.Time
		var lastLogULMbps float64
		client.SetCallbackUpload(func(rate stlib.ByteRate) {
			mbps := rate.Mbps()
			pct := mbps / maxSpeedMbps
			if pct > 1.0 {
				pct = 1.0
			}
			send(MsgUploadProgress{Mbps: mbps, Percent: pct})
			delta := math.Abs(mbps-lastLogULMbps) / math.Max(lastLogULMbps, 1)
			if time.Since(lastLogULTime) >= time.Second || delta >= 0.10 {
				log(send, fmt.Sprintf("↑  %.1f Mbps", mbps), ev.LogData)
				lastLogULTime = time.Now()
				lastLogULMbps = mbps
			}
		})
		if err := server.UploadTest(); err != nil {
			return MsgError{Err: err}
		}
		ulMbps := server.ULSpeed.Mbps()
		send(MsgUploadProgress{Mbps: ulMbps, Percent: ulMbps / maxSpeedMbps})
		log(send, fmt.Sprintf("Upload complete: %.1f Mbps", ulMbps), ev.LogSuccess)

		baseURL := serverBaseURL(server.URL)
		speedResult := Result{
			ServerName:   server.Name,
			Country:      server.Country,
			Sponsor:      server.Sponsor,
			ServerURL:    baseURL,
			IP:           user.IP,
			ISP:          user.Isp,
			PingMs:       pingMs,
			JitterMs:     jitterMs,
			DownloadMbps: dlMbps,
			UploadMbps:   ulMbps,
		}
		send(MsgSpeedDone{Result: speedResult})

		// 6. Bufferbloat
		bloatCmd := bb.Run(baseURL, send)
		return bloatCmd()
	}
}
