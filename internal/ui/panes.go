// Glue between the model and herdr: follow the chat pane, focus, zoom.
package ui

import (
	"herdr-explorer/internal/git"
	"herdr-explorer/internal/herdr"
	"os"
	"time"

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

// zoomCmd zooms this pane in (reader, search results) or back out. Zooming
// in starts watching focus: herdr doesn't tell a pane when alt+e moves away,
// and a zoomed pane would keep covering the pane alt+e went to.
func (m *model) zoomCmd(on bool) tea.Cmd {
	zoom := func() tea.Msg {
		herdr.Zoom(on)
		return nil
	}
	if !on || m.watching {
		return zoom
	}
	m.watching = true
	return tea.Batch(zoom, watchFocus())
}

type focusMsg struct {
	self    bool
	focused string
	ok      bool
}

// shortcut: polls herdr every 500ms while zoomed; use focus events if herdr adds them.
func watchFocus() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		self, focused, ok := herdr.Focus()
		return focusMsg{self, focused, ok}
	})
}

// onFocus unzooms when focus left the zoomed pane and zooms back on return.
func (m *model) onFocus(f focusMsg) tea.Cmd {
	if m.reading == nil && !(m.mode == modePick && m.view == viewFiles) {
		m.watching, m.zoomedOut = false, false
		return nil
	}
	if !f.ok { // herdr hiccup: skip this round, keep watching
		return watchFocus()
	}
	var cmd tea.Cmd
	switch {
	case !f.self && !m.zoomedOut:
		m.zoomedOut = true
		cmd = func() tea.Msg { herdr.UnzoomTo(f.focused); return nil }
	case f.self && m.zoomedOut:
		m.zoomedOut = false
		cmd = func() tea.Msg { herdr.Zoom(true); return nil }
	}
	// Sequence: the next check waits for UnzoomTo, which briefly focuses this pane.
	return tea.Sequence(cmd, watchFocus())
}

// focusChat (esc): the chat is right of Files and left of Changes.
func focusChat(v viewKind) error {
	dir := "right"
	if v == viewDiff {
		dir = "left"
	}
	return herdr.FocusFrom(os.Getenv("HERDR_PANE_ID"), dir)
}
