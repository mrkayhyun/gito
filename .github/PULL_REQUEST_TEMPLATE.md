## What

<!-- What does this change, in a sentence or two? -->

## Why

<!-- The motivation. Link the issue if there is one: Fixes #123 -->

## How I verified it

<!-- What you actually ran or clicked through. "gofmt/vet/test pass" plus the
     TUI flow you exercised by hand is the useful answer here. -->

## Checklist

- [ ] `gofmt -l .` prints nothing
- [ ] `go build ./... && go vet ./... && go test -race ./...` pass
- [ ] Commit messages follow Conventional Commits (`feat:` / `fix:` / `docs:` / …)

If this PR touches user-facing strings:

- [ ] Strings added to all four locales in `internal/i18n` (en, ko, ja, zh) —
      or English placeholders used, and called out below

If this PR execs `git` (see [SECURITY.md](../SECURITY.md#threat-model)):

- [ ] Arguments passed as a slice, never as a shell string
- [ ] `--end-of-options` before user-influenced refs, `--` before pathspecs
- [ ] User-supplied branch/tag names run through `git.ValidateRefName`

If this PR adds a destructive operation:

- [ ] Guarded by a confirmation step that accepts `esc` to cancel

## Notes for the reviewer

<!-- Anything you're unsure about, translations you couldn't do, follow-ups you
     deliberately left out of scope. -->
