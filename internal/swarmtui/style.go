package swarmtui

import "github.com/charmbracelet/lipgloss"

var (
	borderASCII = lipgloss.Border{
		Top:         "-",
		Bottom:      "-",
		Left:        "|",
		Right:       "|",
		TopLeft:     "+",
		TopRight:    "+",
		BottomLeft:  "+",
		BottomRight: "+",
	}

	tabBarStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Background(lipgloss.Color("236"))
	tabActiveStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("24")).Bold(true).Padding(0, 1)
	tabInactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Background(lipgloss.Color("236")).Padding(0, 1)

	paneStyle       = lipgloss.NewStyle().Border(borderASCII).BorderForeground(lipgloss.Color("238")).Padding(0, 1)
	paneActiveStyle = paneStyle.Copy().BorderForeground(lipgloss.Color("33"))

	labelStyle         = lipgloss.NewStyle().Bold(true)
	valueMuted         = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	statusErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	statusSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	selectedBorder     = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
)
