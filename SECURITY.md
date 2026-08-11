# Security Policy

## Supported versions

gito is a single-binary CLI with no server component. Only the latest released
version receives security fixes. Please upgrade before reporting an issue.

| Version | Supported |
| ------- | --------- |
| latest release | ✅ |
| older releases | ❌ |

## Reporting a vulnerability

Please **do not** open a public issue for a security problem.

Use GitHub's private reporting instead:
[**Report a vulnerability**](https://github.com/mrkayhyun/gito/security/advisories/new).

If private reporting is unavailable to you, email **mrkayhyun@gmail.com** with
`[gito security]` in the subject.

Please include:

- the gito version (`gito version`) and OS/arch,
- the `git` version (`git --version`),
- a minimal reproduction — ideally the exact ref name, path, or repository
  state that triggers the behaviour,
- what you believe an attacker gains.

**Response expectations.** This is a small volunteer-maintained project, so
please treat these as good-faith targets rather than guarantees: an
acknowledgement within 7 days, an assessment within 14 days, and a fix released
before public disclosure where the report is confirmed. Reporters are credited
in the release notes unless they ask otherwise.

## Threat model

gito is a TUI wrapper that builds argument lists and executes the system `git`
binary. It does not open sockets, does not phone home, and holds no credentials
of its own. The realistic attack surface is therefore narrow and specific:

### 1. Argument construction for `git` (primary surface)

Every git operation is an `exec.Command("git", ...)` call with an explicitly
constructed argument slice — gito never builds a shell string, so there is no
shell metacharacter interpretation and no `sh -c` involved. The residual risk is
**option injection**: a value that reaches git in argument position but begins
with `-` would be parsed as a flag rather than as data.

Three mitigations apply, and they are defence in depth rather than alternatives:

- **`--end-of-options`** is passed before user-influenced ref arguments
  (`git checkout --end-of-options <name>`, `git show --end-of-options <hash>`,
  and similar), so git treats what follows as an operand regardless of content.
- **`--`** separates options from pathspecs on path-taking commands
  (`git add -- <path>`, `git restore -- <path>`).
- **`ValidateRefName`** (`internal/git/validate.go`) rejects names that are
  empty, begin with `-`, contain whitespace or ASCII control characters, contain
  git ref metacharacters (`..`, `@{`, `~`, `^`, `:`, `?`, `*`, `[`, `\`), start
  or end with `/`, end with `.lock`, or have a path component starting with `.`.
  It is a pure function with unit tests in `validate_test.go`.

Reports we are especially interested in: **a code path that passes a
user-controlled value to git without either `--end-of-options`/`--` or
`ValidateRefName`.** Not every call site needs all three, but a ref-accepting
path that has none of them is a bug worth reporting.

Note that gosec's **G204** (subprocess launched with variable arguments) is
excluded in CI. That exclusion is deliberate and documented inline in
`.github/workflows/ci.yml`: gito's entire purpose is to exec `git` with
constructed arguments, so G204 fires on essentially every call site and cannot
distinguish the guarded ones. All other gosec rules remain active. If you
believe the exclusion is masking a genuine finding, say so in your report — that
is a legitimate thing to challenge.

### 2. `install.sh` modifying shell configuration

`install.sh` builds the binary, installs it to `INSTALL_DIR` (default
`~/.local/bin`), and **appends a PATH block to `~/.zshrc`** between
`# >>> gito installer >>>` / `# <<< gito installer <<<` markers. Writing to a
user's shell rc file is a genuinely sensitive operation, so it is in scope:

- the block is idempotent (re-running does not duplicate it) and
  `./install.sh --uninstall` removes both the binary and the block;
- `INSTALL_DIR` and `ZSHRC` are caller-supplied environment variables — a
  scenario where they cause a write outside the intended file, or where the
  marker-block `awk` rewrite corrupts or truncates an existing `~/.zshrc`, is a
  valid report;
- the script requires a local Go toolchain and is intended to be run from a
  checked-out clone that the user has already inspected. We do not publish a
  `curl | sh` one-liner, and adding one is not planned.

### 3. Rendering untrusted repository content

gito renders git output — commit messages, branch names, diffs, blame output,
author names — into a terminal. Repository content is attacker-controllable in
the case of a cloned hostile repository. Terminal escape sequences that survive
rendering and alter terminal state, spoof UI chrome, or affect the clipboard
integration are in scope.

### Out of scope

- Vulnerabilities in `git` itself — report those to the git project.
- Vulnerabilities in Go module dependencies with no gito-specific exploit path;
  Dependabot already tracks these (`.github/dependabot.yml`). A dependency
  advisory that *is* reachable through gito is in scope, so do report it.
- Anything requiring an attacker to already have write access to your shell,
  your `PATH`, your `~/.gitconfig`, or the gito binary itself.
- Destructive-but-intended behaviour. gito performs real git operations,
  including ones that lose work (branch delete, stash drop, hard restore).
  Confirmation prompts are a UX guard, not a security boundary. A *missing*
  confirmation on a destructive action is a valid bug report — please file it as
  a normal issue rather than a security advisory.
