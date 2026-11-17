// Glue between the model and herdr: follow the chat pane, focus, zoom.
package ui

import (
	"herdr-explorer/internal/git"
	"herdr-explorer/internal/herdr"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func rootOf(dir string) string {
	if top := git.TopLevel(dir); top != "" {
		return top
	}
	return dir
}

// switchRoot moves the model to root, keeping the pane's size and view.
func (m *model) switchRoot(root string) {
	nm := newModel(root, m.view == viewDiff, m.iconSet)
	nm.width, nm.height = m.width, m.height
	*m = *nm
}

// Layout is  Files | chat | Changes ; alt+e (herdr cycle_pane_next) cycles
// all three. herdr only focuses neighbours, so esc moves one step to the chat.

// zoomCmd zooms this pane in (reader, search results) or back out.
func zoomCmd(on bool) tea.Cmd {
	return func() tea.Msg {
		herdr.Zoom(on)
		return nil
	}
}

// focusChat (esc): the chat is right of Files and left of Changes.
func focusChat(v viewKind) error {
	dir := "right"
	if v == viewDiff {
		dir = "left"
	}
	return herdr.FocusFrom(os.Getenv("HERDR_PANE_ID"), dir)
}
