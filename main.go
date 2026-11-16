// Command herdr-explorer is a herdr plugin pane: a file tree with file-type
// icons and git status, plus a per-file collapsible git diff viewer.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) > 2 && os.Args[1] == "open" {
		os.Exit(openPane(os.Args[2]))
	}

	viewFlag := flag.String("view", "files", "initial view: files or diff")
	rootFlag := flag.String("root", "", "directory to show (default: taken from the herdr context)")
	iconsFlag := flag.String("icons", envOr("HERDR_EXPLORER_ICONS", "nerd"), "icon set: nerd or none")
	printCtx := flag.Bool("print-context", false, "print the herdr context and the resolved root, then exit")
	flag.Parse()

	root := resolveRoot(*rootFlag)
	if *printCtx {
		fmt.Println("HERDR_PLUGIN_CONTEXT_JSON =", os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"))
		fmt.Println("resolved root =", root)
		return
	}

	m := newModel(root, *viewFlag == "diff", *iconsFlag != "none")
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// openPane is what the [[actions]] run: it asks herdr to open one of this
// plugin's panes, so the panes can be bound to keys.
func openPane(entry string) int {
	bin := envOr("HERDR_BIN_PATH", "herdr")
	id := envOr("HERDR_PLUGIN_ID", "herdr-explorer")
	cmd := exec.Command(bin, "plugin", "pane", "open", "--plugin", id, "--entrypoint", entry)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// resolveRoot picks the folder to show: --root, $HERDR_EXPLORER_ROOT, a
// directory found in the herdr context JSON, then the working directory.
// Inside a git repo it moves up to the repo top level so diff paths line up.
func resolveRoot(flagRoot string) string {
	var cands []string
	cands = append(cands, flagRoot, os.Getenv("HERDR_EXPLORER_ROOT"))
	cands = append(cands, contextDirs(os.Getenv("HERDR_PLUGIN_CONTEXT_JSON"))...)
	pluginRoot := os.Getenv("HERDR_PLUGIN_ROOT")
	if wd, err := os.Getwd(); err == nil && (pluginRoot == "" || !samePath(wd, pluginRoot)) {
		cands = append(cands, wd)
	}
	if home, err := os.UserHomeDir(); err == nil {
		cands = append(cands, home)
	}
	for _, c := range cands {
		if c == "" || !isDir(c) {
			continue
		}
		abs, err := filepath.Abs(c)
		if err != nil {
			continue
		}
		if top := gitTopLevel(abs); top != "" {
			return top
		}
		return abs
	}
	return "."
}

// contextDirs walks the context JSON and returns every string value that is
// an existing directory, worktree/cwd-like keys first.
func contextDirs(raw string) []string {
	if raw == "" {
		return nil
	}
	var v any
	if json.Unmarshal([]byte(raw), &v) != nil {
		return nil
	}
	type cand struct {
		score int
		dir   string
	}
	var found []cand
	pluginRoot := os.Getenv("HERDR_PLUGIN_ROOT")
	var walk func(key string, v any)
	walk = func(key string, v any) {
		switch t := v.(type) {
		case map[string]any:
			keys := make([]string, 0, len(t))
			for k := range t {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				walk(strings.ToLower(k), t[k])
			}
		case []any:
			for _, e := range t {
				walk(key, e)
			}
		case string:
			if t == "" || !isDir(t) || (pluginRoot != "" && samePath(t, pluginRoot)) {
				return
			}
			score := 3
			switch {
			case strings.Contains(key, "worktree"):
				score = 0
			case strings.Contains(key, "cwd"):
				score = 1
			case strings.Contains(key, "dir"), strings.Contains(key, "path"), strings.Contains(key, "root"):
				score = 2
			}
			found = append(found, cand{score, t})
		}
	}
	walk("", v)
	sort.SliceStable(found, func(i, j int) bool { return found[i].score < found[j].score })
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.dir
	}
	return out
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

func samePath(a, b string) bool {
	a, _ = filepath.Abs(a)
	b, _ = filepath.Abs(b)
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
