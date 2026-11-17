package ui

import (
	"os"
	"path/filepath"
	"testing"

	"herdr-explorer/internal/git"

	tea "github.com/charmbracelet/bubbletea"
)

// Test repos (same helpers as the git package tests).

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "core.autocrlf", "false"},
	} {
		if err := git.Run(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestThaiKeys(t *testing.T) {
	m := newModel(t.TempDir(), false, "none")
	if cmd := m.handleKey("ๆ"); cmd == nil || cmd() != tea.Quit() {
		t.Error("ๆ (q on the Thai layout) should quit")
	}
}

// TestSnapshot renders both views for EXPLORER_DEMO_ROOT (skipped if unset).
func TestSnapshot(t *testing.T) {
	root := os.Getenv("EXPLORER_DEMO_ROOT")
	if root == "" {
		t.Skip("set EXPLORER_DEMO_ROOT")
	}
	m := newModel(root, false, "emoji")
	m.Update(m.loadCmd(true)())
	m.width, m.height = 90, 24
	for _, n := range m.trows {
		if n.isDir {
			m.toggleDir(n)
		}
	}
	m.trows = flatten(m.tree, nil)
	t.Log("\n" + m.View())
	// The Changes pane loads the full diff.
	m.view = viewDiff
	m.Update(m.loadCmd(true)())
	if len(m.diffs) != 5 || m.drows[0].line != -1 {
		t.Fatalf("want 5 file diffs starting with a header, got %d", len(m.diffs))
	}
	t.Log("\n" + m.View())
}
