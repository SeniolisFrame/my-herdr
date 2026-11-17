package git

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Log returns the commit graph of all branches, coloured by git.
func Log(root string) []string {
	out, err := Out(root, "log", "--graph", "--all", "--date-order", "--color=always",
		"--format=%C(auto)%h%d %s %C(dim)%an, %ar%C(reset)", "-n", "500")
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	for i, l := range lines {
		// Redraw git's ASCII graph (the part before the hash) with box lines.
		// ANSI colour codes contain none of these characters.
		end := len(l)
		if h := LogHash(l); h != "" {
			end = strings.Index(l, h)
		}
		lines[i] = graphChars.Replace(l[:end]) + l[end:]
	}
	return lines
}

var graphChars = strings.NewReplacer("*", "●", "|", "│", "/", "╱", "\\", "╲", "_", "─")

var HashRe = regexp.MustCompile(`\b[0-9a-f]{7,40}\b`)

// LogHash finds the commit hash on a (coloured) graph line; "" for pure graph lines.
func LogHash(line string) string { return HashRe.FindString(ansi.Strip(line)) }

// Show returns the diff a commit introduced (vs its first parent).
func Show(root, hash string) ([]*FileDiff, error) {
	out, err := Out(root, "show", "--format=", "--no-color", "--no-ext-diff", "-M", "-m", "--first-parent", hash)
	if err != nil {
		return nil, Err(out, err)
	}
	return ParseDiff(out), nil
}
