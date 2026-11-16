package main

import (
	"fmt"
	"os"
	"os/exec"
)

// Layout is  Files | chat | Changes . herdr only focuses neighbours of a
// pane, so each move is "neighbour of X in direction D".

// focusChat (esc): the chat is right of Files and left of Changes.
func focusChat(v viewKind) error {
	dir := "right"
	if v == viewDiff {
		dir = "left"
	}
	return focusFrom(os.Getenv("HERDR_PANE_ID"), dir)
}

// focusNext (tab) cycles Files → chat → Changes → Files. Inside the chat,
// herdr's own cycle key (alt+e / prefix+tab) does the chat → Changes step.
func focusNext(v viewKind) error {
	if v == viewFiles {
		return focusFrom(os.Getenv("HERDR_PANE_ID"), "right")
	}
	// Changes → Files: Files is the chat's left neighbour.
	if chat := os.Getenv("HERDR_EXPLORER_TARGET"); chat != "" {
		return focusFrom(chat, "left")
	}
	return focusFrom(os.Getenv("HERDR_PANE_ID"), "left")
}

func focusFrom(pane, dir string) error {
	if pane == "" {
		return fmt.Errorf("not running in a herdr pane")
	}
	return exec.Command(herdrBin(), "pane", "focus", "--pane", pane, "--direction", dir).Run()
}
