package main

import (
	"errors"
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Git operations behind the Fork-like keys of the Changes pane.

func gitRun(root string, args ...string) error { return gitErr(git(root, args...)) }

func gitLines(root string, args ...string) []string {
	out, err := git(root, args...)
	if err != nil {
		return nil
	}
	return strings.Fields(out)
}

func gitHasHead(root string) bool {
	_, err := git(root, "rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

func gitMerging(root string) bool {
	_, err := git(root, "rev-parse", "--verify", "-q", "MERGE_HEAD")
	return err == nil
}

var errConflicts = errors.New("conflicts: fix each C file (O ours / T theirs / edit + space), then c to commit, X aborts")

// conflictErr replaces git's "exit status 1" after a merge/pull/pop that
// stopped on conflicts with what to do next.
func conflictErr(root string, err error) error {
	if err != nil && (gitMerging(root) || len(gitLines(root, "diff", "--name-only", "--diff-filter=U")) > 0) {
		return errConflicts
	}
	return err
}

// gitBranches returns local branches and remote branches (without */HEAD).
func gitBranches(root string) (local, remote []string) {
	local = gitLines(root, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	for _, r := range gitLines(root, "for-each-ref", "--format=%(refname:short)", "refs/remotes") {
		if strings.Contains(r, "/") && !strings.HasSuffix(r, "/HEAD") {
			remote = append(remote, r)
		}
	}
	return local, remote
}

func gitSwitch(root, branch string, isRemote bool) error {
	if isRemote {
		return gitRun(root, "switch", "--track", branch)
	}
	return gitRun(root, "switch", branch)
}

func gitMerge(root, branch string) error {
	return conflictErr(root, gitRun(root, "merge", "--no-edit", branch))
}

func gitPull(root string) error {
	return conflictErr(root, gitRun(root, "pull", "--no-edit"))
}

// gitPush pushes; a branch without upstream is published to origin.
func gitPush(root string) error {
	if _, err := git(root, "rev-parse", "--abbrev-ref", "@{upstream}"); err != nil {
		return gitRun(root, "push", "-u", "origin", "HEAD")
	}
	return gitRun(root, "push")
}

func gitStashPop(root string) error {
	return conflictErr(root, gitRun(root, "stash", "pop"))
}

// gitResolve takes one side of a conflicted file and marks it resolved.
func gitResolve(root, path, side string) error {
	if err := gitRun(root, "checkout", "--"+side, "--", path); err != nil {
		return err
	}
	return gitRun(root, "add", "--", path)
}

// gitToggleHunk stages hunk n (0-based) of path's HEAD→worktree diff, or
// unstages it when the index already holds it.
// shortcut: staged-ness is guessed with `apply --check -R`; a hunk that is only
// partly staged can't be toggled (stage/unstage the whole file instead).
func gitToggleHunk(root, path string, n int) (string, error) {
	out, err := git(root, "diff", "HEAD", "--no-color", "--no-ext-diff", "--", path)
	if err != nil {
		return "", gitErr(out, err)
	}
	patch := oneHunk(out, n)
	if patch == "" {
		return "", errors.New("hunk changed: wait for the refresh and try again")
	}
	if _, err := gitIn(root, patch, "apply", "--cached", "--check", "-R"); err == nil {
		return "unstaged hunk", gitErr(gitIn(root, patch, "apply", "--cached", "-R"))
	}
	return "staged hunk", gitErr(gitIn(root, patch, "apply", "--cached"))
}

// oneHunk keeps the file header and hunk n of a one-file diff, byte for byte
// (CRLF included); "" if there is no such hunk.
func oneHunk(diff string, n int) string {
	var head, hunk strings.Builder
	i := -1
	for _, l := range strings.SplitAfter(diff, "\n") {
		if strings.HasPrefix(l, "@@") {
			i++
		}
		switch {
		case i < 0:
			head.WriteString(l)
		case i == n:
			hunk.WriteString(l)
		}
	}
	if hunk.Len() == 0 {
		return ""
	}
	return head.String() + hunk.String()
}

// gitDiscard throws away all changes to one file (the caller confirms first).
func gitDiscard(root string, f *fileDiff) error {
	switch f.status {
	case "C":
		return errors.New("conflicted file: use O/T or edit it")
	case "U":
		return gitRun(root, "clean", "-f", "-q", "--", f.path)
	case "A":
		if err := gitRun(root, "rm", "-q", "--cached", "--", f.path); err != nil {
			return err
		}
		return gitRun(root, "clean", "-f", "-q", "--", f.path)
	}
	paths := []string{f.path}
	if f.oldPath != "" {
		paths = append(paths, f.oldPath)
	}
	return gitRun(root, append([]string{"restore", "--source=HEAD", "--staged", "--worktree", "--"}, paths...)...)
}

// gitLog returns the commit graph of all branches, coloured by git.
func gitLog(root string) []string {
	out, err := git(root, "log", "--graph", "--all", "--date-order", "--color=always",
		"--format=%C(auto)%h%d %s %C(dim)%an, %ar%C(reset)", "-n", "500")
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for i, l := range lines {
		// Redraw git's ASCII graph (the part before the hash) with box lines.
		// ANSI colour codes contain none of these characters.
		end := len(l)
		if h := logHash(l); h != "" {
			end = strings.Index(l, h)
		}
		lines[i] = graphChars.Replace(l[:end]) + l[end:]
	}
	return lines
}

var graphChars = strings.NewReplacer("*", "●", "|", "│", "/", "╱", "\\", "╲", "_", "─")

var hashRe = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)

// logHash finds the commit hash on a (coloured) graph line; "" for pure graph lines.
func logHash(line string) string { return hashRe.FindString(ansi.Strip(line)) }

// gitShow returns the diff a commit introduced (vs its first parent).
func gitShow(root, hash string) ([]*fileDiff, error) {
	out, err := git(root, "show", "--format=", "--no-color", "--no-ext-diff", "-M", "-m", "--first-parent", hash)
	if err != nil {
		return nil, gitErr(out, err)
	}
	return parseDiff(out), nil
}
