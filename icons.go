package main

import (
	"path/filepath"
	"strings"
)

// Glyphs are Nerd Font (v3) code points; the pane needs a Nerd Font in the
// terminal. Run with --icons none (or HERDR_EXPLORER_ICONS=none) without one.
type icon struct {
	glyph string
	color string
}

var defaultFileIcon = icon{"", "#6d8086"}

var nameIcons = map[string]icon{
	"dockerfile":        {"", "#458ee6"},
	"docker-compose.yml": {"", "#458ee6"},
	"compose.yaml":      {"", "#458ee6"},
	"makefile":          {"", "#6d8086"},
	".gitignore":        {"", "#f14c28"},
	".gitattributes":    {"", "#f14c28"},
	".gitmodules":       {"", "#f14c28"},
	"license":           {"", "#d0bf41"},
	"license.md":        {"", "#d0bf41"},
	"package.json":      {"", "#e8274b"},
	"package-lock.json": {"", "#7a0d21"},
	"go.mod":            {"", "#00add8"},
	"go.sum":            {"", "#00add8"},
	"cargo.toml":        {"", "#dea584"},
	"cargo.lock":        {"", "#dea584"},
	".env":              {"", "#faf743"},
	"readme.md":         {"", "#42a5f5"},
	"claude.md":         {"", "#d97757"},
	"tsconfig.json":     {"", "#3178c6"},
}

var extIcons = map[string]icon{
	".go":     {"", "#00add8"},
	".js":     {"", "#f1e05a"},
	".mjs":    {"", "#f1e05a"},
	".cjs":    {"", "#f1e05a"},
	".ts":     {"", "#3178c6"},
	".mts":    {"", "#3178c6"},
	".jsx":    {"", "#61dafb"},
	".tsx":    {"", "#3178c6"},
	".vue":    {"", "#42b883"},
	".svelte": {"", "#ff3e00"},
	".json":   {"", "#cbcb41"},
	".jsonc":  {"", "#cbcb41"},
	".md":     {"", "#519aba"},
	".mdx":    {"", "#519aba"},
	".py":     {"", "#3572a5"},
	".rs":     {"", "#dea584"},
	".html":   {"", "#e34c26"},
	".css":    {"", "#663399"},
	".scss":   {"", "#c6538c"},
	".yaml":   {"", "#cb171e"},
	".yml":    {"", "#cb171e"},
	".toml":   {"", "#9c4221"},
	".sh":     {"", "#4eaa25"},
	".bash":   {"", "#4eaa25"},
	".zsh":    {"", "#4eaa25"},
	".ps1":    {"", "#5391fe"},
	".bat":    {"", "#c1f12e"},
	".c":      {"", "#599eff"},
	".h":      {"", "#a074c4"},
	".cpp":    {"", "#f34b7d"},
	".hpp":    {"", "#a074c4"},
	".java":   {"", "#cc3e44"},
	".kt":     {"", "#7f52ff"},
	".php":    {"", "#a074c4"},
	".rb":     {"", "#cc342d"},
	".lua":    {"", "#51a0cf"},
	".sql":    {"", "#dad8d8"},
	".txt":    {"", "#89e051"},
	".log":    {"", "#8b949e"},
	".lock":   {"", "#bbbbbb"},
	".env":    {"", "#faf743"},
	".png":    {"", "#a074c4"},
	".jpg":    {"", "#a074c4"},
	".jpeg":   {"", "#a074c4"},
	".gif":    {"", "#a074c4"},
	".webp":   {"", "#a074c4"},
	".ico":    {"", "#cbcb41"},
	".svg":    {"", "#ffb13b"},
	".pdf":    {"", "#b30b00"},
	".zip":    {"", "#eca517"},
	".gz":     {"", "#eca517"},
	".tar":    {"", "#eca517"},
	".exe":    {"", "#9f0500"},
	".xml":    {"\U000f05c0", "#e37933"},
	".csv":    {"", "#89e051"},
}

func fileIcon(name string) icon {
	lower := strings.ToLower(name)
	if ic, ok := nameIcons[lower]; ok {
		return ic
	}
	if strings.HasPrefix(lower, ".env.") {
		return nameIcons[".env"]
	}
	if ic, ok := extIcons[filepath.Ext(lower)]; ok {
		return ic
	}
	return defaultFileIcon
}

var dirIcons = map[string]string{
	".github":      "",
	".git":         "",
	"node_modules": "",
	".vscode":      "",
}

func dirIcon(name string, open bool) icon {
	if g, ok := dirIcons[strings.ToLower(name)]; ok {
		return icon{g, "#7ebae4"}
	}
	if open {
		return icon{"", "#7ebae4"}
	}
	return icon{"", "#7ebae4"}
}

const branchGlyph = ""
