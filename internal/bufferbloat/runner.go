// Package bufferbloat measures latency degradation under load (bufferbloat).
//
// The test works by:
//  1. Measuring a baseline ping to the target server (median of 10 probes)
//  2. Saturating the download path with concurrent HTTP GETs while
//     continuously pinging every 200ms for ~10 seconds
//  3. Doing the same for upload with concurrent HTTP POSTs
//
// The latency degradation (peak-under-load minus baseline) is used to assign
// a letter grade (A+ through F) following the Waveform bufferbloat scale.
package bufferbloat

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	ev "github.com/marcelofrau/speedy/internal/event"
)

// --- Grades ---

// Grade returns a letter grade for a given latency degradation in ms.
//
//	A+  < 5 ms
//	A   < 30 ms
//	B   < 60 ms
//	C   < 200 ms
//	D   < 400 ms
//	F   >= 400 ms
func Grade(degradationMs float64) string {
	switch {
	case degradationMs < 5:
		return "A+"
	case degradationMs < 30:
		return "A"
	case degradationMs < 60:
		return "B"
	case degradationMs < 200:
		return "C"
	case degradationMs < 400:
		return "D"
	default:
		return "F"
	}
}

// GradeColor returns the Lip Gloss hex color for a grade.
func GradeColor(grade string) string {
	switch grade {
	case "A+", "A":
		return "#9ece6a" // green
	case "B":
		return "#7aa2f7" // blue
	case "C":
		return "#ff9e64" // orange
	case "D", "F":
		return "#f7768e" // red/pink
	default:
		return "#565f89"
	}
}

// --- Messages emitted to Bubble Tea ---

// MsgBloatBaseline is sent after the idle baseline ping is measured.
type MsgBloatBaseline struct {
	Ms float64
}

// MsgBloatDownloadPing is sent for each ping sample during download saturation.
type MsgBloatDownloadPing struct {
	Ms float64
}

// MsgBloatUploadPing is sent for each ping sample during upload saturation.
type MsgBloatUploadPing struct {
	Ms float64
}

// MsgBloatDone is sent when all bufferbloat measurements are complete.
type MsgBloatDone struct {
	Result Result
}

// Result holds the final bufferbloat measurements.
type Result struct {
	BaselineMs     float64
	DLPeakMs       float64
	DLDegradationMs float64
	DLGrade        string
	ULPeakMs       float64
	ULDegradationMs float64
	ULGrade        string
	OverallGrade   string
}

// --- Runner ---

const (
	bloatDuration    = 10 * time.Second
	pingInterval     = 200 * time.Millisecond
	baselineProbes   = 10
	dlWorkers        = 8
	ulWorkers        = 4
	dlChunkSize      = 8 * 1024 * 1024  // 8 MB download chunks
	ulChunkSize      = 4 * 1024 * 1024  // 4 MB upload payload
)

// emitLog is a helper to send a MsgLog via send.
func emitLog(send func(tea.Msg), text string, level ev.LogLevel) {
	send(ev.MsgLog{Text: text, Level: level})
}

