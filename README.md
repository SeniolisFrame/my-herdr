# my-herdr

herdr plugin (`herdr-explorer`): file tree with file-type icons and git status,
current git branch, a per-file collapsible git diff viewer, and sending files
to the chat pane as `@path`.

## Build & link

```
go build -o bin/herdr-explorer.exe .
herdr plugin link <path-to-this-folder>
```

## Keys

| Key | Action |
| --- | --- |
| `prefix+f` / `prefix+d` | open file tree / changes popup (config.toml bindings) |
| `tab` | switch Files ↔ Changes |
| `⏎` | open folder / fold file diff |
| `s`, right-click | type `@path ` into the neighbouring chat pane |
| `a` | fold/unfold all diffs |
| `n` / `p` | next / previous file in diff |
| `q` | quit |

Icons need a Nerd Font; use `--icons none` otherwise.
