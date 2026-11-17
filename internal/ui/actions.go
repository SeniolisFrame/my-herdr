package ui

import (
	"fmt"
	"herdr-explorer/internal/git"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Fork-like actions: modes (text input, picker list, commit view), the git
// operations behind the keys, and the help screen.

type uiMode int

const (
	modeNormal uiMode = iota
	modeInput         // footer is a text box (commit message, branch name)
	modePick          // content is a list (branches, log graph, confirm)
	modeShow          // Changes pane shows one commit's diff (from the log)
)

type opMsg struct {
	text string
	err  error
}

// op runs a git action in the background; opMsg reports it and reloads.
func (m *model) op(busy string, f func() (string, error)) tea.Cmd {
	// Future flashAt keeps "busy…" on screen until opMsg replaces it.
	m.flash, m.flashOK, m.flashAt = busy+"…", true, time.Now().Add(time.Hour)
	return func() tea.Msg {
		t, err := f()
		return opMsg{t, err}
	}
}

func (m *model) ask(prompt string, f func(string) tea.Cmd) tea.Cmd {
	m.mode, m.prompt, m.input, m.onInput = modeInput, prompt, "", f
	return nil
}

func (m *model) pick(title string, choices []string, cur int, f func(int) tea.Cmd) tea.Cmd {
	m.mode, m.pickTitle, m.choices, m.onPick = modePick, title, choices, f
	m.ccur, m.coff = cur, 0
	m.clamp()
	return nil
}

func (m *model) inputKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = modeNormal
	case tea.KeyEnter:
		m.mode = modeNormal
		return m.onInput(strings.TrimSpace(m.input))
	case tea.KeyBackspace:
		if r := []rune(m.input); len(r) > 0 {
			m.input = string(r[:len(r)-1])
		}
	case tea.KeySpace:
		m.input += " "
	case tea.KeyRunes:
		m.input += string(msg.Runes)
	}
	return nil
}

func (m *model) pickKey(k string) tea.Cmd {
	if latin, ok := thaiKeys[k]; ok {
		k = latin
	}
	switch k {
	case "up", "k":
		m.ccur--
	case "down", "j":
		m.ccur++
	case "pgup", "ctrl+u":
		m.ccur -= m.contentHeight() - 2
	case "pgdown", "ctrl+d":
		m.ccur += m.contentHeight() - 2
	case "home", "g":
		m.ccur = 0
	case "end", "G":
		m.ccur = len(m.choices) - 1
	case "esc", "q":
		m.mode = modeNormal
		if m.view == viewFiles && m.reading == nil {
			return m.zoomCmd(false) // back from search-all results
		}
	case "enter", " ":
		m.mode = modeNormal
		if m.ccur < len(m.choices) {
			return m.onPick(m.ccur)
		}
	}
	m.clamp()
	return nil
}

// repoKey handles the repo-wide actions available in both panes.
func (m *model) repoKey(k string) (tea.Cmd, bool) {
	root := m.root
	switch k {
	case "?":
		m.help = !m.help
	case "b": // checkout
		local, remote := git.Branches(root)
		var choices []string
		isRemote := map[string]bool{}
		for _, b := range local {
			if b != m.branch {
				choices = append(choices, b)
			}
		}
		for _, r := range remote {
			if !slices.Contains(local, r[strings.IndexByte(r, '/')+1:]) {
				choices = append(choices, r)
				isRemote[r] = true
			}
		}
		return m.pick("Checkout branch", choices, 0, func(i int) tea.Cmd {
			b := choices[i]
			return m.op("checking out "+b, func() (string, error) {
				return "on " + b, git.Switch(root, b, isRemote[b])
			})
		}), true
	case "B": // new branch
		return m.ask("new branch", func(name string) tea.Cmd {
			if name == "" {
				return nil
			}
			return m.op("creating "+name, func() (string, error) {
				return "created " + name, git.Run(root, "switch", "-c", name)
			})
		}), true
	case "m": // merge
		local, remote := git.Branches(root)
		var choices []string
		for _, b := range append(local, remote...) {
			if b != m.branch {
				choices = append(choices, b)
			}
		}
		return m.pick("Merge into "+m.branch, choices, 0, func(i int) tea.Cmd {
			b := choices[i]
			return m.op("merging "+b, func() (string, error) { return "merged " + b, git.Merge(root, b) })
		}), true
	case "X": // abort merge
		if !m.merging {
			return nil, true
		}
		return m.confirm("Abort the merge and drop its changes?", "Abort merge", func() tea.Cmd {
			return m.op("aborting merge", func() (string, error) { return "merge aborted", git.Run(root, "merge", "--abort") })
		}), true
	case "f":
		return m.op("fetching", func() (string, error) { return "fetched", git.Run(root, "fetch", "--all", "--prune") }), true
	case "F":
		return m.op("pulling", func() (string, error) { return "pulled", git.Pull(root) }), true
	case "P":
		return m.op("pushing", func() (string, error) { return "pushed", git.Push(root) }), true
	case "z":
		return m.op("stashing", func() (string, error) { return "stashed", git.Run(root, "stash", "push", "-u") }), true
	case "Z":
		return m.op("popping stash", func() (string, error) { return "stash popped", git.StashPop(root) }), true
	default:
		return nil, false
	}
	return nil, true
}

