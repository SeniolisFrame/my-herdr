package herdr

import (
	"path/filepath"
	"strings"
	"testing"
)

func filepathIsAbs(p string) bool { return filepath.IsAbs(filepath.FromSlash(p)) }

func TestMention(t *testing.T) {
	root := t.TempDir()
	cases := []struct{ rel, cwd, want string }{
		{"src/main.go", root, "@src/main.go "},
		{"my docs/a.md", root, `@"my docs/a.md" `},
	}
	for _, c := range cases {
		if got := Mention(root, c.rel, c.cwd); got != c.want {
			t.Errorf("mention(%q) = %q, want %q", c.rel, got, c.want)
		}
	}
	for _, bad := range []string{"a\nrm -rf ~", "a\rb", "a\x1bb"} {
		if _, err := SendToChat(Context{}, root, bad); err == nil || !strings.Contains(err.Error(), "control") {
			t.Errorf("sendToChat(%q) must refuse control characters, got %v", bad, err)
		}
	}
	// Target pane elsewhere: absolute path.
	if got := Mention(root, "a.go", t.TempDir()); got[0] != '@' || !filepathIsAbs(got[1:len(got)-1]) {
		t.Errorf("want absolute mention, got %q", got)
	}
}
