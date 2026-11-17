package main

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
		if err := gitRun(dir, args...); err != nil {
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
	must(t, setStaged(r, true, "a.txt"))
	if _, staged := gitStatus(r); staged["a.txt"] != "full" {
		t.Fatalf("a.txt should be staged, got %v", staged)
	}
	must(t, setStaged(r, false, "a.txt"))
	if st, staged := gitStatus(r); staged["a.txt"] != "" || st["a.txt"] != "U" {
		t.Fatalf("a.txt should be untracked again, got %v %v", st, staged)
	}
	must(t, setStaged(r, true, "a.txt"))
	if h, err := gitCommit(r, "init"); err != nil || h == "" {
		t.Fatalf("commit: %q %v", h, err)
	}

	// Partial staging is reported.
	write(t, r, "a.txt", "one\ntwo\n")
	must(t, setStaged(r, true, "a.txt"))
	write(t, r, "a.txt", "one\ntwo\nthree\n")
	if _, staged := gitStatus(r); staged["a.txt"] != "partial" {
		t.Fatalf("want partial, got %v", staged)
	}
	must(t, gitDiscard(r, &fileDiff{path: "a.txt", status: "M"}))

	// Conflicting branches → merge stops with C files, resolve, commit.
	must(t, gitRun(r, "switch", "-q", "-c", "feature"))
	write(t, r, "a.txt", "feature\n")
	must(t, setStaged(r, true, "a.txt"))
	_, err := gitCommit(r, "feature change")
	must(t, err)
	must(t, gitSwitch(r, "main", false))
	write(t, r, "a.txt", "main\n")
	must(t, setStaged(r, true, "a.txt"))
	_, err = gitCommit(r, "main change")
	must(t, err)
	if err := gitMerge(r, "feature"); err != errConflicts {
		t.Fatalf("want errConflicts, got %v", err)
	}
	if st, _ := gitStatus(r); st["a.txt"] != "C" || !gitMerging(r) {
		t.Fatalf("want conflict on a.txt while merging, got %v", st)
	}
	must(t, gitResolve(r, "a.txt", "theirs"))
	if _, err := gitCommit(r, ""); err != nil || gitMerging(r) {
		t.Fatalf("merge commit: %v merging=%v", err, gitMerging(r))
	}
	if b, _ := os.ReadFile(filepath.Join(r, "a.txt")); string(b) != "feature\n" {
		t.Fatalf("theirs not taken: %q", b)
	}

	// Stash push / pop, discard an untracked file.
	write(t, r, "a.txt", "wip\n")
	must(t, gitRun(r, "stash", "push", "-q", "-u"))
	if st, _ := gitStatus(r); len(st) != 0 {
		t.Fatalf("stash should leave a clean tree, got %v", st)
	}
	must(t, gitStashPop(r))
	write(t, r, "junk.txt", "x\n")
	must(t, gitDiscard(r, &fileDiff{path: "junk.txt", status: "U"}))
	if _, err := os.Stat(filepath.Join(r, "junk.txt")); !os.IsNotExist(err) {
		t.Fatal("untracked file should be gone")
	}

	// Push a branch without upstream to a bare remote, then plain push.
	remote := t.TempDir()
	must(t, gitRun(remote, "init", "-q", "--bare"))
	must(t, gitRun(r, "remote", "add", "origin", remote))
	must(t, gitPush(r))
	if _, _, ok := gitAheadBehind(r); !ok {
		t.Fatal("push should set an upstream")
	}
	must(t, setStaged(r, true, "a.txt"))
	_, err = gitCommit(r, "wip")
	must(t, err)
	must(t, gitPush(r))
	must(t, gitPull(r))

	// Log graph shows the merge; show returns the commit's diff.
	log := strings.Join(gitLog(r), "\n")
	if !strings.Contains(log, "Merge branch") {
		t.Fatalf("log missing merge commit:\n%s", log)
	}
	hash := logHash(gitLog(r)[0])
	fs, err := gitShow(r, hash)
	if err != nil || len(fs) != 1 || fs[0].path != "a.txt" {
		t.Fatalf("show %s: %v %+v", hash, err, fs)
	}
	local, remotes := gitBranches(r)
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
	must(t, setStaged(r, true, "a.txt"))
	_, err := gitCommit(r, "init")
	must(t, err)
	lines[1], lines[18] = "first", "second"
	write(t, r, "a.txt", body())

	msg, err := gitToggleHunk(r, "a.txt", 1)
	must(t, err)
	cached, _ := git(r, "diff", "--cached")
	if msg != "staged hunk" || !strings.Contains(cached, "+second") || strings.Contains(cached, "+first") {
		t.Fatalf("want only the second hunk staged (%s):\n%s", msg, cached)
	}
	msg, err = gitToggleHunk(r, "a.txt", 1)
	must(t, err)
	if cached, _ := git(r, "diff", "--cached"); msg != "unstaged hunk" || cached != "" {
		t.Fatalf("toggle again should unstage (%s):\n%s", msg, cached)
	}
}