// Run returns a tea.Cmd that runs the full bufferbloat test against serverURL.
// serverURL should be the host URL of the selected speedtest server,
// e.g. "http://speedtest.example.com:8080".
func Run(serverURL string, send func(tea.Msg)) tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 30 * time.Second}

		pingURL := serverURL + "/speedtest/latency.txt"
		dlURL := serverURL + "/speedtest/random4000x4000.jpg"
		ulURL := serverURL + "/speedtest/upload.php"

		// 1. Baseline ping
		emitLog(send, "Bufferbloat: measuring idle baseline ping (10 probes)...", ev.LogInfo)
		baselineMs := measureBaselinePing(client, pingURL, baselineProbes)
		send(MsgBloatBaseline{Ms: baselineMs})
		emitLog(send, fmt.Sprintf("Baseline ping: %.0f ms", baselineMs), ev.LogData)

		// 2. Download saturation + ping (log throttled to every 2s)
		emitLog(send, fmt.Sprintf("Saturating download (%d streams) + pinging every 200 ms...", dlWorkers), ev.LogInfo)
		var lastLogDLTime time.Time
		dlPings := saturateAndPing(client, pingURL, dlURL, "", bloatDuration, dlWorkers, func(ms float64) {
			send(MsgBloatDownloadPing{Ms: ms})
			if time.Since(lastLogDLTime) >= 2*time.Second {
				deg := ms - baselineMs
				level := ev.LogData
				if Grade(deg) == "C" || Grade(deg) == "D" || Grade(deg) == "F" {
					level = ev.LogWarn
				}
				emitLog(send, fmt.Sprintf("↓ ping under load: %.0f ms  (+%.0f ms)", ms, deg), level)
				lastLogDLTime = time.Now()
			}
		})

		dlPeak := peakMs(dlPings)
		dlDeg := dlPeak - baselineMs
		if dlDeg < 0 {
			dlDeg = 0
		}
		dlGrade := Grade(dlDeg)
		emitLog(send,
			fmt.Sprintf("Download bufferbloat: %s  peak %.0f ms  (+%.0f ms)", dlGrade, dlPeak, dlDeg),
			ev.LogSuccess)

		// 3. Upload saturation + ping
		emitLog(send, fmt.Sprintf("Saturating upload (%d streams) + pinging every 200 ms...", ulWorkers), ev.LogInfo)
		var lastLogULTime time.Time
		ulPings := saturateAndPing(client, pingURL, "", ulURL, bloatDuration, ulWorkers, func(ms float64) {
			send(MsgBloatUploadPing{Ms: ms})
			if time.Since(lastLogULTime) >= 2*time.Second {
				deg := ms - baselineMs
				level := ev.LogData
				if Grade(deg) == "C" || Grade(deg) == "D" || Grade(deg) == "F" {
					level = ev.LogWarn
				}
				emitLog(send, fmt.Sprintf("↑ ping under load: %.0f ms  (+%.0f ms)", ms, deg), level)
				lastLogULTime = time.Now()
			}
		})

		ulPeak := peakMs(ulPings)
		ulDeg := ulPeak - baselineMs
		if ulDeg < 0 {
			ulDeg = 0
		}
		ulGrade := Grade(ulDeg)
		emitLog(send,
			fmt.Sprintf("Upload bufferbloat: %s  peak %.0f ms  (+%.0f ms)", ulGrade, ulPeak, ulDeg),
			ev.LogSuccess)

		overall := worstGrade(dlGrade, ulGrade)
		emitLog(send, fmt.Sprintf("All tests complete — overall grade: %s", overall), ev.LogSuccess)

		return MsgBloatDone{Result: Result{
			BaselineMs:      baselineMs,
			DLPeakMs:        dlPeak,
			DLDegradationMs: dlDeg,
			DLGrade:         dlGrade,
			ULPeakMs:        ulPeak,
			ULDegradationMs: ulDeg,
			ULGrade:         ulGrade,
			OverallGrade:    overall,
		}}
	}
}

