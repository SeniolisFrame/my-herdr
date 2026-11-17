package main

import (
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// File reader in the Files pane: enter on a file shows it read-only, with the
// pane zoomed so it is wide enough to read; esc closes it.

func (m *model) openReader(rel string) tea.Cmd {
	m.reading, m.roff = untrackedDiff(m.root, rel), 0
	return zoomCmd(true)
}

func (m *model) readerKey(k string) tea.Cmd {
	h := m.contentHeight()
	switch k {
	case "esc", "q", "backspace", "enter", "left", "h":
		m.reading = nil
		return zoomCmd(false)
	case "s":
		return m.sendCmd(m.reading.path)
	case "up", "k":
		m.roff--
	case "down", "j":
		m.roff++
	case "pgup", "ctrl+u":
		m.roff -= h
	case "pgdown", "ctrl+d", " ":
		m.roff += h
	case "home", "g":
		m.roff = 0
	case "end", "G":
		m.roff = len(m.reading.lines)
	}
	m.roff = min(max(m.roff, 0), max(len(m.readerText())-h, 0))
	return nil
}

// readerText is the file's lines (untrackedDiff starts them with a '@' header).
func (m *model) readerText() []diffLine {
	if l := m.reading.lines; len(l) > 0 && l[0].kind == '@' {
		return l[1:]
	}
	return m.reading.lines
}

func (m *model) readerLines(h int) []string {
	f := m.reading
	switch {
	case f.binary:
		return []string{sDim.Render(" binary file")}
	case f.note != "" && len(f.lines) == 0:
		return []string{sDim.Render(" " + f.note)}
	}
	var out []string
	for _, l := range m.readerText()[m.roff:] {
		if len(out) == h {
			break
		}
		out = append(out, sDim.Render(lineNo(l.newNo)+" │ ")+strings.ReplaceAll(l.text, "\t", "    "))
	}
	return out
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
