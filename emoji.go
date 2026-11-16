package main

import (
	"path/filepath"
	"strings"
)

// Emoji icon set: renders in colour in Windows Terminal (Segoe UI Emoji) and
// most modern terminals without installing a font. Every emoji here has
// default emoji presentation (no U+FE0F needed) so it is exactly 2 cells wide.

var emojiNames = map[string]string{
	"dockerfile":         "🐳",
	"docker-compose.yml": "🐳",
	"compose.yaml":       "🐳",
	"makefile":           "🔨",
	".gitignore":         "🙈",
	".gitattributes":     "🙈",
	".gitmodules":        "🙈",
	"license":            "📜",
	"license.md":         "📜",
	"readme.md":          "📘",
	"claude.md":          "🤖",
	"agents.md":          "🤖",
	"package.json":       "📦",
	"package-lock.json":  "🔒",
	"go.mod":             "📦",
	"go.sum":             "🔒",
	"cargo.toml":         "📦",
	"cargo.lock":         "🔒",
	".env":               "🔑",
	"herdr-plugin.toml":  "🧩",
}

var emojiExts = map[string]string{
	".go":     "🐹",
	".js":     "🟨",
	".mjs":    "🟨",
	".cjs":    "🟨",
	".ts":     "🟦",
	".mts":    "🟦",
	".jsx":    "🔵",
	".tsx":    "🔵",
	".vue":    "🟩",
	".svelte": "🟧",
	".py":     "🐍",
	".rs":     "🦀",
	".java":   "🍵",
	".kt":     "🟪",
	".rb":     "💎",
	".php":    "🐘",
	".lua":    "🌙",
	".c":      "🔷",
	".h":      "🔷",
	".cpp":    "🔷",
	".hpp":    "🔷",
	".cs":     "🟪",
	".swift":  "🐦",
	".html":   "🌐",
	".css":    "🎨",
	".scss":   "🎨",
	".json":   "📋",
	".jsonc":  "📋",
	".yaml":   "🔧",
	".yml":    "🔧",
	".toml":   "🔧",
	".ini":    "🔧",
	".xml":    "📰",
	".md":     "📝",
	".mdx":    "📝",
	".txt":    "📄",
	".log":    "📃",
	".csv":    "📊",
	".sql":    "💾",
	".db":     "💾",
	".sh":     "🐚",
	".bash":   "🐚",
	".zsh":    "🐚",
	".ps1":    "🐚",
	".bat":    "🐚",
	".cmd":    "🐚",
	".exe":    "🚀",
	".lock":   "🔒",
	".env":    "🔑",
	".pem":    "🔑",
	".key":    "🔑",
	".png":    "🌄",
	".jpg":    "🌄",
	".jpeg":   "🌄",
	".gif":    "🌄",
	".webp":   "🌄",
	".svg":    "🌄",
	".ico":    "🌄",
	".pdf":    "📕",
	".zip":    "📦",
	".gz":     "📦",
	".tar":    "📦",
	".7z":     "📦",
	".mp3":    "🎵",
	".wav":    "🎵",
	".mp4":    "🎬",
	".mov":    "🎬",
	".ttf":    "🔤",
	".woff":   "🔤",
	".woff2":  "🔤",
}

func emojiFile(name string) string {
	lower := strings.ToLower(name)
	if e, ok := emojiNames[lower]; ok {
		return e
	}
	if strings.HasPrefix(lower, ".env.") {
		return "🔑"
	}
	if strings.HasSuffix(lower, "_test.go") || strings.Contains(lower, ".test.") || strings.Contains(lower, ".spec.") {
		return "🧪"
	}
	if e, ok := emojiExts[filepath.Ext(lower)]; ok {
		return e
	}
	return "📄"
}

var emojiDirs = map[string]string{
	".github":      "🐙",
	".vscode":      "🔵",
	".claude":      "🤖",
	"node_modules": "📦",
	"bin":          "🚀",
	"dist":         "🚀",
	"build":        "🚀",
	"test":         "🧪",
	"tests":        "🧪",
	"docs":         "📚",
	"assets":       "🌄",
	"images":       "🌄",
}

func emojiDir(name string, open bool) string {
	if e, ok := emojiDirs[strings.ToLower(name)]; ok {
		return e
	}
	if open {
		return "📂"
	}
	return "📁"
}
