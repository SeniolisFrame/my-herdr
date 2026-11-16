package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// drain runs a command and feeds its messages back into the model, like the
// bubbletea runtime does (ticks are skipped so the test cannot loop forever).
func drain(m *model, cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, c := range msg {
			drain(m, c)
		}
	case tickMsg, nil:
	default:
		_, next := m.Update(msg)
		drain(m, next)
	}
}

func press(m *model, keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case " ":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		_, cmd := m.Update(msg)
		drain(m, cmd)
	}
}

// TestUIFlow drives the Changes pane with key presses: stage, commit with a
// Thai message, history, new branch, merge conflict resolved with T, discard.
func TestUIFlow(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a.txt", "one\n")
	must(t, setStaged(r, true, "a.txt"))
	_, err := gitCommit(r, "init")
	must(t, err)

	m := newModel(r, true, "none")
	m.width, m.height = 100, 30
	write(t, r, "a.txt", "two\n")
	drain(m, m.loadCmd(true))

	press(m, " ")
	if m.staged["a.txt"] != "full" || !strings.Contains(m.View(), "✓") {
		t.Fatalf("space should stage a.txt: %v\n%s", m.staged, m.View())
	}

	press(m, "c")
	if m.mode != modeInput {
		t.Fatal("c should open the commit message box")
	}
	press(m, "แก้", " ", "bug", "enter")
	if out, _ := git(r, "log", "-1", "--format=%s"); strings.TrimSpace(out) != "แก้ bug" {
		t.Fatalf("commit message: %q (flash %q)", out, m.flash)
	}

	press(m, "L")
	if m.mode != modePick || !strings.Contains(m.View(), "แก้ bug") {
		t.Fatalf("L should open history:\n%s", m.View())
	}
	press(m, "enter")
	if m.mode != modeShow || len(m.diffs) != 1 {
		t.Fatalf("enter should show the commit: mode %v diffs %d", m.mode, len(m.diffs))
	}
	press(m, "esc", "esc")
	if m.mode != modeNormal {
		t.Fatalf("esc esc should return to the changes list, mode %v", m.mode)
	}

	// New branch with a conflicting change, merge it from main via the picker.
	press(m, "B", "feature", "enter")
	write(t, r, "a.txt", "feature\n")
	must(t, setStaged(r, true, "a.txt"))
	_, err = gitCommit(r, "feature")
	must(t, err)
	press(m, "b")
	if m.mode != modePick || !strings.Contains(m.View(), "main") {
		t.Fatalf("b should list main:\n%s", m.View())
	}
	press(m, "enter") // the only other branch: main
	write(t, r, "a.txt", "main\n")
	must(t, setStaged(r, true, "a.txt"))
	_, err = gitCommit(r, "main side")
	must(t, err)
	drain(m, m.loadCmd(true))

	press(m, "m", "enter") // merge feature
	if !m.merging || m.status["a.txt"] != "C" || m.flash != errConflicts.Error() {
		t.Fatalf("want a conflict: merging=%v status=%v flash=%q", m.merging, m.status, m.flash)
	}
	if !strings.Contains(m.View(), "MERGING") {
		t.Fatalf("branch bar should say MERGING:\n%s", m.View())
	}
	press(m, "T", "c", "enter") // take theirs, commit with git's merge message
	if m.merging {
		t.Fatalf("merge should be committed, flash %q", m.flash)
	}

	// Discard asks first; Cancel is the default.
	write(t, r, "a.txt", "oops\n")
	drain(m, m.loadCmd(true))
	press(m, "D", "enter")
	if m.status["a.txt"] != "M" {
		t.Fatal("enter on the default Cancel must keep the change")
	}
	press(m, "D", "down", "enter")
	if len(m.status) != 0 {
		t.Fatalf("discard should leave a clean tree, got %v (flash %q)", m.status, m.flash)
	}
}
