package git

import (
	"errors"
	"strings"
)

// Git operations behind the Fork-like keys of the Changes pane.

func Run(root string, args ...string) error { return Err(Out(root, args...)) }

func outFields(root string, args ...string) []string {
	out, err := Out(root, args...)
	if err != nil {
		return nil
	}
	return strings.Fields(out)
}

func HasHead(root string) bool {
	_, err := Out(root, "rev-parse", "--verify", "-q", "HEAD")
	return err == nil
}

func Merging(root string) bool {
	_, err := Out(root, "rev-parse", "--verify", "-q", "MERGE_HEAD")
	return err == nil
}

var ErrConflicts = errors.New("conflicts: fix each C file (O ours / T theirs / edit + space), then c to commit, X aborts")

// conflictErr replaces git's "exit status 1" after a merge/pull/pop that
// stopped on conflicts with what to do next.
func conflictErr(root string, err error) error {
	if err != nil && (Merging(root) || len(outFields(root, "diff", "--name-only", "--diff-filter=U")) > 0) {
		return ErrConflicts
	}
	return err
}

// Branches returns local branches and remote branches (without */HEAD).
func Branches(root string) (local, remote []string) {
	local = outFields(root, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	for _, r := range outFields(root, "for-each-ref", "--format=%(refname:short)", "refs/remotes") {
		if strings.Contains(r, "/") && !strings.HasSuffix(r, "/HEAD") {
			remote = append(remote, r)
		}
	}
	return local, remote
}

func Switch(root, branch string, isRemote bool) error {
	if isRemote {
		return Run(root, "switch", "--track", branch)
	}
	return Run(root, "switch", branch)
}

func Merge(root, branch string) error {
	return conflictErr(root, Run(root, "merge", "--no-edit", branch))
}

func Pull(root string) error {
	return conflictErr(root, Run(root, "pull", "--no-edit"))
}

// Push pushes; a branch without upstream is published to origin.
func Push(root string) error {
	if _, err := Out(root, "rev-parse", "--abbrev-ref", "@{upstream}"); err != nil {
		return Run(root, "push", "-u", "origin", "HEAD")
	}
	return Run(root, "push")
}

func StashPop(root string) error {
	return conflictErr(root, Run(root, "stash", "pop"))
}

// Resolve takes one side of a conflicted file and marks it resolved.
func Resolve(root, path, side string) error {
	if err := Run(root, "checkout", "--"+side, "--", path); err != nil {
		return err
	}
	return Run(root, "add", "--", path)
}

// ToggleHunk stages hunk n (0-based) of path's HEAD→worktree diff, or
// unstages it when the index already holds it.
// shortcut: staged-ness is guessed with `apply --check -R`; a hunk that is only
// partly staged can't be toggled (stage/unstage the whole file instead).
func ToggleHunk(root, path string, n int) (string, error) {
	out, err := Out(root, "diff", "HEAD", "--no-color", "--no-ext-diff", "--", path)
	if err != nil {
		return "", Err(out, err)
	}
	patch := oneHunk(out, n)
	if patch == "" {
		return "", errors.New("hunk changed: wait for the refresh and try again")
	}
	if _, err := outIn(root, patch, "apply", "--cached", "--check", "-R"); err == nil {
		return "unstaged hunk", Err(outIn(root, patch, "apply", "--cached", "-R"))
	}
	return "staged hunk", Err(outIn(root, patch, "apply", "--cached"))
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

// Discard throws away all changes to one file (the caller confirms first).
func Discard(root string, f *FileDiff) error {
	switch f.Status {
	case "C":
		return errors.New("conflicted file: use O/T or edit it")
	case "U":
		return Run(root, "clean", "-f", "-q", "--", f.Path)
	case "A":
		if err := Run(root, "rm", "-q", "--cached", "--", f.Path); err != nil {
			return err
		}
		return Run(root, "clean", "-f", "-q", "--", f.Path)
	}
	paths := []string{f.Path}
	if f.OldPath != "" {
		paths = append(paths, f.OldPath)
	}
	return Run(root, append([]string{"restore", "--source=HEAD", "--staged", "--worktree", "--"}, paths...)...)
}
