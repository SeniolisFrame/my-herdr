package main

import (
	"fmt"
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

	press(m, "c")
	if m.mode != modeNormal || !strings.Contains(m.flash, "nothing staged") {
		t.Fatalf("c with nothing staged should explain, mode %v flash %q", m.mode, m.flash)
	}
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

// TestReader opens a file from the Files pane, scrolls, wraps, searches,
// and finds it again through search-all.
func TestReader(t *testing.T) {
	r := newRepo(t)
	var b strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&b, "row %d\n", i)
	}
	b.WriteString(strings.Repeat("x", 130) + "\n")
	write(t, r, "a.txt", b.String())
	write(t, r, "m.go", "package main\n\nfunc main() {}\n")
	m := newModel(r, false, "none")
	m.width, m.height = 60, 20
	m.refreshTree()
	open := func(name string) {
		for m.tcur = 0; m.tcur < len(m.trows) && m.trows[m.tcur].rel != name; m.tcur++ {
		}
		press(m, "enter")
	}

	open("a.txt")
	if m.reading == nil || !strings.Contains(m.View(), "   1 │ row 1") {
		t.Fatalf("enter should open a.txt:\n%s", m.View())
	}
	press(m, "G")
	if v := m.View(); !strings.Contains(v, " 101 │ xxx") || !strings.Contains(v, "     │ xxx") {
		t.Fatalf("G should show the last line, wrapped:\n%s", v)
	}
	press(m, "g", "/", "row 50", "enter")
	if v := m.View(); !strings.Contains(v, "  50 │ row 50") || !strings.Contains(v, "/row 50 1/1") {
		t.Fatalf("search should jump to row 50:\n%s", v)
	}
	press(m, "esc")
	if m.reading != nil {
		t.Fatal("esc should close the reader")
	}

	open("m.go")
	if segs := m.reading.segs[2]; len(segs) < 3 {
		t.Fatalf("m.go should be split into highlighted tokens, got %d", len(segs))
	}
	press(m, "esc", "/", "ROW 7", "enter")
	if m.mode != modePick || !strings.Contains(m.View(), "a.txt:7  row 7") {
		t.Fatalf("search all should list a.txt:7:\n%s", m.View())
	}
	press(m, "down", "enter") // second hit: row 70
	if v := m.View(); m.reading == nil || !strings.Contains(v, "  70 │ row 70") || !strings.Contains(v, "2/11") {
		t.Fatalf("enter should open a.txt at row 70:\n%s", v)
	}
}

// TestFollow switches the panes to the chat pane's new folder.
func TestFollow(t *testing.T) {
	a, b := newRepo(t), newRepo(t)
	write(t, b, "only-in-b.txt", "x\n")
	m := newModel(a, false, "none")
	m.width, m.height = 60, 20
	m.Update(dataMsg{cwd: b, newRoot: b})
	if m.root != b || m.chatCwd != b || !strings.Contains(m.View(), "only-in-b.txt") {
		t.Fatalf("should follow to %s, root %s:\n%s", b, m.root, m.View())
	}
	if m.width != 60 {
		t.Fatal("size must be kept")
	}
}

// TestBlameHistory toggles blame and opens a commit from the file's history.
func TestBlameHistory(t *testing.T) {
	r := newRepo(t)
	write(t, r, "a.txt", "first\n")
	must(t, setStaged(r, true, "a.txt"))
	_, err := gitCommit(r, "one")
	must(t, err)
	write(t, r, "a.txt", "first\nsecond\n")
	must(t, setStaged(r, true, "a.txt"))
	h2, err := gitCommit(r, "two")
	must(t, err)
	write(t, r, "a.txt", "first\nsecond\nthird\n")

	m := newModel(r, false, "none")
	m.width, m.height = 90, 20
	m.refreshTree()
	for m.tcur = 0; m.trows[m.tcur].rel != "a.txt"; m.tcur++ {
	}
	press(m, "enter", "b")
	v := m.View()
	if !strings.Contains(v, h2+" t") || !strings.Contains(v, "second") || !strings.Contains(v, "not committed") {
		t.Fatalf("blame should show %s and the uncommitted line:\n%s", h2, v)
	}
	press(m, "L")
	if m.mode != modePick || !strings.Contains(m.View(), "two") || !strings.Contains(m.View(), "one") {
		t.Fatalf("L should list both commits:\n%s", m.View())
	}
	press(m, "enter") // newest: two
	if v := m.View(); !strings.Contains(v, "a.txt @ "+h2) || !strings.Contains(v, "+second") {
		t.Fatalf("enter should show commit two's diff:\n%s", v)
	}
	press(m, "esc")
	if m.reading == nil || m.reading.back != nil || m.reading.blame == nil {
		t.Fatal("esc should return to the file, blame still on")
	}
	press(m, "esc")
	if m.reading != nil {
		t.Fatal("second esc should close the reader")
	}
}
