// Package ui is the explorer pane: file tree, changes, reader.
package ui

import (
	"herdr-explorer/internal/git"
	"herdr-explorer/internal/herdr"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type viewKind int

const (
	viewFiles viewKind = iota
	viewDiff
)

const refreshEvery = 2 * time.Second

// headerRows: title + branch bar above the content.
const headerRows = 2

// autoCollapseLines: files with bigger diffs (lockfiles...) start collapsed.
const autoCollapseLines = 400

type drow struct {
	file int
	line int // -1 file header, -2 info row (binary/empty), >=0 diff line
}

type model struct {
	watching  bool    // polling focus while zoomed (panes.go)
	zoomedOut bool    // zoomed view left via alt+e, so unzoomed for now
	chatCwd   string  // chat pane's folder at the last poll (follow.go)
	reading   *reader // Files pane: file open in the reader (reader.go)
	root      string
	isGit     bool
	iconSet   string // emoji | none
	view      viewKind

	width, height int

	tree  *node
	trows []*node
	tcur  int
	toff  int

	branch        string
	ahead, behind int
	hasUpstream   bool
	status        map[string]string
	dirtyDirs     map[string]bool

	diffs     []*git.FileDiff
	collapsed map[string]bool
	drows     []drow
	dcur      int
	doff      int

	loading bool
	err     string

	ctx     herdr.Context
	flash   string
	flashAt time.Time
	flashOK bool

	staged  map[string]string // path -> full | partial
	merging bool

	mode      uiMode
	prompt    string // modeInput
	input     string
	onInput   func(string) tea.Cmd
	pickTitle string // modePick
	choices   []string
	ccur      int
	coff      int
	onPick    func(int) tea.Cmd
	showTitle string // modeShow
	logCur    int
	help      bool
}

type flashMsg struct {
	text string
	ok   bool
}

type dataMsg struct {
	branch        string
	ahead, behind int
	hasUpstream   bool
	status        map[string]string
	staged        map[string]string
	merging       bool
	diffs         []*git.FileDiff
	err           error
	withDiff      bool
	cwd, newRoot  string // chat pane's folder; set newRoot when that is another repo/folder
}

type tickMsg struct{}

// New is the explorer pane for root; startDiff opens the Changes view.
func New(root string, startDiff bool, iconSet string) tea.Model {
	return newModel(root, startDiff, iconSet)
}

func newModel(root string, startDiff bool, iconSet string) *model {
	m := &model{
		root:      root,
		isGit:     git.TopLevel(root) != "",
		iconSet:   iconSet,
		collapsed: map[string]bool{},
		status:    map[string]string{},
		staged:    map[string]string{},
		dirtyDirs: map[string]bool{},
		ctx:       herdr.ReadContext(),
	}
	if startDiff {
		m.view = viewDiff
	}
	m.tree = &node{name: filepath.Base(root), path: root, isDir: true, open: true, depth: -1}
	loadDir(m.tree)
	m.trows = flatten(m.tree, nil)
	return m
}

func (m *model) Init() tea.Cmd {
	m.loading = true
	return tea.Batch(m.loadCmd(m.view == viewDiff), tick())
}

func tick() tea.Cmd {
	return tea.Tick(refreshEvery, func(time.Time) tea.Msg { return tickMsg{} })
}

// loadCmd always reads branch + status (cheap); the full diff only when
// withDiff, because every tab runs its own explorer on a 2s timer.
func (m *model) loadCmd(withDiff bool) tea.Cmd {
	root, isGit, last := m.root, m.isGit, m.chatCwd
	return func() tea.Msg {
		cwd := herdr.ChatCwd()
		if cwd != "" && cwd != last { // only a moved chat pane costs a git call
			if r := rootOf(cwd); !herdr.SamePath(r, root) {
				return dataMsg{cwd: cwd, newRoot: r}
			}
		}
		if !isGit {
			return dataMsg{status: map[string]string{}, cwd: cwd}
		}
		d := dataMsg{branch: git.Branch(root), merging: git.Merging(root), withDiff: withDiff, cwd: cwd}
		d.status, d.staged = git.ReadStatus(root)
		d.ahead, d.behind, d.hasUpstream = git.AheadBehind(root)
		if withDiff {
			d.diffs, d.err = git.LoadDiffs(root)
		}
		return d
	}
}

func (m *model) contentHeight() int {
	return max(1, m.height-headerRows-1)
}

// ---------- update ----------

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case focusMsg:
		return m, m.onFocus(msg)
	case editedMsg:
		return m, m.afterEdit(msg)
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.clamp()
	case tickMsg:
		m.refreshTree()
		cmds := []tea.Cmd{tick()}
		if !m.loading {
			m.loading = true
			cmds = append(cmds, m.loadCmd(m.view == viewDiff))
		}
		return m, tea.Batch(cmds...)
	case dataMsg:
		m.loading = false
		if msg.newRoot != "" {
			// Switch only while idle; otherwise the next poll asks again.
			if m.mode == modeNormal && m.reading == nil && !m.help {
				m.switchRoot(msg.newRoot)
				m.chatCwd = msg.cwd
				m.loading = true
				return m, m.loadCmd(m.view == viewDiff)
			}
			return m, nil
		}
		if msg.cwd != "" {
			m.chatCwd = msg.cwd
		}
		m.applyData(msg)
	case flashMsg:
		m.flash, m.flashOK, m.flashAt = msg.text, msg.ok, time.Now()
	case opMsg:
		m.flash, m.flashOK, m.flashAt = msg.text, msg.err == nil, time.Now()
		if msg.err != nil {
			m.flash = msg.err.Error()
		}
		m.refreshTree()
		m.loading = true
		return m, m.loadCmd(m.view == viewDiff && m.mode != modeShow)
	case tea.KeyMsg:
		switch m.mode {
		case modeInput:
			return m, m.inputKey(msg)
		case modePick:
			return m, m.pickKey(msg.String())
		}
		return m, m.handleKey(msg.String())
	case tea.MouseMsg:
		if m.mode == modeInput || m.mode == modePick {
			return m, nil
		}
		return m, m.handleMouse(msg)
	}
	return m, nil
}

