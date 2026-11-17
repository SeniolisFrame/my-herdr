// Package herdr talks to the herdr CLI: panes, layout, focus, sending text.
package herdr

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
)

// Sending a file to the chat: terminals can't drag & drop between panes, so
// the explorer types "@path " into the neighbouring pane (Claude Code and most
// coding agents treat @path as a file mention). Enter is not pressed.

type Pane struct {
	PaneID      string `json:"pane_id"`
	Label       string `json:"label"`
	Focused     bool   `json:"focused"`
	Cwd         string `json:"cwd"`
	AgentStatus string `json:"agent_status"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
}

// Context is the herdr context captured when the pane was opened.
type Context struct {
	FocusedPaneID string `json:"focused_pane_id"`
	TabID         string `json:"tab_id"`
	WorkspaceID   string `json:"workspace_id"`
}

func ReadContext() Context {
	var c Context
	_ = json.Unmarshal([]byte(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON")), &c)
	return c
}

func Bin() string { return envOr("HERDR_BIN_PATH", "herdr") }

func ListPanes() ([]Pane, error) {
	out, err := exec.Command(Bin(), "pane", "list").Output()
	if err != nil {
		return nil, fmt.Errorf("herdr pane list: %w", err)
	}
	var resp struct {
		Result struct {
			Panes []Pane `json:"panes"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("herdr pane list: %w", err)
	}
	return resp.Result.Panes, nil
}
