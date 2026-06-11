package tui

import "github.com/charmbracelet/lipgloss"

// Synthwave palette — vivid neons on a deep dark background
const (
	// Backgrounds / borders
	colorBg         = "#0d0d1a" // near-black with purple tint
	colorBorder      = "#3d1f5e" // dark purple border
	colorBorderActive = "#c724b1" // hot magenta for active panels

	// Neon primaries
	colorNeonPink    = "#ff2d9b" // hot pink / magenta
	colorNeonPurple  = "#c724b1" // electric purple
	colorNeonBlue    = "#00b4fc" // electric blue
	colorNeonCyan    = "#00f5ff" // bright cyan
	colorNeonGreen   = "#39ff14" // neon green
	colorNeonYellow  = "#ffe600" // electric yellow
	colorNeonOrange  = "#ff6c11" // neon orange

	// Text
	colorText    = "#f0e6ff" // very light lavender
	colorTextDim = "#b89ec4" // muted lavender
	colorMuted   = "#6b4d7e" // dark purple-grey for placeholders/labels
	colorDim     = "#3d2b52" // very dark, for separators
)

var (
	// Base
	StyleBase = lipgloss.NewStyle().Foreground(lipgloss.Color(colorText))
	StyleDim  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	StyleMuted = lipgloss.NewStyle().Foreground(lipgloss.Color(colorTextDim))

	// Header
	StyleTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonCyan))

	StyleVersion = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted))

	StyleHeader = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorNeonPurple)).
			Padding(0, 2)

	// Info rows
	StyleLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Width(8)

	StyleValue = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorText))

	StyleBullet = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorBorder)).
			SetString("•")

	// Panels
	StylePanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder)).
			Padding(1, 2)

	StylePanelActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(colorNeonPink)).
				Padding(1, 2)

	// Section headings inside panels
	StyleSectionDown = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorNeonBlue))

	StyleSectionUp = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonGreen))

	StyleSectionPing = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorNeonCyan))

	// Speed values — big numbers
	StyleSpeedDown = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonBlue))

	StyleSpeedUp = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonGreen))

	StyleSpeedPing = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonCyan))

	StyleUnit = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextDim))

	// Arc / sparkline characters
	ArcFilled  = "█"
	ArcEmpty   = "░"
	SparkChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

	// Spinner
	StyleSpinner = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorNeonPink))

	StyleStatus = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextDim)).
			Italic(true)

	// Stepper
	StyleStepDone = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonGreen))

	StyleStepActive = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonPink))

	StyleStepPending = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorMuted))

	StyleStepLabel = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted))

	StyleStepLabelActive = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorText))

	StyleStepLabelDone = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTextDim))

	StyleStepSep = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorDim))

	// Status bar
	StyleStatusBar = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextDim)).
			Italic(true)

	StyleStatusHighlight = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorNeonCyan))

	// Placeholder (empty state)
	StylePlaceholder = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorDim))

	// Results
	StyleResultsBox = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(colorNeonPurple)).
				Padding(1, 3)

	StyleResultsTitle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorNeonPink)).
				Align(lipgloss.Center)

	StyleResultKey = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Width(12)

	StyleResultVal = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorText))

	StyleHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorMuted)).
			Italic(true)

	// Activity log panel
	StyleLogPanel = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorBorder)).
			Padding(0, 1)

	StyleLogTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorTextDim))

	StyleLogTimestamp = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorMuted))

	StyleLogInfo = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorTextDim))

	StyleLogSuccess = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorNeonGreen))

	StyleLogData = lipgloss.NewStyle().
			Foreground(lipgloss.Color(colorNeonCyan))

	StyleLogWarn = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonOrange))

	// Results screen — 2-column summary
	StyleResultsGradeLetter = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(colorNeonPink))

	StyleResultsGradeDesc = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorTextDim)).
				Italic(true)

	StyleTableHeader = lipgloss.NewStyle().
				Foreground(lipgloss.Color(colorMuted))

	StyleTableOK = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonGreen))

	StyleTableWarn = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonOrange))

	StyleTableFail = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonPink))

	StyleResultsColLeft = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(colorBorder)).
				Padding(1, 2)

	StyleResultsColRight = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(colorNeonPurple)).
				Padding(1, 2)

	// Wait-for-keypress screen
	StyleWaitKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonPink))

	// Error
	StyleError = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(colorNeonPink))
)
