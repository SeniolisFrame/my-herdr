package ui

import (
	"fmt"
	"herdr-explorer/internal/git"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// ---------- view ----------

var (
	cAccent  = lipgloss.Color("#58a6ff")
	cDim     = lipgloss.Color("#8b949e")
	cAdd     = lipgloss.Color("#3fb950")
	cDel     = lipgloss.Color("#f85149")
	cMod     = lipgloss.Color("#d29922")
	cRen     = lipgloss.Color("#a371f7")
	cBranch  = lipgloss.Color("#f0883e")
	sDim     = lipgloss.NewStyle().Foreground(cDim)
	sAdd     = lipgloss.NewStyle().Foreground(cAdd)
	sDel     = lipgloss.NewStyle().Foreground(cDel)
	sHunk    = lipgloss.NewStyle().Foreground(cAccent).Faint(true)
	sCursor  = lipgloss.NewStyle().Foreground(cAccent)
	sTitle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#0d1117")).Background(cAccent).Padding(0, 1)
	sErr     = lipgloss.NewStyle().Foreground(cDel)
	sBranch  = lipgloss.NewStyle().Foreground(lipgloss.Color("#0d1117")).Background(cBranch).Bold(true).Padding(0, 1)
	sSelName = lipgloss.NewStyle().Reverse(true)
	sGuide   = lipgloss.NewStyle().Foreground(lipgloss.Color("#3b4261"))
	sMerging = lipgloss.NewStyle().Foreground(lipgloss.Color("#0d1117")).Background(cDel).Bold(true).Padding(0, 1)
)

func statusColor(code string) lipgloss.Color {
	switch code {
	case "A", "U":
		return cAdd
	case "D", "C":
		return cDel
	case "R":
		return cRen
	default:
		return cMod
	}
}

// title names the pane; each explorer pane shows one fixed view.
func (m *model) title() string {
	t := "Files"
	if m.view == viewDiff {
		t = "Changes"
		if n := len(m.status); n > 0 {
			t += fmt.Sprintf(" %d", n)
		}
		t += countStaged(m.staged)
		if m.mode == modeShow {
			t = m.showTitle + " (esc back)"
		}
	}
	if m.reading != nil {
		t = m.reading.f.Path
	}
	if m.iconSet == "emoji" && m.reading != nil {
		t = "📄 " + t
	} else if m.iconSet == "emoji" {
		t = map[viewKind]string{viewFiles: "📁 ", viewDiff: "🔀 "}[m.view] + t
	}
	return sTitle.Render(t)
}

// branchBar is its own row so the branch stays readable in a narrow pane.
func (m *model) branchBar() string {
	repo := sDim.Render(" " + filepath.Base(m.root))
	if m.branch == "" {
		if !m.isGit {
			return sDim.Render(" (no git)") + repo
		}
		return repo
	}
	br := m.branch
	if m.iconSet == "emoji" {
		br = "🌿 " + br
	}
	s := sBranch.Render(br)
	if m.hasUpstream && (m.ahead > 0 || m.behind > 0) {
		s += " " + sDim.Render(fmt.Sprintf("↑%d ↓%d", m.ahead, m.behind))
	}
	if m.merging {
		s += " " + sMerging.Render("MERGING")
	}
	return s + repo
}

func (m *model) footer() string {
	if m.mode == modeInput {
		return m.inputLine()
	}
	if m.flash != "" && time.Since(m.flashAt) < 4*time.Second {
		if m.flashOK {
			return sAdd.Render(m.flash)
		}
		return sErr.Render(m.flash)
	}
	if m.err != "" {
		return sErr.Render(m.err)
	}
	if m.reading != nil {
		return m.readerFooter()
	}
	if m.view == viewFiles {
		return sDim.Render("⏎ open  s send  esc chat  ? help")
	}
	if m.mode == modePick {
		return sDim.Render("⏎ choose  esc back")
	}
	if m.mode == modeShow {
		return sDim.Render("⏎ fold  n/p file  esc back to history")
	}
	return sDim.Render("␣ stage  c commit  L history  ? help")
}

func (m *model) View() string {
	if m.width == 0 {
		return ""
	}
	h := m.contentHeight()
	var lines []string
	switch {
	case m.help:
		lines = helpText
	case m.mode == modePick:
		lines = m.pickLines(h)
	case m.reading != nil:
		lines = m.readerLines(h)
	case m.view == viewFiles:
		lines = m.treeLines(h)
	default:
		lines = m.diffLines(h)
	}
	if len(lines) > h {
		lines = lines[:h]
	}
	for len(lines) < h {
		lines = append(lines, "")
	}
	out := make([]string, 0, h+2)
	out = append(out, m.title(), m.branchBar())
	out = append(out, lines...)
	out = append(out, m.footer())
	for i, l := range out {
		out[i] = ansi.Truncate(l, m.width, "…")
	}
	return strings.Join(out, "\n")
}

func cursorMark(sel bool) string {
	if sel {
		return sCursor.Render("▌")
	}
	return " "
}

func (m *model) fileIconStr(name string) string {
	if m.iconSet != "emoji" {
		return ""
	}
	return emojiFile(name) + " "
}

func (m *model) dirIconStr(name string, open bool) string {
	if m.iconSet != "emoji" {
		return ""
	}
	return emojiDir(name, open) + " "
}

// indentGuides draws "│ " for each ancestor level, like an editor sidebar.
func indentGuides(depth int) string {
	if depth <= 0 {
		return ""
	}
	return sGuide.Render(strings.Repeat("│ ", depth))
}

func (m *model) treeLines(h int) []string {
	if len(m.trows) == 0 {
		msg := "(empty folder)"
		if m.tree.err != "" {
			msg = m.tree.err
		}
		return []string{sDim.Render("  " + msg)}
	}
	var out []string
	for i := m.toff; i < len(m.trows) && len(out) < h; i++ {
		n := m.trows[i]
		sel := i == m.tcur
		var b strings.Builder
		b.WriteString(cursorMark(sel))
		b.WriteString(indentGuides(n.depth))
		nameStyle := lipgloss.NewStyle()
		badge := ""
		if n.isDir {
			arrow := "▸ "
			if n.open {
				arrow = "▾ "
			}
			b.WriteString(sDim.Render(arrow))
			b.WriteString(m.dirIconStr(n.name, n.open))
			if m.dirtyDirs[n.rel] {
				nameStyle = nameStyle.Foreground(cMod)
				badge = " " + lipgloss.NewStyle().Foreground(cMod).Render("●")
			}
		} else {
			b.WriteString("  ")
			b.WriteString(m.fileIconStr(n.name))
			if code, ok := m.status[n.rel]; ok {
				c := statusColor(code)
				nameStyle = nameStyle.Foreground(c)
				badge = " " + lipgloss.NewStyle().Foreground(c).Bold(true).Render(code)
			}
		}
		if sel {
			nameStyle = nameStyle.Inherit(sSelName)
		}
		b.WriteString(nameStyle.Render(n.name))
		b.WriteString(badge)
		out = append(out, b.String())
	}
	return out
}

func (m *model) diffLines(h int) []string {
	if !m.isGit {
		return []string{sDim.Render("  not a git repository")}
	}
	if len(m.drows) == 0 {
		if m.loading {
			return []string{sDim.Render("  loading…")}
		}
		return []string{sDim.Render("  no changes — working tree clean")}
	}
	var out []string
	for i := m.doff; i < len(m.drows) && len(out) < h; i++ {
		r := m.drows[i]
		f := m.diffs[r.file]
		sel := i == m.dcur
		switch r.line {
		case -1:
			out = append(out, cursorMark(sel)+m.fileHeader(f, sel))
		case -2:
			msg := f.Note
			if f.Binary {
				msg = "binary file"
			}
			if msg == "" {
				msg = "no content changes (mode/rename only)"
			}
			out = append(out, cursorMark(sel)+"    "+sDim.Render(msg))
		default:
			out = append(out, cursorMark(sel)+m.diffLine(f.Lines[r.line]))
		}
	}
	return out
}

func (m *model) fileHeader(f *git.FileDiff, sel bool) string {
	arrow := "▾ "
	if m.isCollapsed(f) {
		arrow = "▸ "
	}
	c := statusColor(f.Status)
	name := f.Path
	if f.OldPath != "" && f.OldPath != f.Path {
		name = f.OldPath + " → " + f.Path
	}
	ns := lipgloss.NewStyle().Bold(true)
	if sel {
		ns = ns.Inherit(sSelName)
	}
	mark := ""
	if m.mode != modeShow {
		mark = stagedMark(m.staged[f.Path])
	}
	s := sDim.Render(arrow) + mark +
		lipgloss.NewStyle().Foreground(c).Bold(true).Render(f.Status) + " " +
		m.fileIconStr(filepath.Base(f.Path)) +
		ns.Render(name)
	if f.Adds > 0 {
		s += " " + sAdd.Render(fmt.Sprintf("+%d", f.Adds))
	}
	if f.Dels > 0 {
		s += " " + sDel.Render(fmt.Sprintf("-%d", f.Dels))
	}
	return s
}

func lineNo(n int) string {
	if n == 0 {
		return "    "
	}
	return fmt.Sprintf("%4d", n)
}

func (m *model) diffLine(l git.Line) string {
	text := strings.ReplaceAll(l.Text, "\t", "    ")
	gutter := sDim.Render(lineNo(l.OldNo) + " " + lineNo(l.NewNo) + " │")
	switch l.Kind {
	case '@':
		return " " + stagedMark(l.Staged) + sHunk.Render(text)
	case '+':
		return "   " + gutter + sAdd.Render("+"+text)
	case '-':
		return "   " + gutter + sDel.Render("-"+text)
	case '\\':
		return "   " + gutter + sDim.Render(text)
	default:
		return "   " + gutter + " " + text
	}
}
