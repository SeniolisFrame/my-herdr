package main

import (
	"fmt"
	"os"
	"os/exec"
)

// toggleFocus is the "toggle" action (bound to one key in config.toml):
// in the file tree → jump to the chat pane on its right;
// anywhere else → jump to this tab's file tree (opening it if missing).
func toggleFocus() int {
	ctx := readLaunchContext()
	panes, err := listPanes()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	var cur *paneInfo
	for i := range panes {
		if panes[i].PaneID == ctx.FocusedPaneID {
			cur = &panes[i]
		}
	}
	if cur == nil {
		fmt.Fprintln(os.Stderr, "toggle: no focused pane in context")
		return 1
	}
	if cur.Label == "Files" {
		return run(herdrBin(), "pane", "focus", "--pane", cur.PaneID, "--direction", "right")
	}
	files := findFilesPane(cur.TabID)
	if files == "" {
		ensureFilesPane(cur.WorkspaceID, cur.TabID)
		files = findFilesPane(cur.TabID)
	}
	if files == "" {
		fmt.Fprintln(os.Stderr, "toggle: could not open the file tree")
		return 1
	}
	return run(herdrBin(), "plugin", "pane", "focus", files)
}

func findFilesPane(tab string) string {
	panes, err := listPanes()
	if err != nil {
		return ""
	}
	for _, p := range panes {
		if p.TabID == tab && p.Label == "Files" {
			return p.PaneID
		}
	}
	return ""
}

// focusChat moves focus from the explorer pane itself to its right neighbour.
func focusChat() error {
	self := os.Getenv("HERDR_PANE_ID")
	if self == "" {
		return fmt.Errorf("not running in a herdr pane")
	}
	return exec.Command(herdrBin(), "pane", "focus", "--pane", self, "--direction", "right").Run()
}

func run(name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}
