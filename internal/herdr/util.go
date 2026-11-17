package herdr

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// The side panes follow the chat pane (checked on the 2s poll): when it is in
// another folder, they switch to that folder's repo (or the folder itself).

// ChatCwd is the chat pane's current folder; "" outside herdr or when unknown.
func ChatCwd() string {
	target := os.Getenv("HERDR_EXPLORER_TARGET")
	if target == "" {
		return ""
	}
	panes, err := ListPanes()
	if err != nil {
		return ""
	}
	for _, p := range panes {
		if p.PaneID == target && IsDir(p.Cwd) {
			return filepath.Clean(p.Cwd)
		}
	}
	return ""
}

// Zoom zooms this plugin pane in or out; outside herdr it does nothing.
func Zoom(on bool) {
	if pane := os.Getenv("HERDR_PANE_ID"); pane != "" {
		flag := map[bool]string{true: "--on", false: "--off"}[on]
		_ = exec.Command(Bin(), "pane", "zoom", "--pane", pane, flag).Run()
	}
}

// Focus reports which pane has focus in this pane's tab and whether it is
// this pane; ok is false outside herdr or when herdr can't be asked.
func Focus() (self bool, focused string, ok bool) {
	me := os.Getenv("HERDR_PANE_ID")
	panes, err := ListPanes()
	if me == "" || err != nil {
		return false, "", false
	}
	tab := ""
	for _, p := range panes {
		if p.PaneID == me {
			tab = p.TabID
		}
	}
	for _, p := range panes {
		if p.Focused && p.TabID == tab {
			return p.PaneID == me, p.PaneID, true
		}
	}
	return false, "", false
}

// UnzoomTo unzooms this pane, which also pulls focus back to it, then walks
// focus right until it reaches want again (the pane alt+e moved to).
func UnzoomTo(want string) {
	me := os.Getenv("HERDR_PANE_ID")
	Zoom(false)
	cur := me
	for range 3 {
		if cur == want || want == "" {
			return
		}
		out, err := exec.Command(Bin(), "pane", "focus", "--pane", cur, "--direction", "right").Output()
		if err != nil {
			return
		}
		var resp struct {
			Result struct {
				Focus struct {
					ID string `json:"focused_pane_id"`
				} `json:"focus"`
			} `json:"result"`
		}
		if json.Unmarshal(out, &resp) != nil || resp.Result.Focus.ID == cur {
			return
		}
		cur = resp.Result.Focus.ID
	}
}

func IsDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func SamePath(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
