// Package git runs the git commands behind the explorer panes.
package git

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func Out(root string, args ...string) (string, error) { return outIn(root, "", args...) }

// outIn is git with stdin (for `git apply` of a patch).
func outIn(root, stdin string, args ...string) (string, error) {
	// --no-optional-locks: the 2s status poll must not hold index.lock while a
	// stage/commit runs. No prompts/editors: they would hang inside the TUI.
	full := append([]string{"--no-optional-locks", "-C", root, "-c", "core.quotepath=off"}, args...)
	cmd := exec.Command("git", full...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_EDITOR=true")
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	return string(out), err
}

func TopLevel(dir string) string {
	out, err := Out(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return filepath.FromSlash(strings.TrimSpace(out))
}

// Branch returns the branch name, or "detached@<sha>" / "" outside a repo.
func Branch(root string) string {
	if out, err := Out(root, "symbolic-ref", "--short", "-q", "HEAD"); err == nil {
		if b := strings.TrimSpace(out); b != "" {
			return b
		}
	}
	if out, err := Out(root, "rev-parse", "--short", "HEAD"); err == nil {
		return "detached@" + strings.TrimSpace(out)
	}
	return ""
}

// AheadBehind returns commits ahead/behind the upstream; ok=false if none.
func AheadBehind(root string) (ahead, behind int, ok bool) {
	out, err := Out(root, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	if err != nil {
		return 0, 0, false
	}
	f := strings.Fields(out)
	if len(f) != 2 {
		return 0, 0, false
	}
	ahead, _ = strconv.Atoi(f[0])
	behind, _ = strconv.Atoi(f[1])
	return ahead, behind, true
}

// ReadStatus maps slash-separated paths (relative to the repo top) to one of
// M, A, D, R, U (untracked), and staged paths to "full" or "partial".
func ReadStatus(root string) (st, staged map[string]string) {
	st, staged = map[string]string{}, map[string]string{}
	out, err := Out(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return st, staged
	}
	parts := strings.Split(out, "\x00")
	for i := 0; i < len(parts); i++ {
		e := parts[i]
		if len(e) < 4 {
			continue
		}
		xy, p := e[:2], e[3:]
		if xy[0] == 'R' || xy[0] == 'C' {
			i++ // the original path follows
		}
		if strings.Contains(xy, "U") && xy != "??" || xy == "AA" || xy == "DD" {
			st[p] = "C" // merge conflict
			continue
		}
		switch {
		case xy == "??":
			st[p] = "U"
		case strings.Contains(xy, "D"):
			st[p] = "D"
		case strings.Contains(xy, "A"):
			st[p] = "A"
		case strings.Contains(xy, "R"):
			st[p] = "R"
		default:
			st[p] = "M"
		}
		if xy[0] != ' ' && xy[0] != '?' {
			staged[p] = "full"
			if xy[1] != ' ' {
				staged[p] = "partial"
			}
		}
	}
	return st, staged
}

// SetStaged stages (git add) or unstages a file. paths includes the old name
// of a rename so both sides move together.
func SetStaged(root string, stage bool, paths ...string) error {
	args := append([]string{"add", "-A", "--"}, paths...)
	if !stage {
		args = append([]string{"restore", "--staged", "--"}, paths...)
		if !HasHead(root) {
			args = append([]string{"rm", "-r", "-q", "--cached", "--"}, paths...) // no commits yet
		}
	}
	return Run(root, args...)
}

// Commit commits what is staged and returns the new short hash. An empty
// message finishing a merge keeps git's "Merge branch ..." message.
func Commit(root, msg string) (string, error) {
	args := []string{"commit", "-q", "-m", msg}
	if msg == "" && Merging(root) {
		args = []string{"commit", "-q", "--no-edit"}
	}
	if err := Run(root, args...); err != nil {
		return "", err
	}
	out, err := Out(root, "rev-parse", "--short", "HEAD")
	return strings.TrimSpace(out), err
}

// Err turns git's stderr into the error text the footer shows.
func Err(out string, err error) error {
	var ee *exec.ExitError
	if errors.As(err, &ee) && len(ee.Stderr) > 0 {
		return errors.New(strings.TrimSpace(string(ee.Stderr)))
	}
	// Some failures (e.g. "nothing to commit") are reported on stdout only.
	if lines := strings.Split(strings.TrimSpace(out), "\n"); err != nil && lines[len(lines)-1] != "" {
		return errors.New(lines[len(lines)-1])
	}
	return err
}
