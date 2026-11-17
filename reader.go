package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// File reader in the Files pane: enter on a file shows it read-only,
// syntax-highlighted and wrapped, with the pane zoomed so it is wide enough
// to read; / searches the file, esc closes it.

type seg struct {
	style lipgloss.Style
	text  string
}

type rrow struct {
	src   int // source line index
	first bool
	text  string
}

type reader struct {
	f       *fileDiff
	lines   []diffLine // the file's lines (no '@' header)
	segs    [][]seg    // highlighted, per source line
	rows    []rrow     // wrapped to width
	width   int
	off     int // first visible row
	query   string
	matches []int // source lines containing query
	mcur    int
}

func (m *model) openReader(rel string) tea.Cmd {
	f := untrackedDiff(m.root, rel)
	r := &reader{f: f, lines: f.lines}
	if len(r.lines) > 0 && r.lines[0].kind == '@' {
		r.lines = r.lines[1:]
	}
	r.segs = highlight(rel, r.lines)
	m.reading = r
	return zoomCmd(true)
}

// highlight colours the whole file at once so multi-line comments and
// strings stay right, then splits the tokens back into lines.
func highlight(name string, lines []diffLine) [][]seg {
	text := make([]string, len(lines))
	for i, l := range lines {
		text[i] = strings.ReplaceAll(l.text, "\t", "    ")
	}
	src := strings.Join(text, "\n")
	out := make([][]seg, len(lines))
	lexer := lexers.Match(path.Base(name))
	if lexer == nil {
		lexer = lexers.Analyse(src)
	}
	var it chroma.Iterator
	if lexer != nil {
		it, _ = chroma.Coalesce(lexer).Tokenise(nil, src)
	}
	if it == nil || len(lines) == 0 {
		for i, t := range text {
			out[i] = []seg{{lipgloss.NewStyle(), t}}
		}
		return out
	}
	theme := styles.Get("onedark")
	cache := map[chroma.TokenType]lipgloss.Style{}
	i := 0
	for tok := it(); tok != chroma.EOF; tok = it() {
		st, ok := cache[tok.Type]
		if !ok {
			e := theme.Get(tok.Type)
			st = lipgloss.NewStyle().Bold(e.Bold == chroma.Yes).Italic(e.Italic == chroma.Yes)
			if e.Colour.IsSet() {
				st = st.Foreground(lipgloss.Color(e.Colour.String()))
			}
			cache[tok.Type] = st
		}
		for j, part := range strings.Split(tok.Value, "\n") {
			if j > 0 {
				i++
			}
			if part != "" && i < len(out) {
				out[i] = append(out[i], seg{st, part})
			}
		}
	}
	return out
}

const gutterW = 7 // "1234 │ "

// layout wraps the highlighted lines to the pane width (cached per width).
func (r *reader) layout(width int) {
	w := max(width-gutterW, 8)
	if r.width == w && r.rows != nil {
		return
	}
	top := 0 // keep the same source line at the top after a resize
	if r.off < len(r.rows) {
		top = r.rows[r.off].src
	}
	r.width, r.rows = w, r.rows[:0]
	for i, segs := range r.segs {
		var cur strings.Builder
		used, first := 0, true
		flush := func() {
			r.rows = append(r.rows, rrow{i, first, cur.String()})
			cur.Reset()
			used, first = 0, false
		}
		for _, s := range segs {
			text := s.text
			for text != "" {
				part := ansi.Truncate(text, w-used, "")
				if part == "" && used > 0 { // next character does not fit on this row
					flush()
					continue
				}
				if part == "" {
					_, size := utf8.DecodeRuneInString(text)
					part = text[:size]
				}
				cur.WriteString(s.style.Render(part))
				used += ansi.StringWidth(part)
				text = text[len(part):]
				if text != "" {
					flush()
				}
			}
		}
		flush()
	}
	for i, row := range r.rows {
		if row.src == top && row.first {
			r.off = i
			break
		}
	}
}

func (m *model) readerKey(k string) tea.Cmd {
	r := m.reading
	h := m.contentHeight()
	r.layout(m.width)
	switch k {
	case "esc", "q", "backspace", "enter", "left", "h":
		m.reading = nil
		return zoomCmd(false)
	case "s":
		return m.sendCmd(r.f.path)
	case "/":
		return m.ask("search file", func(q string) tea.Cmd {
			r.search(q)
			r.jump(0, h)
			return nil
		})
	case "n":
		r.jump(1, h)
	case "N":
		r.jump(-1, h)
	case "up", "k":
		r.off--
	case "down", "j":
		r.off++
	case "pgup", "ctrl+u":
		r.off -= h
	case "pgdown", "ctrl+d", " ":
		r.off += h
	case "home", "g":
		r.off = 0
	case "end", "G":
		r.off = len(r.rows)
	}
	r.off = min(max(r.off, 0), max(len(r.rows)-h, 0))
	return nil
}

