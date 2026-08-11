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
| `commit` | 5-step wizard (type → scope → subject → body → confirm); types customizable via `gito.json` | `y` commit, `a` amend, `e` edit |
| `log` | Scrollable commit log with detail diff | `↑/↓` move, `enter` detail, `g/G` top/bottom |
| `branch` | Fuzzy-filter switch + create / rename / delete | `enter` switch, `^b` create, `^r` rename, `^d` delete, `^x` force-delete |
| `status` | Stage / unstage / diff / discard | `space` toggle, `a` stage all, `d` diff, `D` discard |
| `stash` | Manage the stash list | `p` pop, `a` apply, `d` diff, `D` drop |
| `tag` | Create (lightweight/annotated) / delete / push tags | `c` create, `p` push, `P` delete remote, `D` delete |
| `remote` | Remote list, fetch, upstream ahead/behind | `f` fetch, `F` fetch all, `r` refresh |
| `diff` | Pick two refs (branch/tag) and compare | `enter` select (base → target) |
| `reflog` | Browse reflog and recover commits (non-destructive: creates a new branch) | `b` branch from here |
| `blame` | Pick a file and view line-by-line blame | `enter` view blame |

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

### Exit keys

- List screens: `esc` or `ctrl+c` to quit (screens without a filter also accept `q`)
- Detail / diff screens: `q` or `esc` to go back to the list

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

## Architecture

gito follows a simple three-layer design:

```
main.go                 // subcommand routing + help
└── internal/
    ├── git/            // thin wrappers around the git CLI (isolated side effects, unit-tested)
    ├── ui/             // one Bubble Tea model (Model/Update/View) per command
    ├── config/         // gito.json / ~/.config/gito loading
    ├── i18n/           // localization catalog (en/ko/ja/zh) + locale detection
    └── style/          // shared Lip Gloss palette
```

**Design principle:** state lives in the model, side effects live in the git layer, and the UI stays
a pure `Update` function — easy to test and reason about.

## Development

```bash
go build ./...     # build
go vet ./...       # static analysis
go test ./...      # tests (the git layer is verified against real temp repos)
```

## Contributing

Issues and pull requests are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md).

Security problems should be reported privately; see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE) © mrkayhyun
