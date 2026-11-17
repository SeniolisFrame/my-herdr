package git

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Line struct {
	Kind   byte // '+', '-', ' ', '@' hunk header, '\\' "no newline" note
	Text   string
	OldNo  int
	NewNo  int
	Staged string // on '@' lines: "full" / "partial" / "" (see markHunks)
}

type FileDiff struct {
	Path    string
	OldPath string
	Status  string // M A D R U
	Adds    int
	Dels    int
	Binary  bool
	Note    string
	Lines   []Line
}

// LoadDiffs returns working tree + index changes against HEAD, plus untracked files.
func LoadDiffs(root string) ([]*FileDiff, error) {
	base := "HEAD"
	if _, err := Out(root, "rev-parse", "--verify", "-q", "HEAD"); err != nil {
		// No commits yet: diff against the empty tree.
		out, err := Out(root, "hash-object", "-t", "tree", "--stdin")
		if err != nil {
			return nil, fmt.Errorf("git hash-object: %w", err)
		}
		base = strings.TrimSpace(out)
	}
	out, err := Out(root, "diff", base, "--no-color", "--no-ext-diff", "-M")
	if err != nil {
		return nil, fmt.Errorf("git diff: %w", err)
	}
	files := ParseDiff(out)
	if out, err := Out(root, "diff", "--cached", base, "--no-color", "--no-ext-diff", "-M"); err == nil {
		markHunks(files, ParseDiff(out))
	}

	if out, err := Out(root, "ls-files", "--others", "--exclude-standard", "-z"); err == nil {
		for _, p := range strings.Split(out, "\x00") {
			if p != "" {
				files = append(files, ReadFile(root, p))
			}
		}
	}
	return files, nil
}

// markHunks marks each HEAD→worktree hunk "full" when the index (cached
// diff) holds all of its changes, "partial" when it holds some.
func markHunks(files, cached []*FileDiff) {
	in := map[string]bool{}
	for _, c := range cached {
		for _, keys := range changeKeys(c) {
			for _, k := range keys {
				in[c.Path+"\x00"+k] = true
			}
		}
	}
	for _, f := range files {
		h := 0
		keys := changeKeys(f)
		for i := range f.Lines {
			if f.Lines[i].Kind != '@' {
				continue
			}
			n := 0
			for _, k := range keys[h] {
				if in[f.Path+"\x00"+k] {
					n++
				}
			}
			switch {
			case n > 0 && n == len(keys[h]):
				f.Lines[i].Staged = "full"
			case n > 0:
				f.Lines[i].Staged = "partial"
			}
			h++
		}
	}
}

// changeKeys lists each hunk's changed lines as "HEAD line:kind:text"; both
// diffs count lines from HEAD, so a staged change has the same key in each.
func changeKeys(f *FileDiff) [][]string {
	var out [][]string
	old := 0
	for _, l := range f.Lines {
		switch l.Kind {
		case '@':
			old, _ = parseHunkHeader(l.Text)
			out = append(out, nil)
		case ' ':
			old++
		case '-', '+':
			out[len(out)-1] = append(out[len(out)-1], fmt.Sprintf("%d:%c:%s", old, l.Kind, l.Text))
			if l.Kind == '-' {
				old++
			}
		}
	}
	return out
}

const maxUntrackedBytes = 512 << 10

func ReadFile(root, rel string) *FileDiff {
	fd := &FileDiff{Path: rel, Status: "U"}
	f, err := os.Open(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		fd.Note = err.Error()
		return fd
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUntrackedBytes+1))
	if err != nil {
		fd.Note = err.Error()
		return fd
	}
	head := data
	if len(head) > 8000 {
		head = head[:8000]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		fd.Binary = true
		return fd
	}
	if len(data) > maxUntrackedBytes {
		data = data[:maxUntrackedBytes]
		fd.Note = "showing first 512 KiB"
	}
	text := strings.TrimSuffix(string(data), "\n")
	if text == "" {
		fd.Note = "empty file"
		return fd
	}
	lines := strings.Split(text, "\n")
	fd.Lines = append(fd.Lines, Line{Kind: '@', Text: fmt.Sprintf("@@ -0,0 +1,%d @@ new file", len(lines))})
	for i, l := range lines {
		fd.Lines = append(fd.Lines, Line{Kind: '+', Text: strings.TrimSuffix(l, "\r"), NewNo: i + 1})
	}
	fd.Adds = len(lines)
	return fd
}

// ParseDiff splits `git diff` output into per-file diffs with line numbers.
func ParseDiff(out string) []*FileDiff {
	var files []*FileDiff
	var cur *FileDiff
	var oldNo, newNo int
	inHunk := false
	for _, raw := range strings.Split(out, "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if strings.HasPrefix(line, "diff --git ") {
			cur = &FileDiff{Status: "M"}
			files = append(files, cur)
			inHunk = false
			rest := strings.TrimPrefix(line, "diff --git ")
			if i := strings.Index(rest, " b/"); i >= 0 {
				cur.Path = rest[i+3:]
			}
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(line, "@@") {
			oldNo, newNo = parseHunkHeader(line)
			inHunk = true
			cur.Lines = append(cur.Lines, Line{Kind: '@', Text: line})
			continue
		}
		if !inHunk {
			switch {
			case strings.HasPrefix(line, "new file mode"):
				cur.Status = "A"
			case strings.HasPrefix(line, "deleted file mode"):
				cur.Status = "D"
			case strings.HasPrefix(line, "rename from "):
				cur.Status = "R"
				cur.OldPath = strings.TrimPrefix(line, "rename from ")
			case strings.HasPrefix(line, "rename to "):
				cur.Path = strings.TrimPrefix(line, "rename to ")
			case strings.HasPrefix(line, "Binary files "):
				cur.Binary = true
			case strings.HasPrefix(line, "+++ "):
				if p := strings.TrimPrefix(line, "+++ "); p != "/dev/null" {
					cur.Path = strings.TrimPrefix(p, "b/")
				}
			case strings.HasPrefix(line, "--- "):
				if p := strings.TrimPrefix(line, "--- "); p != "/dev/null" && cur.Path == "" {
					cur.Path = strings.TrimPrefix(p, "a/")
				}
			}
			continue
		}
		if line == "" {
			continue // trailing newline of the output
		}
		switch line[0] {
		case '+':
			cur.Lines = append(cur.Lines, Line{Kind: '+', Text: line[1:], NewNo: newNo})
			newNo++
			cur.Adds++
		case '-':
			cur.Lines = append(cur.Lines, Line{Kind: '-', Text: line[1:], OldNo: oldNo})
			oldNo++
			cur.Dels++
		case ' ':
			cur.Lines = append(cur.Lines, Line{Kind: ' ', Text: line[1:], OldNo: oldNo, NewNo: newNo})
			oldNo++
			newNo++
		case '\\':
			cur.Lines = append(cur.Lines, Line{Kind: '\\', Text: line})
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