// measureBaselinePing sends n sequential HTTP GET pings and returns the median RTT in ms.
func measureBaselinePing(client *http.Client, pingURL string, n int) float64 {
	samples := make([]float64, 0, n)
	for i := 0; i < n; i++ {
		ms := httpPing(client, pingURL)
		if ms > 0 {
			samples = append(samples, ms)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(samples) == 0 {
		return 0
	}
	return median(samples)
}

// saturateAndPing saturates the link (download or upload) while measuring ping latency.
// Pass a non-empty dlURL to saturate download, ulURL to saturate upload.
// Returns all collected ping samples.
func saturateAndPing(
	client *http.Client,
	pingURL, dlURL, ulURL string,
	duration time.Duration,
	workers int,
	onPing func(ms float64),
) []float64 {
	ctx, cancel := context.WithTimeout(context.Background(), duration+5*time.Second)
	defer cancel()

	deadline := time.Now().Add(duration)
	var wg sync.WaitGroup

	// Launch saturation workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for time.Now().Before(deadline) {
				if dlURL != "" {
					drainDownload(ctx, client, dlURL)
				} else if ulURL != "" {
					driveUpload(ctx, client, ulURL)
				}
			}
		}()
	}

	// Ping loop — runs for the full duration
	var mu sync.Mutex
	var samples []float64

	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		for time.Now().Before(deadline) {
			ms := httpPing(client, pingURL)
			if ms > 0 {
				mu.Lock()
				samples = append(samples, ms)
				mu.Unlock()
				onPing(ms)
			}
			time.Sleep(pingInterval)
		}
	}()

	<-pingDone
	cancel() // signal workers to stop
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	return samples
}

// drainDownload performs a single large GET and discards the body.
func drainDownload(ctx context.Context, client *http.Client, url string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	buf := make([]byte, 32*1024)
	for {
		_, err := resp.Body.Read(buf)
		if err != nil {
			break
		}
		if ctx.Err() != nil {
			break
		}
	}
}

// driveUpload performs a single large POST with random payload.
func driveUpload(ctx context.Context, client *http.Client, url string) {
	payload := make([]byte, ulChunkSize)
	rand.Read(payload) //nolint:gosec // random upload payload, not security-sensitive
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url,
		newInfiniteReader(ctx, payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(ulChunkSize)
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
}

// httpPing measures the round-trip time of a single HTTP GET in milliseconds.
// Returns -1 on error.
func httpPing(client *http.Client, url string) float64 {
	pingClient := &http.Client{Timeout: 2 * time.Second}
	start := time.Now()
	resp, err := pingClient.Get(fmt.Sprintf("%s?t=%d", url, time.Now().UnixNano()))
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	return float64(time.Since(start).Milliseconds())
}

// median returns the median of a float64 slice (sorts a copy).
func median(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	cp := make([]float64, len(vals))
	copy(cp, vals)
	sort.Float64s(cp)
	mid := len(cp) / 2
	if len(cp)%2 == 0 {
		return (cp[mid-1] + cp[mid]) / 2
	}
	return cp[mid]
}

// peakMs returns the 95th-percentile (near-peak) latency from a sample slice.
// Using p95 instead of max avoids single outliers inflating the result.
func peakMs(samples []float64) float64 {
	if len(samples) == 0 {
		return 0
	}
	cp := make([]float64, len(samples))
	copy(cp, samples)
	sort.Float64s(cp)
	idx := int(float64(len(cp)) * 0.95)
	if idx >= len(cp) {
		idx = len(cp) - 1
	}
	return cp[idx]
}

// gradeRank maps grades to a numeric rank for comparison (lower = better).
var gradeRank = map[string]int{
	"A+": 0, "A": 1, "B": 2, "C": 3, "D": 4, "F": 5,
}

// worstGrade returns the worse of two letter grades.
func worstGrade(a, b string) string {
	if gradeRank[a] >= gradeRank[b] {
		return a
	}
	return b
}

// infiniteReader wraps a payload slice and repeats it until the context is done.
type infiniteReader struct {
	ctx     context.Context
	payload []byte
	pos     int
	limit   int
	sent    int
}

func newInfiniteReader(ctx context.Context, payload []byte) *infiniteReader {
	return &infiniteReader{ctx: ctx, payload: payload, limit: len(payload)}
}

func (r *infiniteReader) Read(p []byte) (int, error) {
	if r.ctx.Err() != nil || r.sent >= r.limit {
		return 0, io.EOF
	}
	n := copy(p, r.payload[r.pos:])
	r.pos = (r.pos + n) % len(r.payload)
	r.sent += n
	return n, nil
}
