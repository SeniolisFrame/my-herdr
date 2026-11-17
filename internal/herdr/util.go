package herdr

import (
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
