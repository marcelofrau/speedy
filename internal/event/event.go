// Package event defines shared message types used across internal packages.
// It has zero external dependencies so any package can import it without
// creating circular imports.
package event

import "time"

// LogLevel controls how a log entry is rendered in the activity log panel.
type LogLevel int

const (
	LogInfo    LogLevel = iota // dim — generic status message
	LogSuccess                 // neon green — something completed successfully
	LogData                    // neon cyan — a measured value
	LogWarn                    // neon orange — degradation or warning
)

// MsgLog is a Bubble Tea message carrying a single activity log entry.
type MsgLog struct {
	Text    string
	Level   LogLevel
	Elapsed time.Duration // time since test start; set by the TUI on receive
}
