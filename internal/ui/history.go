package ui

import (
	"herdr-explorer/internal/git"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

// Per-file git in the reader: b toggles blame (who last changed each line),
// L lists the file's commits; enter shows what that commit did to the file.

const blameW = 30 // "a1b2c3d author---- 2026-11-17 "

func (r *reader) blameW() int {
	if r.blame == nil {
		return 0
	}
	return blameW
}

func (m *model) toggleBlame() {
	r := m.reading
	if r.back != nil { // a commit's diff has no blame
		return
	}
	if r.blame != nil {
		r.blame = nil
		return
	}
	out, err := git.Out(m.root, "blame", "--line-porcelain", "--", r.rel)
	if err != nil {
		m.flash, m.flashOK, m.flashAt = git.Err(out, err).Error(), false, time.Now()
		return
	}
	r.blame = parseBlame(out)
}

// parseBlame turns `git blame --line-porcelain` into one cell per line.
func parseBlame(out string) []string {
	var cells []string
	var hash, author, date string
	for _, l := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(l, "\t"):
			cells = append(cells, blameCell(hash, author, date))
		case strings.HasPrefix(l, "author "):
			author = l[len("author "):]
		case strings.HasPrefix(l, "author-time "):
			sec, _ := strconv.ParseInt(l[len("author-time "):], 10, 64)
			date = time.Unix(sec, 0).Format("2006-01-02")
		default:
			if f := strings.Fields(l); len(f) >= 3 && len(f[0]) == 40 && git.HashRe.MatchString(f[0]) {
				hash = f[0][:7]
			}
		}
	}
	return cells
}

func blameCell(hash, author, date string) string {
	if strings.Trim(hash, "0") == "" {
		return pad("not committed", blameW)
	}
	return hash + " " + pad(ansi.Truncate(author, 10, "…"), 10) + " " + date + " "
}

func pad(s string, w int) string {
	return s + strings.Repeat(" ", max(w-ansi.StringWidth(s), 0))
}

// fileHistory (L) picks one of the file's commits and shows its diff.
func (m *model) fileHistory() tea.Cmd {
	r := m.reading
	if r.back != nil {
		return nil
	}
	out, err := git.Out(m.root, "log", "--follow", "-n", "300", "--date=short",
		"--format=%h %ad %an  %s", "--", r.rel)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if err != nil || lines[0] == "" {
		m.flash, m.flashOK, m.flashAt = "no history: file not committed yet", false, time.Now()
		return nil
	}
	return m.pick("history of "+r.rel, lines, 0, func(i int) tea.Cmd {
		hash := strings.Fields(lines[i])[0]
		text, err := git.Out(m.root, "log", "-1", "-p", "-M", "--follow", "--no-color", "--no-ext-diff",
			"--date=iso", "--format=commit %H%nAuthor: %an <%ae>%nDate:   %ad%n%n%w(0,4,4)%B", hash, "--", r.rel)
		if err != nil {
			m.flash, m.flashOK, m.flashAt = git.Err(text, err).Error(), false, time.Now()
			return nil
		}
		m.reading = textReader(r.rel, r.rel+" @ "+hash, text, r)
		return nil
	})
}

// textReader shows text (a commit's diff) in the reader; esc returns to back.
func textReader(rel, title, text string, back *reader) *reader {
	f := &git.FileDiff{Path: title}
	for i, l := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		f.Lines = append(f.Lines, git.Line{Kind: '+', Text: strings.TrimSuffix(l, "\r"), NewNo: i + 1})
	}
	r := &reader{rel: rel, back: back, f: f, lines: f.Lines}
	r.segs = highlight("commit.diff", r.lines)
	return r
}
