// Package ui provides shared terminal styling for kronk commands.
package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette.
const (
	colorGreen  = lipgloss.Color("#00FF87")
	colorRed    = lipgloss.Color("#FF5F87")
	colorYellow = lipgloss.Color("#FFFF87")
	colorGrey   = lipgloss.Color("#626262")
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
// widths sets the visible character width of each column.
// Cell values must be plain strings — no ANSI codes inside cells.
func RenderTable(headers []string, rows [][]string, widths []int) string {
	colWidth := func(i int) int {
		if i < len(widths) {
			return widths[i]
		}
		return 20
	}

	// totalWidth is the sum of all column widths plus 2-space gaps.
	totalWidth := 0
	for i := range headers {
		totalWidth += colWidth(i) + 2
	}

	// buildLine uses fmt.Sprintf for reliable fixed-width layout.
	// %-*s: left-align in a field of exactly w characters.
	// Each line is padded to exactly totalWidth characters.
	buildLine := func(cells []string) string {
		var sb strings.Builder
		for i, cell := range cells {
			fmt.Fprintf(&sb, "%-*s  ", colWidth(i), cell)
		}
		line := sb.String()
		// Pad or trim to exactly totalWidth so borders align.
		if len(line) < totalWidth {
			line += strings.Repeat(" ", totalWidth-len(line))
		}
		return line[:totalWidth]
	}

	headerLine := buildLine(headers)
	border := strings.Repeat("─", totalWidth)

	var sb strings.Builder
	sb.WriteString(MutedStyle.Render("┌"+border+"┐") + "\n")
	sb.WriteString(" " + HeaderStyle.Render(headerLine) + "\n")
	sb.WriteString(" " + MutedStyle.Render(border) + "\n")
	for _, row := range rows {
		sb.WriteString(" " + buildLine(row) + "\n")
	}
	sb.WriteString(MutedStyle.Render("└" + border + "┘"))
	return sb.String()
}

// Truncate shortens a string to maxLen characters, adding "…" if trimmed.
func Truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

// StatusStyle returns a colored string for a job status value.
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
