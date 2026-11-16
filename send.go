package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Sending a file to the chat: terminals can't drag & drop between panes, so
// the explorer types "@path " into the neighbouring pane (Claude Code and most
// coding agents treat @path as a file mention). Enter is not pressed.

type paneInfo struct {
	PaneID      string `json:"pane_id"`
	Label       string `json:"label"`
	Focused     bool   `json:"focused"`
	Cwd         string `json:"cwd"`
	AgentStatus string `json:"agent_status"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
}

// launchContext is the herdr context captured when the pane was opened.
type launchContext struct {
	FocusedPaneID string `json:"focused_pane_id"`
	TabID         string `json:"tab_id"`
	WorkspaceID   string `json:"workspace_id"`
}

func readLaunchContext() launchContext {
	var c launchContext
	_ = json.Unmarshal([]byte(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON")), &c)
	return c
}

func herdrBin() string { return envOr("HERDR_BIN_PATH", "herdr") }

func listPanes() ([]paneInfo, error) {
	out, err := exec.Command(herdrBin(), "pane", "list").Output()
	if err != nil {
		return nil, fmt.Errorf("herdr pane list: %w", err)
	}
	var resp struct {
		Result struct {
			Panes []paneInfo `json:"panes"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("herdr pane list: %w", err)
	}
	return resp.Result.Panes, nil
}

// pickTarget chooses the pane to type into: the pane that was focused when
// the explorer opened, else an agent pane in the same tab/workspace, else any
// other pane in the same tab.
func pickTarget(ctx launchContext) (paneInfo, error) {
	panes, err := listPanes()
	if err != nil {
		return paneInfo{}, err
	}
	self := os.Getenv("HERDR_PANE_ID")
	var others []paneInfo
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
	score := func(p paneInfo) int {
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
		return paneInfo{}, fmt.Errorf("no other pane to send to")
	}
	return others[best], nil
}

// mention formats a file as @path: relative when the target pane sits in the
// repo root, absolute otherwise; quoted when it contains spaces.
func mention(root, rel, targetCwd string) string {
	p := filepath.Join(root, filepath.FromSlash(rel))
	if targetCwd != "" && samePath(targetCwd, root) {
		p = rel
	} else {
		p = filepath.ToSlash(p)
	}
	if strings.ContainsAny(p, " \t") {
		return `@"` + p + `" `
	}
	return "@" + p + " "
}

// sendToChat types "@path " into the target pane and returns a status line.
func sendToChat(ctx launchContext, root, rel string) (string, error) {
	if os.Getenv("HERDR_ENV") == "" && os.Getenv("HERDR_SOCKET_PATH") == "" {
		return "", fmt.Errorf("not running inside herdr")
	}
	t, err := pickTarget(ctx)
	if err != nil {
		return "", err
	}
	text := mention(root, rel, t.Cwd)
	if out, err := exec.Command(herdrBin(), "pane", "send-text", t.PaneID, text).CombinedOutput(); err != nil {
		return "", fmt.Errorf("send-text: %v %s", err, strings.TrimSpace(string(out)))
	}
	return fmt.Sprintf("sent %s→ %s", text, t.PaneID), nil
}
