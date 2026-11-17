package git

import "testing"

func TestParseDiff(t *testing.T) {
	out := "diff --git a/a.go b/a.go\nindex 1..2 100644\n--- a/a.go\n+++ b/a.go\n@@ -1,3 +1,3 @@ func x\n ctx\n-old\n+new\n ctx2\n" +
		"diff --git a/old.txt b/new.txt\nsimilarity index 100%\nrename from old.txt\nrename to new.txt\n" +
		"diff --git a/gone.md b/gone.md\ndeleted file mode 100644\n--- a/gone.md\n+++ /dev/null\n@@ -1 +0,0 @@\n-bye\n"
	fs := ParseDiff(out)
	if len(fs) != 3 {
		t.Fatalf("want 3 files, got %d", len(fs))
	}
	if f := fs[0]; f.Path != "a.go" || f.Adds != 1 || f.Dels != 1 || f.Lines[3].NewNo != 2 || f.Lines[2].OldNo != 2 {
		t.Errorf("bad first file: %+v", f)
	}
	if f := fs[1]; f.Status != "R" || f.OldPath != "old.txt" || f.Path != "new.txt" {
		t.Errorf("bad rename: %+v", f)
	}
	if f := fs[2]; f.Status != "D" || f.Path != "gone.md" || f.Dels != 1 {
		t.Errorf("bad delete: %+v", f)
	}
}
