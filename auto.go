package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// autoOpen runs from the [[startup]] and [[events]] hooks: it makes sure the
// Files (left) and Changes (right) panes are open around the chat.
// Create the file "no-auto" in the plugin config dir to turn this off.
func autoOpen() int {
	if dir := os.Getenv("HERDR_PLUGIN_CONFIG_DIR"); dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "no-auto")); err == nil {
			return 0
		}
	}
	event := os.Getenv("HERDR_PLUGIN_EVENT")
	if event == "startup" {
		wss, err := listWorkspaces()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		for _, w := range wss {
			ensurePanes(w.WorkspaceID, w.ActiveTabID)
		}
		return 0
	}

	ws, tab := findIDs(os.Getenv("HERDR_PLUGIN_EVENT_JSON"))
	if ws == "" {
		ws = os.Getenv("HERDR_WORKSPACE_ID")
	}
	if tab == "" {
		tab = os.Getenv("HERDR_TAB_ID")
	}
	if tab == "" && ws != "" {
		if wss, err := listWorkspaces(); err == nil {
			for _, w := range wss {
				if w.WorkspaceID == ws {
					tab = w.ActiveTabID
				}
			}
		}
	}
	if ws == "" || tab == "" {
		fmt.Fprintf(os.Stderr, "auto: no workspace/tab in event %q\n", event)
		return 0
	}
	ensurePanes(ws, tab)
	return 0
}

type workspaceInfo struct {
	WorkspaceID string `json:"workspace_id"`
	ActiveTabID string `json:"active_tab_id"`
}

func listWorkspaces() ([]workspaceInfo, error) {
	out, err := exec.Command(herdrBin(), "workspace", "list").Output()
	if err != nil {
		return nil, fmt.Errorf("herdr workspace list: %w", err)
	}
	var resp struct {
		Result struct {
			Workspaces []workspaceInfo `json:"workspaces"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, err
	}
	return resp.Result.Workspaces, nil
}

// findIDs digs workspace_id / tab_id out of an event payload of unknown shape.
func findIDs(raw string) (ws, tab string) {
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return "", ""
	}
	var walk func(v any)
	walk = func(v any) {
		switch t := v.(type) {
		case map[string]any:
			if s, ok := t["workspace_id"].(string); ok && ws == "" {
				ws = s
			}
			if s, ok := t["tab_id"].(string); ok && tab == "" {
				tab = s
			}
			for _, c := range t {
				walk(c)
			}
		case []any:
			for _, c := range t {
				walk(c)
			}
		}
	}
	walk(v)
	return ws, tab
}

// tabLock stops two hooks (workspace.created + tab.created) racing on one tab.
func tabLock(tab string) (release func(), ok bool) {
	dir := envOr("HERDR_PLUGIN_STATE_DIR", os.TempDir())
	_ = os.MkdirAll(dir, 0o755)
	p := filepath.Join(dir, "auto-"+strings.NewReplacer(":", "_", "/", "_", `\`, "_").Replace(tab)+".lock")
	if fi, err := os.Stat(p); err == nil && time.Since(fi.ModTime()) > 30*time.Second {
		_ = os.Remove(p) // stale
	}
	f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, false
	}
	f.Close()
	return func() { _ = os.Remove(p) }, true
}

// ensurePanes lays a tab out as  Files | chat | Changes , opening whichever
// explorer pane is missing.
func ensurePanes(ws, tab string) {
	if ws == "" || tab == "" {
		return
	}
	release, ok := tabLock(tab)
	if !ok {
		return
	}
	defer release()

	panes, err := listPanes()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	var chat *paneInfo
	have := map[string]bool{}
	for i := range panes {
		p := &panes[i]
		if p.TabID != tab {
			continue
		}
		if p.Label == "Files" || p.Label == "Changes" {
			have[p.Label] = true
		} else if chat == nil || p.Focused {
			chat = p
		}
	}
	if chat == nil {
		return
	}
	// back = a pane we opened + the direction from it to the chat, so focus
	// can return to the chat (herdr only focuses neighbours).
	var back, dir string
	// Changes first: split chat → [chat | Changes], Changes gets 30%.
	if !have["Changes"] {
		if id := openBeside(chat, "changes"); id != "" {
			_ = exec.Command(herdrBin(), "pane", "resize", "--pane", id, "--direction", "right", "--amount", "0.2").Run()
			back, dir = id, "left"
		}
	}
	// Files: split chat again, then swap it to the chat's left and narrow it.
	if !have["Files"] {
		if id := openBeside(chat, "files"); id != "" {
			_ = exec.Command(herdrBin(), "pane", "swap", "--pane", id, "--direction", "left").Run()
			_ = exec.Command(herdrBin(), "pane", "resize", "--pane", id, "--direction", "left", "--amount", "0.25").Run()
			back, dir = id, "right"
		}
	}
	if back != "" {
		_ = exec.Command(herdrBin(), "pane", "focus", "--pane", back, "--direction", dir).Run()
	}
}

// openBeside opens an explorer entrypoint to the right of chat; returns its pane id.
func openBeside(chat *paneInfo, entry string) string {
	args := []string{"plugin", "pane", "open",
		"--plugin", envOr("HERDR_PLUGIN_ID", "herdr-explorer"), "--entrypoint", entry,
		"--target-pane", chat.PaneID, "--direction", "right", "--no-focus",
		"--env", "HERDR_EXPLORER_TARGET=" + chat.PaneID}
	if chat.Cwd != "" {
		args = append(args, "--env", "HERDR_EXPLORER_ROOT="+chat.Cwd)
	}
	out, err := exec.Command(herdrBin(), args...).Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pane open", entry+":", err)
		return ""
	}
	var resp struct {
		Result struct {
			PluginPane struct {
				Pane paneInfo `json:"pane"`
			} `json:"plugin_pane"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &resp) != nil {
		return ""
	}
	return resp.Result.PluginPane.Pane.PaneID
}
