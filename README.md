<div align="center">

# gito

**A fast, subcommand-based Git TUI for people who live in the terminal.**

Pick from a menu instead of memorizing flags. Every destructive action asks first.
Built in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea) & [Lip Gloss](https://github.com/charmbracelet/lipgloss).

[![CI](https://github.com/mrkayhyun/gito/actions/workflows/ci.yml/badge.svg)](https://github.com/mrkayhyun/gito/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/mrkayhyun/gito?sort=semver)](https://github.com/mrkayhyun/gito/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/mrkayhyun/gito)](https://goreportcard.com/report/github.com/mrkayhyun/gito)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Made with Charm](https://img.shields.io/badge/Made%20with-Charm-ff69b4.svg)](https://charm.sh)

English · [한국어](README.ko.md)

<img src="docs/demo.gif" alt="gito demo" width="720">

</div>

---

## Why gito?

Most everyday Git friction comes from **commands and flags you have to remember**.
`git rebase -i`, `git reflog`, `git commit --amend`, `git push origin --delete <tag>` are
powerful, but you have to recall the exact syntax every time — and mistakes are hard to undo.

gito removes that friction:

- **Discoverable** — choose from a menu and follow on-screen key hints instead of memorizing commands.
- **Safe by default** — destructive operations (branch delete, remote tag delete, discard…) always confirm first.
- **Terminal-native** — no separate GUI; works the same over SSH and on remote servers.
- **Instant** — ships as a single static binary with no runtime dependencies.
- **Multilingual** — UI auto-detects your locale (English, 한국어, 日本語, 中文).

## gito vs. lazygit / tig / gitui

`lazygit`, `tig`, and `gitui` are excellent. gito aims at a **different point in the design space**:

| | **gito** | lazygit | tig | gitui |
|---|:---:|:---:|:---:|:---:|
| Model | **Focused subcommands** (`gito tag`, `gito commit`) | One big dashboard | Log-centric browser | One big dashboard |
| Enter a task | Jump straight to one screen, then back to the shell | Navigate panels | Navigate views | Navigate tabs |
| Commit conventions | **Config-driven** (`gito.json` commit types) | — | — | — |
| Built-in i18n | **en / ko / ja / zh** | — | — | — |
| Codebase | **Minimal, one Bubble Tea model per command** (great to learn from) | Large | C | Rust |
| Language | Go | Go | C | Rust |

In short: gito is **a set of small, purpose-built tools** rather than a single monolithic UI — enter the exact
screen you need, finish, and drop back to your shell. Its small, readable codebase also makes it a friendly
place to learn Go + TUI development.

## Commands

```
gito commit    Interactive commit wizard (5-step, config-driven types)
gito log       Scrollable log viewer  (↑/↓ navigate, enter: detail)
gito branch    Fuzzy branch switcher / create / rename / delete
gito status    Interactive stage / unstage / diff / discard
gito stash     Stash list  (pop / apply / show diff / drop)
gito tag       Tag manager (create / delete / push / show diff)
gito remote    Remote list (fetch / ahead-behind status)
gito diff      Compare two refs (branch/tag) and view the diff
gito reflog    Browse reflog and recover commits into a new branch
gito blame     Pick a file and view line-by-line blame
```

| Command | What it does | Key bindings |
|------|------|---------|
| `commit` | 5-step wizard (type → scope → subject → body → confirm) with a live message preview; types customizable via `gito.json` | `enter` next, `esc` back, `y` commit, `a` amend, `e` edit |
| `log` | Scrollable commit log with detail diff | `↑/↓` `j/k` move, `g/G` top/bottom, `enter` detail |
| `branch` | Fuzzy-filter switch + create / rename / delete | type to filter, `↑/↓` `^p/^n` move, `enter` switch, `^b` create, `^r` rename, `^d` delete, `^x` force-delete |
| `status` | Stage / unstage / diff / discard | `space` toggle, `a` stage all, `d` diff, `D` discard |
| `stash` | Manage the stash list | `enter` / `p` pop, `a` apply, `d` diff, `D` drop |
| `tag` | Create (lightweight/annotated) / delete / push tags | `enter` / `d` show, `c` create, `p` push, `P` delete remote, `D` delete |
| `remote` | Remote list, fetch, upstream ahead/behind | `f` fetch, `F` fetch all, `r` refresh |
| `diff` | Pick two refs (branch/tag) and compare | `enter` select (base → target), `esc` back one step |
| `reflog` | Browse reflog and recover commits (non-destructive: creates a new branch) | `g/G` top/bottom, `b` branch from here |
| `blame` | Pick a file and view line-by-line blame | type to filter, `↑/↓` `^p/^n` move, `enter` view blame |

Every screen shows its own hints in a footer fitted to the terminal width, and `?` opens the full
key table on the screens that have one (see [Keys and help](#keys-and-help)).

## Install

### Homebrew (macOS / Linux)

```bash
brew install mrkayhyun/tap/gito
```

### go install

```bash
go install github.com/mrkayhyun/gito@latest
```

### Prebuilt binaries

Download the archive for your OS/arch from the [Releases page](https://github.com/mrkayhyun/gito/releases),
extract it, and put the `gito` binary on your `PATH`.

### Install script (zsh)

Builds from source, installs to `~/.local/bin`, and wires up your `~/.zshrc` PATH automatically.

```bash
git clone https://github.com/mrkayhyun/gito && cd gito
./install.sh
source ~/.zshrc   # or open a new terminal
```

- Custom location: `INSTALL_DIR=/usr/local/bin ./install.sh`
- Uninstall: `./install.sh --uninstall` (removes the binary + the `~/.zshrc` block)
- The `~/.zshrc` change is a single marked, idempotent block — safe to re-run.

### Build from source

```bash
git clone https://github.com/mrkayhyun/gito && cd gito
go build -o gito .
# stamp a version:
go build -ldflags "-X main.version=v1.1.0" -o gito .
```

The result is a single dependency-free binary.

## Usage

Run any subcommand from inside a Git repository:

```bash
gito status     # stage / review changes
gito commit     # write a commit interactively
gito log        # browse history
gito diff       # compare two branches/tags
```

### Launcher menu

Don't remember the command names? Just run `gito` with no arguments to open the **interactive launcher**.
Move with `↑/↓` or jump directly by number (`1`–`9`, `0` for the 10th).

```bash
gito            # open the launcher menu
gito menu       # (explicitly) open the launcher menu
```

### Other commands

```bash
gito help       # full command help (same as gito -h / --help)
gito version    # print version    (same as gito -v / --version)
```

Running a command outside a Git repository prints a friendly hint and exits.

### Keys and help

- List screens: `esc` or `ctrl+c` to quit (screens without a text filter also accept `q`)
- Detail / diff screens: `↑/↓` `j/k` scroll, `PgUp/PgDn` page, `q` or `esc` to go back to the list
- Confirmation prompts: `y` confirms, any other key cancels
- `?` opens a key-bindings overlay listing every binding of the current screen, on `status`, `log`,
  `stash`, `tag`, the `diff` ref picker, the `remote` list and the `reflog` list. `branch` and `blame`
  filter as you type and the launcher and `commit` read single keys, so those screens bind no `?`
  and keep their hints in the footer. An armed confirmation ignores `?` until you answer it.

## Configuration (optional)

Define commit types to match your team's conventions. Config is resolved in this order:

1. `./gito.json` (project root, takes precedence)
2. `~/.config/gito/config.json`

```json
{
  "lang": "en",
  "commit_types": [
    {"key": "feat", "label": "feat      A new feature"},
    {"key": "fix",  "label": "fix       A bug fix"},
    {"key": "docs", "label": "docs      Documentation only"}
  ]
}
```

Without config, gito uses Conventional Commits defaults (feat/fix/docs/style/refactor/test/chore).
The `lang` field (`en` / `ko` / `ja` / `zh`) overrides locale auto-detection; you can also set `GITO_LANG`.

## Terminal compatibility

gito should look the same in a local terminal and over SSH on a bare server.

- **Light and dark backgrounds.** Every color is an adaptive Lip Gloss color with a separate light and
  dark variant, and it degrades to whatever the terminal reports. `NO_COLOR` is honored.
- **Non-UTF-8 terminals.** Cursors, check marks, arrows, box borders and launcher icons come from a
  glyph table that falls back to plain ASCII when `LC_ALL` / `LC_CTYPE` / `LANG` do not announce UTF-8.
  Set `GITO_ASCII=1` (also `true` / `yes` / `y` / `on`) to force the ASCII table anywhere.
- **Any terminal size.** Every screen reacts to resize: lists scroll and keep the cursor visible, key
  hints are dropped to fit one line, and long lines are truncated instead of wrapping. Narrow or short
  terminals get a degraded layout rather than a broken one. The layout floors at **20 columns by
  6 rows**: below that gito keeps laying out for 20x6, so a narrower terminal wraps lines and a
  shorter one scrolls. Every size at or above the floor is handled without wrapping.

## Architecture

gito follows a simple three-layer design:

```
main.go                 // subcommand routing + help
└── internal/
    ├── git/            // thin wrappers around the git CLI (isolated side effects, unit-tested)
    ├── ui/             // one Bubble Tea model (Model/Update/View) per command
    │   └── chrome.go   // shared chrome: layout, header, hint footer, help overlay, rows, scrolling
    ├── config/         // gito.json / ~/.config/gito loading
    ├── i18n/           // localization catalog (en/ko/ja/zh) + locale detection
    └── style/          // semantic adaptive theme, glyph tables, ANSI-aware width helpers
```

All ten command models render through `internal/ui/chrome.go`, so the body-height math, the one-line
header, the fitted key-hint footer, the `?` overlay, message and confirmation banners, selected rows
and list scrolling are written once instead of per screen. `internal/style` exposes color *roles*
(`Hash`, `MetaDim`, `Staged`, `Unstaged`, `DangerBar`, …) instead of hex codes, plus the
Unicode/ASCII glyph table and ANSI-aware width helpers that keep pre-colored `git` output from
breaking the layout.

**Design principle:** state lives in the model, side effects live in the git layer, and the UI stays
a pure `Update` function — easy to test and reason about.

## Development

```bash
go build ./...     # build
go vet ./...       # static analysis
go test ./...      # tests (the git layer is verified against real temp repos)
```

## Contributing

To add a new command:

1. Add the git wrapper function + tests in `internal/git`.
2. Add an `xxxModel` (Model/Update/View) and `RunXxx()` in `internal/ui` — `stash.go` / `tag.go` are good templates.
3. Register the command in the `main.go` switch and help text.
4. Make sure `go build ./... && go vet ./... && go test ./...` all pass.

Issues and pull requests are welcome.

## License

[MIT](LICENSE) © mrkayhyun