// currentRel is the repo-relative path under the cursor in either view.
func (m *model) currentRel() string {
	if m.view == viewFiles {
		if m.tcur < len(m.trows) {
			return m.trows[m.tcur].rel
		}
		return ""
	}
	if m.dcur < len(m.drows) {
		return m.diffs[m.drows[m.dcur].file].Path
	}
	return ""
}

// sendCmd types "@path " into the chat pane next to the explorer.
func (m *model) sendCmd(rel string) tea.Cmd {
	if rel == "" {
		return nil
	}
	ctx, root := m.ctx, m.root
	return func() tea.Msg {
		s, err := herdr.SendToChat(ctx, root, rel)
		if err != nil {
			return flashMsg{text: "send failed: " + err.Error()}
		}
		return flashMsg{text: s, ok: true}
	}
}

func (m *model) refreshTree() {
	var sel *node
	if m.tcur < len(m.trows) {
		sel = m.trows[m.tcur]
	}
	loadDir(m.tree)
	m.trows = flatten(m.tree, nil)
	for i, n := range m.trows {
		if n == sel {
			m.tcur = i
			break
		}
	}
	m.clamp()
}

func (m *model) applyData(d dataMsg) {
	m.err = ""
	if d.err != nil {
		m.err = d.err.Error()
	}
	m.branch, m.ahead, m.behind, m.hasUpstream = d.branch, d.ahead, d.behind, d.hasUpstream
	m.merging = d.merging
	if d.staged != nil {
		m.staged = d.staged
	}
	m.status = d.status
	m.dirtyDirs = map[string]bool{}
	for p := range d.status {
		for i := strings.LastIndexByte(p, '/'); i > 0; i = strings.LastIndexByte(p[:i], '/') {
			m.dirtyDirs[p[:i]] = true
		}
	}

	// Keep the cursor on the same file/line across refreshes.
	curPath, curLine := "", -1
	if m.dcur < len(m.drows) {
		r := m.drows[m.dcur]
		curPath, curLine = m.diffs[r.file].Path, r.line
	}
	if !d.withDiff || m.mode == modeShow {
		return
	}
	if d.err == nil {
		m.diffs = d.diffs
		for _, f := range m.diffs {
			if m.status[f.Path] == "C" {
				f.Status = "C" // git diff shows conflicts as plain modifications
			}
		}
	}
	m.rebuildDiffRows()
	if curPath != "" {
		header := -1
		for i, r := range m.drows {
			if m.diffs[r.file].Path != curPath {
				continue
			}
			if r.line == -1 {
				header = i
			}
			if r.line == curLine {
				m.dcur = i
				header = -2
				break
			}
		}
		if header >= 0 {
			m.dcur = header
		}
	}
	m.clamp()
}

