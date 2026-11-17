# my-herdr

A [herdr](https://herdr.dev) plugin (`herdr-explorer`) that works like a small
Fork git client around your chat pane:

```
 Files            │ chat (your agent) │ Changes
 file tree, read, │                   │ git diff per file, stage files
 edit, search,    │                   │ or hunks, commit, history,
 blame, history   │                   │ branches, merge, pull, push
```

## Install

You need **herdr 0.7 or newer**, **git**, and **Go 1.27 or newer** (the plugin
is built from source on install).

1. Install the plugin. herdr downloads it and runs `go build`:

   ```
   herdr plugin install SeniolisFrame/my-herdr
   ```

2. Open a new workspace or tab, or restart herdr. Each tab now opens as
   `Files | chat | Changes`.

3. Recommended: a key that cycles the three panes from anywhere. Add this to
   herdr's `config.toml` (Windows: `%APPDATA%\herdr\config.toml`), then run
   `herdr server reload-config`:

   ```toml
   [keys]
   cycle_pane_next = ["prefix+tab", "alt+e", "alt+ำ"] # alt+ำ = alt+e on the Thai layout
   ```

Update: `herdr plugin update`. Remove: `herdr plugin uninstall herdr-explorer`.

To stop the panes from opening by themselves, create an empty file named
`no-auto` in the folder printed by `herdr plugin config-dir herdr-explorer`.

### From a local clone (for development)

```
git clone https://github.com/SeniolisFrame/my-herdr
cd my-herdr
go build -o bin/herdr-explorer.exe .
herdr plugin link .
```

Rebuild after every change. On Windows a running pane locks the exe: rename
the old `bin/herdr-explorer.exe` first, then build.

## Use

Both panes follow the chat pane's folder (checked every 2 s). Every key also
works on the Thai (Kedmanee) layout.

| Key | Anywhere |
| --- | --- |
| `alt+e` | next pane (with the config above) |
| `esc` | back to the chat |
| `s`, right-click | type `@path ` into the chat |
| `?` | all keys |

### Files (left)

| Key | Action |
| --- | --- |
| `⏎` | open a folder, or read a file (syntax colours, long lines wrap, pane zooms) |
| `/` | search all files; `⏎` on a hit opens the file at that line |
| in a file: `/` `n` `N` | search in the file, next, previous |
| in a file: `e` | edit the file here (`$EDITOR`, else nano or vim; on Windows the nano from Git for Windows) |
| in a file: `b` / `L` | blame (who changed each line) / the file's history |

The footer shows the full path of the file under the cursor.

nano basics: type to edit, `ctrl+s` save, `ctrl+x` quit, `alt+a` start a
selection, `alt+6` copy, `ctrl+k` cut line, `ctrl+u` paste.

### Changes (right)

| Key | Action |
| --- | --- |
| `space` / `S` / `U` | stage or unstage the file (cursor on its header) or one hunk (cursor inside it) / stage all / unstage all (✓ staged, ~ partly) |
| `c` | commit (empty message during a merge = git's merge message) |
| `D` | discard the file's changes (asks first) |
| `⏎` `a` `n` `p` | fold file / fold all / next / previous file |
| `L` | history graph of all branches; `⏎` shows a commit |
| `O` / `T` | merge conflict (status `C`): take ours / theirs, or edit and `space` |

Both panes: `b` checkout branch, `B` new branch, `m` merge, `X` abort merge,
`f` fetch, `F` pull, `P` push, `z` stash, `Z` pop stash.

## Known issues

- Thai vowel and tone marks can overlap inside herdr panes. herdr itself
  measures the width of these characters, and has no setting for it.
- Icons are emoji (no special font needed); `--icons none` turns them off.

## Code

| Path | What |
| --- | --- |
| `herdr-plugin.toml` | plugin manifest: build command, panes, hooks that open them |
| `main.go` | flags, picks the folder, runs the pane (`auto` = open the panes) |
| `internal/git` | git commands: status, diff parsing, stage/commit, branches, merge, log |
| `internal/herdr` | herdr CLI: pane layout on new tabs, focus, zoom, sending `@path` |
| `internal/ui` | the bubbletea pane: model + keys, view, reader, editor, blame/history, tree |

Tests: `go test ./...`
