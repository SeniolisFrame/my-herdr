package main

import (
	"os"
	"path/filepath"
)

// The side panes follow the chat pane (checked on the 2s poll): when it is in
// another folder, they switch to that folder's repo (or the folder itself).

// chatCwd is the chat pane's current folder; "" outside herdr or when unknown.
func chatCwd() string {
	target := os.Getenv("HERDR_EXPLORER_TARGET")
	if target == "" {
		return ""
	}
	panes, err := listPanes()
	if err != nil {
		return ""
	}
	for _, p := range panes {
		if p.PaneID == target && isDir(p.Cwd) {
			return filepath.Clean(p.Cwd)
		}
	}
	return ""
}

func rootOf(dir string) string {
	if top := gitTopLevel(dir); top != "" {
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
