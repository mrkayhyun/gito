# Contributing to gito

Thanks for taking the time. Issues and pull requests are both welcome — a bug
report with a clear reproduction is as useful as a patch.

By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).
For security problems, do **not** open a public issue — follow
[SECURITY.md](SECURITY.md) instead.

## Getting set up

You need Go 1.24+ and git.

```bash
git clone https://github.com/mrkayhyun/gito
cd gito
go build ./...
go test ./...
go run . status      # run a subcommand straight from source
```

To install your working copy onto your PATH, `./install.sh` builds the binary
into `~/.local/bin` and adds a marked PATH block to `~/.zshrc`;
`./install.sh --uninstall` reverses both.

## Before you open a pull request

Everything CI enforces, you can run locally:

```bash
gofmt -l .              # must print nothing
go build ./...
go vet ./...
go test -race ./...
```

CI additionally runs [gosec](https://github.com/securego/gosec) with `G204`
excluded, and a `go install github.com/mrkayhyun/gito@<sha>` smoke test that
proves the module is installable at its declared path.

## Architecture

The shape of the codebase is deliberately repetitive — the consistency is the
point, and it is what makes a new command easy to add.

```
main.go              subcommand dispatch + help text
internal/git/        git wrappers — exec `git`, parse output. no TUI code.
internal/ui/         one Bubble Tea model per command
internal/i18n/       message catalog (en, ko, ja, zh) + locale detection
internal/style/      shared lipgloss styles
internal/config/     ~/.config/gito/config.json
```

Two rules keep the layers honest:

- `internal/git` never imports Bubble Tea or lipgloss. It returns data and
  errors; it does not render.
- `internal/ui` never calls `exec` directly. Every git invocation goes through
  `internal/git` so that validation and the `--end-of-options` guards live in
  exactly one place.

### Adding a new command

1. Add the git wrapper function **and its tests** in `internal/git`. Tests
   there operate on a real temporary repository — see the existing
   `*_ext_test.go` files for the pattern.
2. Add an `xxxModel` (`Init`/`Update`/`View`) plus `RunXxx()` in `internal/ui`.
   `stash.go` and `tag.go` are the best templates to copy.
3. Register the command in the `main.go` switch and in the help text.
4. Add user-facing strings to **all four locales** in `internal/i18n`. If you
   only speak one of them, add English for the rest and say so in the PR — a
   native speaker can follow up, and that is much better than a missing key.
5. Run the checks above.

### Working with git arguments

This is the one area where a careless change has security consequences, so it
has its own rules — the reasoning is in [SECURITY.md](SECURITY.md#threat-model).

- Never build a shell string. Always pass an argument slice to `exec.Command`.
- Pass `--end-of-options` before any user-influenced ref, and `--` before any
  pathspec.
- Run user-supplied branch and tag names through `git.ValidateRefName` before
  they reach a command that creates or moves a ref.

### Destructive operations

Commands that can lose work (branch delete, stash drop, hard restore, tag
delete) must go through a confirmation step, and the confirm UI must accept
`esc` to cancel. A new destructive command without a confirm will be asked to
add one in review.

## Commits and pull requests

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/)
— `feat:`, `fix:`, `docs:`, `chore:`, `ci:`, `test:`, `refactor:`. This is not
cosmetic: `.goreleaser.yaml` groups the release changelog by these prefixes, and
`docs:`/`chore:`/`ci:`/`test:` are filtered out of it entirely.

Keep a PR to one logical change, describe what you changed and how you verified
it, and mention any locale strings you could not translate. Small PRs get
reviewed faster.

## Releases

Maintainers only. Releases are tag-driven: pushing a `vX.Y.Z` tag triggers
`.github/workflows/release.yml`, which runs GoReleaser to build the
cross-platform archives, publish the GitHub release, and push the Homebrew
formula to `mrkayhyun/homebrew-tap`.
