// Package tui provides a terminal user interface for the registry client.
package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette for consistent styling.
var (
	primaryColor   = lipgloss.Color("39")  // Blue
	secondaryColor = lipgloss.Color("205") // Pink
	successColor   = lipgloss.Color("42")  // Green
	warningColor   = lipgloss.Color("214") // Orange
	errorColor     = lipgloss.Color("196") // Red
	subtleColor    = lipgloss.Color("241") // Gray
	highlightColor = lipgloss.Color("212") // Light pink
)

// Common styles used throughout the TUI.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	helpStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			MarginTop(1)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	spinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	dialogBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			MarginTop(1)

	buttonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			Background(lipgloss.Color("238")).
			Padding(0, 2).
			MarginRight(1)

	activeButtonStyle = buttonStyle.
				Foreground(lipgloss.Color("255")).
				Background(primaryColor).
				Bold(true)

	dimmedStyle = lipgloss.NewStyle().
			Foreground(subtleColor)

	digestStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Italic(true)

	// Log view styles
	logStdoutStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	logStderrStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	logPrefixStdoutStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	logPrefixStderrStyle = lipgloss.NewStyle().
				Foreground(errorColor).
				Bold(true)

	logViewportStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(subtleColor).
				Padding(0, 1)

	// Server status panel styles
	statusPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(subtleColor).
				Padding(0, 1).
				MarginBottom(1)

	statusRunningStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	statusStoppedStyle = lipgloss.NewStyle().
				Foreground(errorColor).
				Bold(true)

	// Image details styles
	layerStyle = lipgloss.NewStyle().
			Foreground(subtleColor)

	sizeStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	// Help overlay styles
	helpOverlayStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(primaryColor).
				Padding(1, 2).
				Background(lipgloss.Color("236"))

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true).
			Width(12)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	helpSectionStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true).
				MarginTop(1).
				MarginBottom(0)

	// Progress/action styles
	progressStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	actionSuccessStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Bold(true)

	actionErrorStyle = lipgloss.NewStyle().
				Foreground(errorColor).
				Bold(true)

	// Notification styles
	notificationStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(warningColor).
				Padding(0, 1).
				MarginTop(1)

	notificationErrorStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(errorColor).
				Background(lipgloss.Color("52")).
				Padding(0, 1).
				MarginTop(1)

	notificationSuccessStyle = lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(successColor).
					Padding(0, 1).
					MarginTop(1)
)
