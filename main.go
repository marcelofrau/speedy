package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/marcelofrau/speedy/internal/tui"
)

func main() {
	// Create the program first with a placeholder model, then wire Send.
	// We need p.Send before Init() runs so the runner goroutine can deliver
	// progress messages back to the TUI.
	m := tui.NewModel()

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
	)

	// Patch the model's send function now that we have a program reference.
	// NewModel returns a value type; Init() is called by p.Run() which has
	// its own internal copy — so we use a wrapper that forwards via p.Send.
	tui.SetSend(func(msg tea.Msg) { p.Send(msg) })

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
