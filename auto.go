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
// Files pane is open on the left of a tab, so nobody has to press prefix+f.
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
			ensureFilesPane(w.WorkspaceID, w.ActiveTabID)
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
	ensureFilesPane(ws, tab)
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

func ensureFilesPane(ws, tab string) {
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
	var target *paneInfo
	for i := range panes {
		p := &panes[i]
		if p.TabID != tab {
			continue
		}
		if p.Label == "Files" {
			return // already there
		}
		if target == nil || p.Focused {
			target = p
		}
	}
	if target == nil {
		return
	}

	args := []string{"plugin", "pane", "open",
		"--plugin", envOr("HERDR_PLUGIN_ID", "herdr-explorer"), "--entrypoint", "files",
		"--target-pane", target.PaneID, "--direction", "right", "--no-focus",
		"--env", "HERDR_EXPLORER_TARGET=" + target.PaneID}
	if target.Cwd != "" {
		args = append(args, "--env", "HERDR_EXPLORER_ROOT="+target.Cwd)
	}
	out, err := exec.Command(herdrBin(), args...).Output()
	if err != nil {
		fmt.Fprintln(os.Stderr, "pane open:", err)
		return
	}
	var resp struct {
		Result struct {
			PluginPane struct {
				Pane paneInfo `json:"pane"`
			} `json:"plugin_pane"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &resp) != nil || resp.Result.PluginPane.Pane.PaneID == "" {
		return
	}
	id := resp.Result.PluginPane.Pane.PaneID
	// Move it to the left and make it narrow, like an editor sidebar.
	_ = exec.Command(herdrBin(), "pane", "swap", "--pane", id, "--direction", "left").Run()
	_ = exec.Command(herdrBin(), "pane", "resize", "--pane", id, "--direction", "left", "--amount", envOr("HERDR_EXPLORER_SHRINK", "0.25")).Run()
	// Swap/resize focus the sidebar; give focus back to the main pane.
	_ = exec.Command(herdrBin(), "pane", "focus", "--pane", id, "--direction", "right").Run()
}
