package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrDetachedHead is returned when an operation needs a current branch but
// HEAD is detached.
var ErrDetachedHead = errors.New("HEAD is detached")

// CurrentBranch returns the short name of the checked-out branch, or
// ErrDetachedHead when HEAD does not point at a branch.
func CurrentBranch() (string, error) {
	out, err := exec.Command("git", "symbolic-ref", "--quiet", "--short", "HEAD").Output()
	if err != nil {
		return "", ErrDetachedHead
	}
	return strings.TrimSpace(string(out)), nil
}

// Upstream returns the current branch's upstream (e.g. "origin/main") and
// whether one is configured.
func Upstream() (string, bool) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}").Output()
	if err != nil {
		return "", false
	}
	up := strings.TrimSpace(string(out))
	return up, up != ""
}

// Push publishes the current branch. It never forces: a rejected
// (non-fast-forward) push fails with git's own message.
//
// With an upstream configured it runs a plain `git push`, honoring the
// user's push configuration. Without one it pushes the branch to remote
// under the same name and records it as the upstream (`push -u`).
func Push(remote string) (string, error) {
	branch, err := CurrentBranch()
	if err != nil {
		return "", err
	}
	var args []string
	if _, ok := Upstream(); ok {
		args = []string{"push"}
	} else {
		if strings.TrimSpace(remote) == "" {
			return "", errors.New("no remote given for a branch without upstream")
		}
		// --end-of-options keeps a remote or branch that starts with '-'
		// from being parsed as an option.
		args = []string{"push", "--set-upstream", "--end-of-options", remote, branch}
	}
	return runSync(args)
}

// Pull fast-forwards the current branch to its upstream. --ff-only means it
// never creates a merge commit and never rewrites local commits: diverged
// history fails with git's message, leaving the branch untouched.
func Pull() (string, error) {
	if _, err := CurrentBranch(); err != nil {
		return "", err
	}
	if _, ok := Upstream(); !ok {
		return "", errors.New("no upstream configured for the current branch")
	}
	return runSync([]string{"pull", "--ff-only"})
}

func runSync(args []string) (string, error) {
	cmd := exec.Command("git", args...)
	// Never block on a credential prompt: the TUI owns the terminal, so git
	// could not read the answer anyway. Credential helpers and SSH agents
	// still work.
	cmd.Env = append(cmd.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	s := stripHints(string(out))
	if err != nil {
		if s == "" {
			s = err.Error()
		}
		return "", fmt.Errorf("%s", s)
	}
	if s == "" {
		s = "(done)"
	}
	return s, nil
}

// stripHints drops git's multi-line "hint:" advice so the actual error (e.g.
// "fatal: Not possible to fast-forward, aborting.") fits in the status line.
func stripHints(out string) string {
	var kept []string
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "hint:") {
			kept = append(kept, line)
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}
