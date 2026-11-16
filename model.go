package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
	root    string
	isGit   bool
	iconSet string // emoji | none
	view    viewKind

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

	diffs     []*fileDiff
	collapsed map[string]bool
	drows     []drow
	dcur      int
	doff      int

	loading bool
	err     string

	ctx     launchContext
	flash   string
	flashAt time.Time
	flashOK bool
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
	diffs         []*fileDiff
	err           error
	withDiff      bool
}

type tickMsg struct{}

func newModel(root string, startDiff bool, iconSet string) *model {
	m := &model{
		root:      root,
		isGit:     gitTopLevel(root) != "",
		iconSet:   iconSet,
		collapsed: map[string]bool{},
		status:    map[string]string{},
		dirtyDirs: map[string]bool{},
		ctx:       readLaunchContext(),
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
	root, isGit := m.root, m.isGit
	return func() tea.Msg {
		if !isGit {
			return dataMsg{status: map[string]string{}}
		}
		d := dataMsg{branch: gitBranch(root), status: gitStatus(root), withDiff: withDiff}
		d.ahead, d.behind, d.hasUpstream = gitAheadBehind(root)
		if withDiff {
			d.diffs, d.err = loadDiffs(root)
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
		m.applyData(msg)
	case flashMsg:
		m.flash, m.flashOK, m.flashAt = msg.text, msg.ok, time.Now()
	case tea.KeyMsg:
		return m, m.handleKey(msg.String())
	case tea.MouseMsg:
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
		return m.diffs[m.drows[m.dcur].file].path
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
		s, err := sendToChat(ctx, root, rel)
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
		curPath, curLine = m.diffs[r.file].path, r.line
	}
	if !d.withDiff {
		return
	}
	if d.err == nil {
		m.diffs = d.diffs
	}
	m.rebuildDiffRows()
	if curPath != "" {
		header := -1
		for i, r := range m.drows {
			if m.diffs[r.file].path != curPath {
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

func (m *model) isCollapsed(f *fileDiff) bool {
	if v, ok := m.collapsed[f.path]; ok {
		return v
	}
	return len(f.lines) > autoCollapseLines
}

func (m *model) rebuildDiffRows() {
	m.drows = m.drows[:0]
	for i, f := range m.diffs {
		m.drows = append(m.drows, drow{file: i, line: -1})
		if m.isCollapsed(f) {
			continue
		}
		if f.binary || len(f.lines) == 0 {
			m.drows = append(m.drows, drow{file: i, line: -2})
			continue
		}
		for j := range f.lines {
			m.drows = append(m.drows, drow{file: i, line: j})
		}
	}
}

func (m *model) clamp() {
	h := m.contentHeight()
	m.tcur, m.toff = clampView(m.tcur, m.toff, len(m.trows), h)
	m.dcur, m.doff = clampView(m.dcur, m.doff, len(m.drows), h)
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
	half := max(1, m.contentHeight()/2)
	switch k {
	case "ctrl+c", "q":
		return tea.Quit
	case "tab":
		return m.focusCmd(focusNext)
	case "s":
		return m.sendCmd(m.currentRel())
	case "esc", "c":
		return m.focusCmd(focusChat)
	case "r":
		m.refreshTree()
		if !m.loading {
			m.loading = true
			return m.loadCmd(m.view == viewDiff)
		}
		return nil
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
		case "home", "g":
			m.tcur = 0
		case "end", "G":
			m.tcur = len(m.trows) - 1
		case "enter", " ":
			if cur != nil && cur.isDir {
				m.toggleDir(cur)
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
	case "enter", " ", "o":
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
			m.collapsed[f.path] = anyOpen
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
	m.collapsed[f.path] = !m.isCollapsed(f)
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
)

func statusColor(code string) lipgloss.Color {
	switch code {
	case "A", "U":
		return cAdd
	case "D":
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
	}
	if m.iconSet == "emoji" {
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
	return s + repo
}

func (m *model) footer() string {
	if m.flash != "" && time.Since(m.flashAt) < 4*time.Second {
		if m.flashOK {
			return sAdd.Render(m.flash)
		}
		return sErr.Render(m.flash)
	}
	if m.err != "" {
		return sErr.Render(m.err)
	}
	if m.view == viewFiles {
		return sDim.Render("⏎ open  s send  tab next pane  esc chat  q quit")
	}
	return sDim.Render("⏎ fold  a fold all  n/p file  s send  tab next  esc chat")
}

func (m *model) View() string {
	if m.width == 0 {
		return ""
	}
	h := m.contentHeight()
	var lines []string
	if m.view == viewFiles {
		lines = m.treeLines(h)
	} else {
		lines = m.diffLines(h)
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
			msg := f.note
			if f.binary {
				msg = "binary file"
			}
			if msg == "" {
				msg = "no content changes (mode/rename only)"
			}
			out = append(out, cursorMark(sel)+"    "+sDim.Render(msg))
		default:
			out = append(out, cursorMark(sel)+m.diffLine(f.lines[r.line]))
		}
	}
	return out
}

func (m *model) fileHeader(f *fileDiff, sel bool) string {
	arrow := "▾ "
	if m.isCollapsed(f) {
		arrow = "▸ "
	}
	c := statusColor(f.status)
	name := f.path
	if f.oldPath != "" && f.oldPath != f.path {
		name = f.oldPath + " → " + f.path
	}
	ns := lipgloss.NewStyle().Bold(true)
	if sel {
		ns = ns.Inherit(sSelName)
	}
	s := sDim.Render(arrow) +
		lipgloss.NewStyle().Foreground(c).Bold(true).Render(f.status) + " " +
		m.fileIconStr(filepath.Base(f.path)) +
		ns.Render(name)
	if f.adds > 0 {
		s += " " + sAdd.Render(fmt.Sprintf("+%d", f.adds))
	}
	if f.dels > 0 {
		s += " " + sDel.Render(fmt.Sprintf("-%d", f.dels))
	}
	return s
}

func lineNo(n int) string {
	if n == 0 {
		return "    "
	}
	return fmt.Sprintf("%4d", n)
}

func (m *model) diffLine(l diffLine) string {
	text := strings.ReplaceAll(l.text, "\t", "    ")
	gutter := sDim.Render(lineNo(l.oldNo) + " " + lineNo(l.newNo) + " │")
	switch l.kind {
	case '@':
		return "   " + sHunk.Render(text)
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