// search finds the source lines containing q (ignoring case).
func (r *reader) search(q string) {
	r.query, r.matches, r.mcur = q, nil, -1
	lq := strings.ToLower(q)
	for i, l := range r.lines {
		if q != "" && strings.Contains(strings.ToLower(l.text), lq) {
			r.matches = append(r.matches, i)
		}
	}
}

// jump moves to the next (+1) / previous (-1) match, or with 0 to the first
// match at or below the top of the screen.
func (r *reader) jump(dir, h int) {
	n := len(r.matches)
	if n == 0 {
		return
	}
	if dir == 0 {
		r.mcur = 0
		for i, src := range r.matches {
			if r.off < len(r.rows) && src >= r.rows[r.off].src {
				r.mcur = i
				break
			}
		}
	} else {
		r.mcur = ((r.mcur+dir)%n + n) % n
	}
	r.goLine(r.matches[r.mcur], h)
}

// goLine scrolls so source line src sits a third of the way down.
func (r *reader) goLine(src, h int) {
	for i, row := range r.rows {
		if row.src == src && row.first {
			r.off = min(max(i-h/3, 0), max(len(r.rows)-h, 0))
			return
		}
	}
}

var (
	sMatch    = lipgloss.NewStyle().Foreground(cMod)
	sMatchCur = lipgloss.NewStyle().Reverse(true).Foreground(cMod)
)

func (m *model) readerLines(h int) []string {
	r := m.reading
	switch {
	case r.f.binary:
		return []string{sDim.Render(" binary file")}
	case r.f.note != "" && len(r.lines) == 0:
		return []string{sDim.Render(" " + r.f.note)}
	}
	r.layout(m.width)
	hit := map[int]bool{}
	for _, src := range r.matches {
		hit[src] = true
	}
	var out []string
	for i := r.off; i < len(r.rows) && len(out) < h; i++ {
		row := r.rows[i]
		g := sDim.Render("     │ ")
		if row.first {
			num := lineNo(row.src + 1)
			switch {
			case r.mcur >= 0 && r.mcur < len(r.matches) && r.matches[r.mcur] == row.src:
				num = sMatchCur.Render(num)
			case hit[row.src]:
				num = sMatch.Render(num)
			default:
				num = sDim.Render(num)
			}
			g = num + sDim.Render(" │ ")
		}
		out = append(out, g+row.text)
	}
	return out
}

func (m *model) readerFooter() string {
	r := m.reading
	if r.query == "" {
		return sDim.Render("/ search  s send  esc close")
	}
	if len(r.matches) == 0 {
		return sErr.Render("no match: " + r.query)
	}
	return sBranch.Render(fmt.Sprintf("/%s %d/%d", r.query, r.mcur+1, len(r.matches))) + sDim.Render("  n/N next/prev  esc close")
}

// searchAll (/ in the Files tree) greps every file and lists the hits;
// enter opens the file at that line with the search kept.
func (m *model) searchAll() tea.Cmd {
	return m.ask("search all files", func(q string) tea.Cmd {
		if q == "" {
			return nil
		}
		where := "--untracked"
		if !m.isGit {
			where = "--no-index"
		}
		out, _ := git(m.root, "grep", where, "-n", "-I", "-i", "-F", "--no-color", "-e", q)
		var hits, choices []string
		for _, l := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
			if p := strings.SplitN(l, ":", 3); len(p) == 3 && len(hits) < 500 {
				hits = append(hits, l)
				choices = append(choices, p[0]+":"+p[1]+"  "+strings.TrimSpace(p[2]))
			}
		}
		if len(hits) == 0 {
			m.flash, m.flashOK, m.flashAt = "no match: "+q, false, time.Now()
			return nil
		}
		m.pick(fmt.Sprintf("%d hits for %q", len(hits), q), choices, 0, func(i int) tea.Cmd {
			p := strings.SplitN(hits[i], ":", 3)
			cmd := m.openReader(p[0])
			n, _ := strconv.Atoi(p[1])
			m.reading.search(q)
			m.reading.layout(m.width)
			for j, src := range m.reading.matches {
				if src == n-1 {
					m.reading.mcur = j
				}
			}
			m.reading.goLine(n-1, m.contentHeight())
			return cmd
		})
		return zoomCmd(true)
	})
}

// zoomCmd zooms this herdr pane in or out; outside herdr it does nothing.
func zoomCmd(on bool) tea.Cmd {
	return func() tea.Msg {
		if pane := os.Getenv("HERDR_PANE_ID"); pane != "" {
			flag := map[bool]string{true: "--on", false: "--off"}[on]
			_ = exec.Command(herdrBin(), "pane", "zoom", "--pane", pane, flag).Run()
		}
		return nil
	}
}
