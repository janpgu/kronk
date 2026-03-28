// Package ui provides shared terminal styling for kronk commands.
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Colour palette.
const (
	colorGreen  = lipgloss.Color("#00FF87")
	colorRed    = lipgloss.Color("#FF5F87")
	colorYellow = lipgloss.Color("#FFFF87")
	colorGrey   = lipgloss.Color("#626262")
	colorWhite  = lipgloss.Color("#FFFFFF")
)

// Prefix symbols used in output lines.
const (
	CheckMark = "✓"
	CrossMark = "✗"
	WarnMark  = "⚠"
	Arrow     = "→"
)

// Shared styles.
var (
	SuccessStyle = lipgloss.NewStyle().Foreground(colorGreen).Bold(true)
	ErrorStyle   = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	WarnStyle    = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	MutedStyle   = lipgloss.NewStyle().Foreground(colorGrey)
	BoldStyle    = lipgloss.NewStyle().Bold(true)
	HeaderStyle  = lipgloss.NewStyle().Foreground(colorGrey).Bold(true)

	tableStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorGrey)
)

// PrintSuccess prints a green checkmark line.
func PrintSuccess(msg string) {
	fmt.Println(SuccessStyle.Render(CheckMark+" ") + msg)
}

// PrintError prints a red cross line to stderr.
func PrintError(msg string) {
	fmt.Fprintln(os.Stderr, ErrorStyle.Render(CrossMark+" ")+msg)
}

// PrintWarn prints a yellow warning line.
func PrintWarn(msg string) {
	fmt.Println(WarnStyle.Render(WarnMark+" ") + msg)
}

// RenderTable renders a styled table to a string.
// headers is the list of column names; rows is the data.
// widths sets the width of each column in characters.
func RenderTable(headers []string, rows [][]string, widths []int) string {
	cols := make([]table.Column, len(headers))
	for i, h := range headers {
		w := 20 // default width
		if i < len(widths) {
			w = widths[i]
		}
		cols[i] = table.Column{Title: h, Width: w}
	}

	tableRows := make([]table.Row, len(rows))
	for i, r := range rows {
		tableRows[i] = table.Row(r)
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(tableRows),
		table.WithFocused(false),
		table.WithHeight(len(rows)+1),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorGrey).
		BorderBottom(true).
		Foreground(colorGrey).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(colorWhite).
		Background(lipgloss.Color("")).
		Bold(false)
	t.SetStyles(s)

	return tableStyle.Render(t.View())
}

// Truncate shortens a string to maxLen characters, adding "…" if trimmed.
func Truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// StatusStyle returns a coloured string for a job status value.
func StatusStyle(status string) string {
	switch status {
	case "active":
		return SuccessStyle.Render(status)
	case "running":
		return WarnStyle.Render(status)
	case "failed":
		return ErrorStyle.Render(status)
	case "paused":
		return MutedStyle.Render(status)
	default:
		return status
	}
}

// noopModel is a minimal Bubble Tea model used to initialise the table renderer.
// The table component requires a tea.Program context to render correctly.
type noopModel struct {
	table table.Model
}

func (m noopModel) Init() tea.Cmd                           { return nil }
func (m noopModel) Update(tea.Msg) (tea.Model, tea.Cmd)    { return m, nil }
func (m noopModel) View() string                            { return m.table.View() }
