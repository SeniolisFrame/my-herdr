package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@t"},
		{"config", "user.name", "t"},
		{"config", "core.autocrlf", "false"},
	} {
		if err := Run(dir, args...); err != nil {
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

// TestGitFlow walks the Fork-like flow: stage, commit, branch, merge with a
// conflict, resolve, stash, discard, push to a remote, log and show.
func TestGitFlow(t *testing.T) {
	r := newRepo(t)

	// Stage / unstage before the first commit, then commit.
	write(t, r, "a.txt", "one\n")
	must(t, SetStaged(r, true, "a.txt"))
	if _, staged := ReadStatus(r); staged["a.txt"] != "full" {
		t.Fatalf("a.txt should be staged, got %v", staged)
	}
	must(t, SetStaged(r, false, "a.txt"))
	if st, staged := ReadStatus(r); staged["a.txt"] != "" || st["a.txt"] != "U" {
		t.Fatalf("a.txt should be untracked again, got %v %v", st, staged)
	}
	must(t, SetStaged(r, true, "a.txt"))
	if h, err := Commit(r, "init"); err != nil || h == "" {
		t.Fatalf("commit: %q %v", h, err)
	}

	// Partial staging is reported.
	write(t, r, "a.txt", "one\ntwo\n")
	must(t, SetStaged(r, true, "a.txt"))
	write(t, r, "a.txt", "one\ntwo\nthree\n")
	if _, staged := ReadStatus(r); staged["a.txt"] != "partial" {
		t.Fatalf("want partial, got %v", staged)
	}
	must(t, Discard(r, &FileDiff{Path: "a.txt", Status: "M"}))

	// Conflicting branches → merge stops with C files, resolve, commit.
	must(t, Run(r, "switch", "-q", "-c", "feature"))
	write(t, r, "a.txt", "feature\n")
	must(t, SetStaged(r, true, "a.txt"))
	_, err := Commit(r, "feature change")
	must(t, err)
	must(t, Switch(r, "main", false))
	write(t, r, "a.txt", "main\n")
	must(t, SetStaged(r, true, "a.txt"))
	_, err = Commit(r, "main change")
	must(t, err)
	if err := Merge(r, "feature"); err != ErrConflicts {
		t.Fatalf("want errConflicts, got %v", err)
	}
	if st, _ := ReadStatus(r); st["a.txt"] != "C" || !Merging(r) {
		t.Fatalf("want conflict on a.txt while merging, got %v", st)
	}
	must(t, Resolve(r, "a.txt", "theirs"))
	if _, err := Commit(r, ""); err != nil || Merging(r) {
		t.Fatalf("merge commit: %v merging=%v", err, Merging(r))
	}
	if b, _ := os.ReadFile(filepath.Join(r, "a.txt")); string(b) != "feature\n" {
		t.Fatalf("theirs not taken: %q", b)
	}

	// Stash push / pop, discard an untracked file.
	write(t, r, "a.txt", "wip\n")
	must(t, Run(r, "stash", "push", "-q", "-u"))
	if st, _ := ReadStatus(r); len(st) != 0 {
		t.Fatalf("stash should leave a clean tree, got %v", st)
	}
	must(t, StashPop(r))
	write(t, r, "junk.txt", "x\n")
	must(t, Discard(r, &FileDiff{Path: "junk.txt", Status: "U"}))
	if _, err := os.Stat(filepath.Join(r, "junk.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked file should be gone")
	}

	// Push a branch without upstream to a bare remote, then plain push.
	remote := t.TempDir()
	must(t, Run(remote, "init", "-q", "--bare"))
	must(t, Run(r, "remote", "add", "origin", remote))
	must(t, Push(r))
	if _, _, ok := AheadBehind(r); !ok {
		t.Fatal("push should set an upstream")
	}
	must(t, SetStaged(r, true, "a.txt"))
	_, err = Commit(r, "wip")
	must(t, err)
	must(t, Push(r))
	must(t, Pull(r))

	// Log graph shows the merge; show returns the commit's diff.
	log := strings.Join(Log(r), "\n")
	if !strings.Contains(log, "Merge branch") {
		t.Fatalf("log missing merge commit:\n%s", log)
	}
	hash := LogHash(Log(r)[0])
	fs, err := Show(r, hash)
	if err != nil || len(fs) != 1 || fs[0].Path != "a.txt" {
		t.Fatalf("show %s: %v %+v", hash, err, fs)
	}
	local, remotes := Branches(r)
	if len(local) != 2 || len(remotes) != 1 {
		t.Fatalf("branches: %v %v", local, remotes)
	}
}

// TestHunkStage stages and unstages one hunk of a CRLF file with two hunks.
func TestHunkStage(t *testing.T) {
	r := newRepo(t)
	lines := make([]string, 20)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i)
	}
	body := func() string { return strings.Join(lines, "\r\n") + "\r\n" }
	write(t, r, "a.txt", body())
	must(t, SetStaged(r, true, "a.txt"))
	_, err := Commit(r, "init")
	must(t, err)
	lines[1], lines[18] = "first", "second"
	write(t, r, "a.txt", body())

	msg, err := ToggleHunk(r, "a.txt", 1)
	must(t, err)
	cached, _ := Out(r, "diff", "--cached")
	if msg != "staged hunk" || !strings.Contains(cached, "+second") || strings.Contains(cached, "+first") {
		t.Fatalf("want only the second hunk staged (%s):\n%s", msg, cached)
	}
	files, err := LoadDiffs(r)
	must(t, err)
	var marks []string
	for _, l := range files[0].Lines {
		if l.Kind == '@' {
			marks = append(marks, l.Staged)
		}
	}
	if fmt.Sprint(marks) != "[ full]" {
		t.Fatalf("hunk marks: want [ full], got %q", marks)
	}
	msg, err = ToggleHunk(r, "a.txt", 1)
	must(t, err)
	if cached, _ := Out(r, "diff", "--cached"); msg != "unstaged hunk" || cached != "" {
		t.Fatalf("toggle again should unstage (%s):\n%s", msg, cached)
	}
}
