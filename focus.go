package main

import (
	"fmt"
	"os"
	"os/exec"
)

// Layout is  Files | chat | Changes ; alt+e (herdr cycle_pane_next) cycles
// all three. herdr only focuses neighbours, so esc moves one step to the chat.

// focusChat (esc): the chat is right of Files and left of Changes.
func focusChat(v viewKind) error {
	dir := "right"
	if v == viewDiff {
		dir = "left"
	}
	return focusFrom(os.Getenv("HERDR_PANE_ID"), dir)
}

func focusFrom(pane, dir string) error {
	if pane == "" {
		return fmt.Errorf("not running in a herdr pane")
	}
	return exec.Command(herdrBin(), "pane", "focus", "--pane", pane, "--direction", dir).Run()
}
