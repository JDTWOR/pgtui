package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/rebelice/lazypg/internal/ui/theme"
)

// SuccessNotification represents a brief success notification overlay.
// It appears after DML queries (INSERT/UPDATE/DELETE) to confirm execution
// without creating a full result tab.
type SuccessNotification struct {
	Message string
	Width   int
	Height  int
	Theme   theme.Theme
}

// NewSuccessNotification creates a new success notification overlay.
func NewSuccessNotification(th theme.Theme) *SuccessNotification {
	return &SuccessNotification{
		Theme:  th,
		Width:  50,
		Height: 10,
	}
}

// SetMessage sets the notification message (e.g. "INSERT public.users · 3 rows (0.015s)").
func (sn *SuccessNotification) SetMessage(msg string) {
	sn.Message = msg
}

// View renders the success notification overlay.
func (sn *SuccessNotification) View() string {
	if sn.Width <= 0 || sn.Height <= 0 || sn.Message == "" {
		return ""
	}

	// Icon + title
	iconLine := lipgloss.NewStyle().
		Bold(true).
		Foreground(sn.Theme.Success).
		Render("✓  Success")

	// Message body
	msgStyle := lipgloss.NewStyle().
		Foreground(sn.Theme.Foreground).
		Padding(0, 2)

	wrappedMsg := wrapText(sn.Message, sn.Width-12)
	msgBody := msgStyle.Render(wrappedMsg)

	// Footer hint
	footerStyle := lipgloss.NewStyle().
		Faint(true).
		Foreground(sn.Theme.Foreground).
		Align(lipgloss.Center)

	footer := footerStyle.Render("Press any key to dismiss")

	// Build inner content
	innerLines := []string{
		"",
		iconLine,
		"",
		msgBody,
		"",
		"",
		footer,
	}
	innerContent := strings.Join(innerLines, "\n")

	// Dialog box
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(sn.Theme.Success).
		Padding(1, 3).
		MaxWidth(sn.Width).
		Background(sn.Theme.Background)

	rendered := boxStyle.Render(innerContent)

	// Center the box in the available space
	return lipgloss.Place(sn.Width, sn.Height, lipgloss.Center, lipgloss.Center, rendered)
}