func (m *model) isCollapsed(f *git.FileDiff) bool {
	if v, ok := m.collapsed[f.Path]; ok {
		return v
	}
	return len(f.Lines) > autoCollapseLines
}

func (m *model) rebuildDiffRows() {
	m.drows = m.drows[:0]
	for i, f := range m.diffs {
		m.drows = append(m.drows, drow{file: i, line: -1})
		if m.isCollapsed(f) {
			continue
		}
		if f.Binary || len(f.Lines) == 0 {
			m.drows = append(m.drows, drow{file: i, line: -2})
			continue
		}
		for j := range f.Lines {
			m.drows = append(m.drows, drow{file: i, line: j})
		}
	}
}

func (m *model) clamp() {
	h := m.contentHeight()
	m.tcur, m.toff = clampView(m.tcur, m.toff, len(m.trows), h)
	m.dcur, m.doff = clampView(m.dcur, m.doff, len(m.drows), h)
	m.ccur, m.coff = clampView(m.ccur, m.coff, len(m.choices), h-1) // title row
}

func clampView(cur, off, n, h int) (int, int) {
	cur = min(max(cur, 0), max(n-1, 0))
	if cur < off {
		off = cur
	}
	if cur >= off+h {
		off = cur - h + 1
	}
	off = min(max(off, 0), max(n-h, 0))
	return cur, off
}

func (m *model) handleKey(k string) tea.Cmd {
	if latin, ok := thaiKeys[k]; ok {
		k = latin
	}
	half := max(1, m.contentHeight()/2)
	if m.help {
		m.help = false
		return nil
	}
	if m.reading != nil {
		return m.readerKey(k)
	}
	if m.mode == modeShow {
		if k == "esc" || k == "q" || k == "backspace" {
			m.mode, m.diffs = modeNormal, nil
			m.rebuildDiffRows()
			m.loading = true
			return tea.Batch(m.loadCmd(true), m.openLog(m.logCur))
		}
		if !strings.Contains(" up down k j pgup pgdown ctrl+u ctrl+d home end g G enter o n p [ ] a ", " "+k+" ") {
			return nil // read-only: no staging inside a past commit
		}
	} else {
		if cmd, ok := m.repoKey(k); ok {
			return cmd
		}
		if m.view == viewDiff {
			if cmd, ok := m.changesKey(k); ok {
				return cmd
			}
		}
	}
	switch k {
	case "ctrl+c", "q":
		return tea.Quit
	case "s":
		return m.sendCmd(m.currentRel())
	case "esc":
		return m.focusCmd(focusChat)
	}

	if m.view == viewFiles {
		var cur *node
		if m.tcur < len(m.trows) {
			cur = m.trows[m.tcur]
		}
		switch k {
		case "up", "k":
			m.tcur--
		case "down", "j":
			m.tcur++
		case "pgup", "ctrl+u":
			m.tcur -= half
		case "pgdown", "ctrl+d":
			m.tcur += half
		case "/":
			return m.searchAll()
		case "home", "g":
			m.tcur = 0
		case "end", "G":
			m.tcur = len(m.trows) - 1
		case "enter", " ":
			if cur != nil && cur.isDir {
				m.toggleDir(cur)
			} else if cur != nil {
				return m.openReader(cur.rel)
			}
		case "right", "l":
			if cur != nil && cur.isDir && !cur.open {
				m.toggleDir(cur)
			}
		case "left", "h":
			if cur != nil && cur.isDir && cur.open {
				m.toggleDir(cur)
			} else if cur != nil && cur.parent != nil && cur.parent != m.tree {
				for i, n := range m.trows {
					if n == cur.parent {
						m.tcur = i
					}
				}
			}
		}
		m.clamp()
		return nil
	}

	switch k {
	case "up", "k":
		m.dcur--
	case "down", "j":
		m.dcur++
	case "pgup", "ctrl+u":
		m.dcur -= half
	case "pgdown", "ctrl+d":
		m.dcur += half
	case "home", "g":
		m.dcur = 0
	case "end", "G":
		m.dcur = len(m.drows) - 1
	case "enter", "o":
		if m.dcur < len(m.drows) {
			m.toggleFile(m.drows[m.dcur].file)
		}
	case "n", "]":
		for i := m.dcur + 1; i < len(m.drows); i++ {
			if m.drows[i].line == -1 {
				m.dcur, m.doff = i, i
				break
			}
		}
	case "p", "[":
		for i := m.dcur - 1; i >= 0; i-- {
			if m.drows[i].line == -1 {
				m.dcur = i
				m.doff = min(m.doff, i)
				break
			}
		}
	case "a":
		anyOpen := false
		for _, f := range m.diffs {
			if !m.isCollapsed(f) {
				anyOpen = true
			}
		}
		file := -1
		if m.dcur < len(m.drows) {
			file = m.drows[m.dcur].file
		}
		for _, f := range m.diffs {
			m.collapsed[f.Path] = anyOpen
		}
		m.rebuildDiffRows()
		m.dcur = m.headerRow(file)
	}
	m.clamp()
	return nil
}

