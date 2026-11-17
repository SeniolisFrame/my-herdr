package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// e in the reader runs a terminal editor in this pane on the open file, at
// the line on top of the screen; quitting the editor reopens the reader.

type editedMsg struct {
	rel  string
	line int
	err  error
}

// editor is $VISUAL / $EDITOR, else nano or vim on PATH or from Git for
// Windows (git.exe sits in Git\cmd or Git\mingw64\bin, editors in Git\usr\bin).
func editor() []string {
	for _, v := range []string{os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if f := strings.Fields(v); len(f) > 0 {
			return f
		}
	}
	var dirs []string
	if g, err := exec.LookPath("git"); err == nil {
		d := filepath.Dir(g)
		dirs = append(dirs, filepath.Join(d, "..", "usr", "bin"), filepath.Join(d, "..", "..", "usr", "bin"))
	}
	for _, name := range []string{"nano", "vim"} {
		if p, err := exec.LookPath(name); err == nil {
			return []string{p}
		}
		for _, d := range dirs {
			if p, err := exec.LookPath(filepath.Join(d, name)); err == nil {
				return []string{p}
			}
		}
	}
	return nil
}

func (m *model) editCmd() tea.Cmd {
	r := m.reading
	ed := editor()
	if ed == nil {
		m.flash, m.flashOK, m.flashAt = "no editor: set EDITOR", false, time.Now()
		return nil
	}
	line := 1
	if r.off < len(r.rows) {
		line = r.rows[r.off].src + 1
	}
	// nano, vim, micro, hx and most terminal editors take +LINE before the file.
	args := append(ed[1:], "+"+strconv.Itoa(line), filepath.Join(m.root, r.rel))
	c := exec.Command(ed[0], args...)
	c.Dir = m.root
	rel := r.rel
	return tea.ExecProcess(c, func(err error) tea.Msg { return editedMsg{rel, line, err} })
}

// afterEdit reloads the file at the same line, keeping blame and search.
func (m *model) afterEdit(e editedMsg) tea.Cmd {
	old := m.reading
	if old == nil || old.rel != e.rel {
		return nil
	}
	m.openReader(e.rel) // already zoomed: its zoom command is not needed
	r := m.reading
	if old.blame != nil {
		m.toggleBlame()
	}
	r.search(old.query)
	r.layout(m.width)
	for i, row := range r.rows { // same line on top as before
		if row.src == e.line-1 && row.first {
			r.off = min(i, max(len(r.rows)-m.contentHeight(), 0))
			break
		}
	}
	if e.err != nil {
		m.flash, m.flashOK, m.flashAt = "editor: "+e.err.Error(), false, time.Now()
	}
	return m.loadCmd(false)
}