// changesKey handles the Changes-pane actions on the file under the cursor.
func (m *model) changesKey(k string) (tea.Cmd, bool) {
	root := m.root
	var f *git.FileDiff
	if m.dcur < len(m.drows) {
		f = m.diffs[m.drows[m.dcur].file]
	}
	paths := func() []string {
		if f.OldPath != "" {
			return []string{f.Path, f.OldPath}
		}
		return []string{f.Path}
	}
	switch k {
	case " ":
		if f == nil {
			return nil, true
		}
		if line := m.drows[m.dcur].line; line >= 0 && f.Status == "M" {
			n := -1 // which hunk the cursor is in
			for _, l := range f.Lines[:line+1] {
				if l.Kind == '@' {
					n++
				}
			}
			path := f.Path
			return m.op("staging hunk", func() (string, error) { return git.ToggleHunk(root, path, n) }), true
		}
		stage := m.staged[f.Path] != "full"
		verb := map[bool]string{true: "staged ", false: "unstaged "}[stage]
		return m.op("staging", func() (string, error) { return verb + f.Path, git.SetStaged(root, stage, paths()...) }), true
	case "S":
		return m.op("staging all", func() (string, error) { return "staged all", git.SetStaged(root, true, ".") }), true
	case "U":
		return m.op("unstaging all", func() (string, error) { return "unstaged all", git.SetStaged(root, false, ".") }), true
	case "c":
		if len(m.staged) == 0 && !m.merging {
			m.flash, m.flashOK, m.flashAt = "nothing staged: space stages a file, S stages all", false, time.Now()
			return nil, true
		}
		prompt := "commit message"
		if m.merging {
			prompt = "merge commit message (empty = default)"
		}
		return m.ask(prompt, func(msg string) tea.Cmd {
			if msg == "" && !m.merging {
				return nil
			}
			return m.op("committing", func() (string, error) {
				h, err := git.Commit(root, msg)
				return "committed " + h, err
			})
		}), true
	case "O", "T":
		if f == nil || f.Status != "C" {
			return nil, true
		}
		side := map[string]string{"O": "ours", "T": "theirs"}[k]
		return m.op("resolving", func() (string, error) { return f.Path + ": took " + side, git.Resolve(root, f.Path, side) }), true
	case "D":
		if f == nil {
			return nil, true
		}
		return m.confirm("Discard all changes in "+f.Path+"?", "Discard", func() tea.Cmd {
			return m.op("discarding", func() (string, error) { return "discarded " + f.Path, git.Discard(root, f) })
		}), true
	case "L":
		return m.openLog(0), true
	}
	return nil, false
}

// confirm asks before destructive actions; "Cancel" is the default choice.
func (m *model) confirm(title, yes string, f func() tea.Cmd) tea.Cmd {
	return m.pick(title, []string{"Cancel", yes}, 0, func(i int) tea.Cmd {
		if i == 1 {
			return f()
		}
		return nil
	})
}

func (m *model) openLog(cur int) tea.Cmd {
	lines := git.Log(m.root)
	return m.pick("History (all branches)", lines, cur, func(i int) tea.Cmd {
		hash := git.LogHash(lines[i])
		if hash == "" {
			return m.openLog(i) // a pure graph line: stay in the log
		}
		fs, err := git.Show(m.root, hash)
		if err != nil {
			m.flash, m.flashOK, m.flashAt = err.Error(), false, time.Now()
			return m.openLog(i)
		}
		m.mode, m.showTitle, m.logCur = modeShow, "commit "+hash, i
		m.diffs, m.dcur, m.doff = fs, 0, 0
		m.rebuildDiffRows()
		return nil
	})
}

var helpText = []string{
	"Files pane",
	"  enter   open folder / read file (pane zooms, esc closes)",
	"  /       search all files; in a file: / search, n / N next / prev",
	"  in a file: b blame (who changed each line), L file history",
	"  in a file: e edit here ($EDITOR, else nano / vim)",
	"Changes pane",
	"  space   stage / unstage file (on its header) or hunk (inside it)",
	"  S / U   stage / unstage all",
	"  c       commit (message box)      D      discard file changes",
	"  enter   fold file   a fold all    n / p  next / previous file",
	"  O / T   conflict: take ours / theirs (or edit + space)",
	"  L       history graph, enter = show commit, esc = back",
	"Both panes",
	"  b checkout branch   B new branch   m merge   X abort merge",
	"  f fetch   F pull   P push   z stash   Z pop stash",
	"  s send file to chat   esc chat   alt+e next pane   q quit",
	"",
	"any key closes this help",
}

func (m *model) pickLines(h int) []string {
	out := []string{sTitle.Render(m.pickTitle)}
	if len(m.choices) == 0 {
		return append(out, sDim.Render("  (nothing)"))
	}
	for i := m.coff; i < len(m.choices) && len(out) < h; i++ {
		out = append(out, cursorMark(i == m.ccur)+" "+m.choices[i])
	}
	return out
}

func (m *model) inputLine() string {
	return sBranch.Render(m.prompt) + " " + m.input + sCursor.Render("█")
}

func stagedMark(s string) string {
	switch s {
	case "full":
		return sAdd.Render("✓") + " "
	case "partial":
		return lipgloss.NewStyle().Foreground(cMod).Render("~") + " "
	}
	return "  "
}

func countStaged(staged map[string]string) string {
	if len(staged) == 0 {
		return ""
	}
	return fmt.Sprintf(" · %d staged", len(staged))
}