func (m *model) toggleDir(n *node) {
	n.open = !n.open
	if n.open {
		loadDir(n)
	}
	m.trows = flatten(m.tree, nil)
}

func (m *model) toggleFile(fi int) {
	f := m.diffs[fi]
	m.collapsed[f.Path] = !m.isCollapsed(f)
	m.rebuildDiffRows()
	m.dcur = m.headerRow(fi)
	m.clamp()
}

func (m *model) headerRow(fi int) int {
	for i, r := range m.drows {
		if r.file == fi && r.line == -1 {
			return i
		}
	}
	return 0
}

// focusCmd runs a pane-focus move in the background; errors go to the footer.
func (m *model) focusCmd(f func(viewKind) error) tea.Cmd {
	v := m.view
	return func() tea.Msg {
		if err := f(v); err != nil {
			return flashMsg{text: "focus: " + err.Error()}
		}
		return nil
	}
}

func (m *model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if msg.Action != tea.MouseActionPress {
		return nil
	}
	switch msg.Button {
	case tea.MouseButtonRight:
		// Right-click a row: send that file to the chat pane.
		row := msg.Y - headerRows
		if row < 0 || row >= m.contentHeight() {
			return nil
		}
		if m.view == viewFiles {
			if i := m.toff + row; i < len(m.trows) {
				m.tcur = i
			}
		} else if i := m.doff + row; i < len(m.drows) {
			m.dcur = i
		}
		m.clamp()
		return m.sendCmd(m.currentRel())
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		d := 3
		if msg.Button == tea.MouseButtonWheelUp {
			d = -3
		}
		if m.view == viewFiles {
			m.tcur += d
		} else {
			m.dcur += d
		}
		m.clamp()
	case tea.MouseButtonLeft:
		row := msg.Y - headerRows
		if row < 0 || row >= m.contentHeight() {
			return nil
		}
		if m.view == viewFiles {
			if i := m.toff + row; i < len(m.trows) {
				m.tcur = i
				if m.trows[i].isDir {
					m.toggleDir(m.trows[i])
				}
			}
		} else if i := m.doff + row; i < len(m.drows) {
			m.dcur = i
			if m.drows[i].line == -1 {
				m.toggleFile(m.drows[i].file)
			}
		}
		m.clamp()
	}
	return nil
}
