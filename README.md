# my-herdr

herdr plugin (`herdr-explorer`): file tree with file-type icons and git status,
current git branch, a per-file collapsible git diff viewer, and sending files
to the chat pane as `@path`.

## Build & link

```
go build -o bin/herdr-explorer.exe .
herdr plugin link <path-to-this-folder>
```

## Layout & keys

Each tab opens as `Files | chat | Changes` automatically.

| Key | Action |
| --- | --- |
| `alt+e` | next pane from anywhere (herdr `cycle_pane_next`, set in config.toml; add `"alt+ำ"` for the Thai layout) |
| `esc` | back to the chat |
| `s`, right-click | type `@path ` into the chat pane |
| `?` | help |

Changes pane (Fork-like):

| Key | Action |
| --- | --- |
| `space` / `S` / `U` | stage-unstage file (cursor on its header) or one hunk (cursor inside it) / stage all / unstage all (✓ staged, ~ partly) |
| `c` | commit message box (empty during a merge = git's merge message) |
| `D` | discard the file's changes (asks first) |
| `⏎` `a` `n` `p` | fold file / fold all / next / previous file |
| `L` | history graph of all branches; `⏎` shows a commit, `esc` back |
| `O` / `T` | conflict (status `C`): take ours / theirs, or edit and `space` |

Both panes follow the chat pane's folder (checked every 2s).
Both panes: `b` checkout branch, `B` new branch, `m` merge, `X` abort merge,
`f` fetch, `F` pull, `P` push, `z` stash, `Z` pop stash. Every key also works
on the Thai (Kedmanee) layout.

Icons are emoji (no special font needed); `--icons none` turns them off.
