package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func filepathIsAbs(p string) bool { return filepath.IsAbs(filepath.FromSlash(p)) }

func TestParseDiff(t *testing.T) {
	out := "diff --git a/a.go b/a.go\nindex 1..2 100644\n--- a/a.go\n+++ b/a.go\n@@ -1,3 +1,3 @@ func x\n ctx\n-old\n+new\n ctx2\n" +
		"diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n" +
		"diff --git a/gone.md b/gone.md\ndeleted file mode 100644\n--- a/gone.md\n+++ /dev/null\n@@ -1 +0,0 @@\n-bye\n"
	fs := parseDiff(out)
	if len(fs) != 3 {
		t.Fatalf("want 3 files, got %d", len(fs))
	}
	if f := fs[0]; f.path != "a.go" || f.adds != 1 || f.dels != 1 || f.lines[3].newNo != 2 || f.lines[2].oldNo != 2 {
		t.Errorf("bad first file: %+v", f)
	}
	if f := fs[1]; f.status != "R" || f.oldPath != "old.txt" || f.path != "new.txt" {
		t.Errorf("bad rename: %+v", f)
	}
	if f := fs[2]; f.status != "D" || f.path != "gone.md" || f.dels != 1 {
		t.Errorf("bad delete: %+v", f)
	}
}

func TestMention(t *testing.T) {
	root := t.TempDir()
	cases := []struct{ rel, cwd, want string }{
		{"src/main.go", root, "@src/main.go "},
		{"my docs/a.md", root, `@"my docs/a.md" `},
	}
	for _, c := range cases {
		if got := mention(root, c.rel, c.cwd); got != c.want {
			t.Errorf("mention(%q) = %q, want %q", c.rel, got, c.want)
		}
	}
	for _, bad := range []string{"a\nrm -rf ~", "a\rb", "a\x1bb"} {
		if _, err := sendToChat(launchContext{}, root, bad); err == nil || !strings.Contains(err.Error(), "control") {
			t.Errorf("sendToChat(%q) must refuse control characters, got %v", bad, err)
		}
	}
	// Target pane elsewhere: absolute path.
	if got := mention(root, "a.go", t.TempDir()); got[0] != '@' || !filepathIsAbs(got[1:len(got)-1]) {
		t.Errorf("want absolute mention, got %q", got)
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
	m := newModel(resolveRoot(root), false, envOr("EXPLORER_ICONS", "emoji"))
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
