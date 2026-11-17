package herdr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// pickTarget chooses the pane to type into: the pane that was focused when
// the explorer opened, else an agent pane in the same tab/workspace, else any
// other pane in the same tab.
func pickTarget(ctx Context) (Pane, error) {
	panes, err := ListPanes()
	if err != nil {
		return Pane{}, err
	}
	self := os.Getenv("HERDR_PANE_ID")
	var others []Pane
	for _, p := range panes {
		if p.PaneID != self {
			others = append(others, p)
		}
	}
	// Auto-opened panes are told their chat pane explicitly.
	for _, want := range []string{os.Getenv("HERDR_EXPLORER_TARGET"), ctx.FocusedPaneID} {
		for _, p := range others {
			if want != "" && p.PaneID == want {
				return p, nil
			}
		}
	}
	score := func(p Pane) int {
		s := 0
		if p.AgentStatus != "" && p.AgentStatus != "unknown" {
			s += 4
		}
		if ctx.TabID != "" && p.TabID == ctx.TabID {
			s += 2
		}
		if ctx.WorkspaceID != "" && p.WorkspaceID == ctx.WorkspaceID {
			s++
		}
		return s
	}
	best, bestScore := -1, -1
	for i, p := range others {
		if ctx.WorkspaceID != "" && p.WorkspaceID != ctx.WorkspaceID {
			continue
		}
		if s := score(p); s > bestScore {
			best, bestScore = i, s
		}
	}
	if best < 0 {
		return Pane{}, fmt.Errorf("no other pane to send to")
	}
	return others[best], nil
}

// Mention formats a file as @path: relative when the target pane sits in the
// repo root, absolute otherwise; quoted when it contains spaces.
func Mention(root, rel, targetCwd string) string {
	p := filepath.Join(root, filepath.FromSlash(rel))
	if targetCwd != "" && SamePath(targetCwd, root) {
		p = rel
	} else {
		p = filepath.ToSlash(p)
	}
	if strings.ContainsAny(p, " \t") {
		return `@"` + p + `" `
	}
	return "@" + p + " "
}

// SendToChat types "@path " into the target pane and returns a status line.
func SendToChat(ctx Context, root, rel string) (string, error) {
	// A newline in a file name would act as Enter in the target pane.
	if strings.IndexFunc(rel, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("file name contains control characters, not sending")
	}
	if os.Getenv("HERDR_ENV") == "" && os.Getenv("HERDR_SOCKET_PATH") == "" {
		return "", fmt.Errorf("not running inside herdr")
	}
	t, err := pickTarget(ctx)
	if err != nil {
		return "", err
	}
	text := Mention(root, rel, t.Cwd)
	if out, err := exec.Command(Bin(), "pane", "send-text", t.PaneID, text).CombinedOutput(); err != nil {
		return "", fmt.Errorf("send-text: %v %s", err, strings.TrimSpace(string(out)))
	}
	return fmt.Sprintf("sent %s→ %s", text, t.PaneID), nil
}
