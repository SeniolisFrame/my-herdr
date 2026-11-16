package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func git(root string, args ...string) (string, error) {
	full := append([]string{"-C", root, "-c", "core.quotepath=off"}, args...)
	out, err := exec.Command("git", full...).Output()
	return string(out), err
}

func gitTopLevel(dir string) string {
	out, err := git(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return filepath.FromSlash(strings.TrimSpace(out))
}

// gitBranch returns the branch name, or "detached@<sha>" / "" outside a repo.
func gitBranch(root string) string {
	if out, err := git(root, "symbolic-ref", "--short", "-q", "HEAD"); err == nil {
		if b := strings.TrimSpace(out); b != "" {
			return b
		}
	}
	if out, err := git(root, "rev-parse", "--short", "HEAD"); err == nil {
		return "detached@" + strings.TrimSpace(out)
	}
	return ""
}

// gitAheadBehind returns commits ahead/behind the upstream; ok=false if none.
func gitAheadBehind(root string) (ahead, behind int, ok bool) {
	out, err := git(root, "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
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

// gitStatus maps slash-separated paths (relative to the repo top) to one of
// M, A, D, R, U (untracked).
func gitStatus(root string) map[string]string {
	st := map[string]string{}
	out, err := git(root, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return st
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
	}
	return st
}

type diffLine struct {
	kind  byte // '+', '-', ' ', '@' hunk header, '\\' "no newline" note
	text  string
	oldNo int
	newNo int
}

type fileDiff struct {
	path    string
	oldPath string
	status  string // M A D R U
	adds    int
	dels    int
	binary  bool
	note    string
	lines   []diffLine
}

// loadDiffs returns working tree + index changes against HEAD, plus untracked files.
func loadDiffs(root string) ([]*fileDiff, error) {
	base := "HEAD"
	if _, err := git(root, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		// No commits yet: diff against the empty tree.
		out, err := git(root, "hash-object", "-t", "tree", "--stdin")
		if err != nil {
			return nil, fmt.Errorf("git hash-object: %w", err)
		}
		base = strings.TrimSpace(out)
	}
	out, err := git(root, "diff", base, "--no-color", "--no-ext-diff", "-M")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	files := parseDiff(out)

	if out, err := git(root, "ls-files", "--others", "--exclude-standard", "-z"); err == nil {
		for _, p := range strings.Split(out, "\x00") {
			if p != "" {
				files = append(files, untrackedDiff(root, p))
			}
		}
	}
	return files, nil
}

const maxUntrackedBytes = 512 << 10

func untrackedDiff(root, rel string) *fileDiff {
	fd := &fileDiff{path: rel, status: "U"}
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		fd.note = err.Error()
		return fd
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUntrackedBytes+1))
	if err != nil {
		fd.note = err.Error()
		return fd
	}
	head := data
	if len(head) > 8000 {
		head = head[:8000]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		fd.binary = true
		return fd
	}
	if len(data) > maxUntrackedBytes {
		data = data[:maxUntrackedBytes]
		fd.note = "showing first 512 KiB"
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		fd.note = "empty file"
		return fd
	}
	lines := strings.Split(text, "\n")
	fd.lines = append(fd.lines, diffLine{kind: '@', text: fmt.Sprintf("@@ -0,0 +1,%d @@ new file", len(lines))})
	for i, l := range lines {
		fd.lines = append(fd.lines, diffLine{kind: '+', text: strings.TrimSuffix(l, "\r"), newNo: i + 1})
	}
	fd.adds = len(lines)
	return fd
}

// parseDiff splits `git diff` output into per-file diffs with line numbers.
func parseDiff(out string) []*fileDiff {
	var files []*fileDiff
	var cur *fileDiff
	var oldNo, newNo int
	inHunk := false
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if strings.HasPrefix(line, "diff --git ") {
			cur = &fileDiff{status: "M"}
			files = append(files, cur)
			inHunk = false
			rest := strings.TrimPrefix(line, "diff --git ")
			if i := strings.Index(rest, " b/"); i >= 0 {
				cur.path = rest[i+3:]
			}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "@@") {
			oldNo, newNo = parseHunkHeader(line)
			inHunk = true
			cur.lines = append(cur.lines, diffLine{kind: '@', text: line})
			continue
		}
		if !inHunk {
			switch {
			case strings.HasPrefix(line, "new file mode"):
				cur.status = "A"
			case strings.HasPrefix(line, "deleted file mode"):
				cur.status = "D"
			case strings.HasPrefix(line, "rename from "):
				cur.status = "R"
				cur.oldPath = strings.TrimPrefix(line, "rename from ")
			case strings.HasPrefix(line, "rename to "):
				cur.path = strings.TrimPrefix(line, "rename to ")
			case strings.HasPrefix(line, "Binary files "):
				cur.binary = true
			case strings.HasPrefix(line, "+++ "):
				if p := strings.TrimPrefix(line, "+++ "); p != "/dev/null" {
					cur.path = strings.TrimPrefix(p, "b/")
				}
			case strings.HasPrefix(line, "--- "):
				if p := strings.TrimPrefix(line, "--- "); p != "/dev/null" && cur.path == "" {
					cur.path = strings.TrimPrefix(p, "a/")
				}
			}
			continue
		}
		if line == "" {
			continue // trailing newline of the output
		}
		switch line[0] {
		case '+':
			cur.lines = append(cur.lines, diffLine{kind: '+', text: line[1:], newNo: newNo})
			newNo++
			cur.adds++
		case '-':
			cur.lines = append(cur.lines, diffLine{kind: '-', text: line[1:], oldNo: oldNo})
			oldNo++
			cur.dels++
		case ' ':
			cur.lines = append(cur.lines, diffLine{kind: ' ', text: line[1:], oldNo: oldNo, newNo: newNo})
			oldNo++
			newNo++
		case '\\':
			cur.lines = append(cur.lines, diffLine{kind: '\\', text: line})
		}
	}
	return files
}

// parseHunkHeader reads the start lines from "@@ -12,5 +12,7 @@ ...".
func parseHunkHeader(line string) (oldStart, newStart int) {
	f := strings.Fields(line)
	if len(f) < 3 {
		return 1, 1
	}
	num := func(s string) int {
		s = strings.TrimLeft(s, "-+")
		if i := strings.IndexByte(s, ','); i >= 0 {
			s = s[:i]
		}
		n, _ := strconv.Atoi(s)
		return n
	}
	return num(f[1]), num(f[2])
}
