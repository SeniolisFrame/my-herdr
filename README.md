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
| `⏎` | open folder / fold file diff |
| `s`, right-click | type `@path ` into the chat pane |
| `a` | fold/unfold all diffs |
| `n` / `p` | next / previous file in diff |
| `q` | close the pane |

Icons are emoji (no special font needed); `--icons none` turns them off.
