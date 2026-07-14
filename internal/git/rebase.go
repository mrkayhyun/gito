package git

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// RebaseAction is the operation applied to a commit during an interactive rebase.
type RebaseAction int

const (
	// ActionPick keeps the commit as-is. It is the zero value so an unset step is
	// always a safe no-op keep.
	ActionPick RebaseAction = iota
	// ActionReword keeps the commit but replaces its message.
	ActionReword
	// ActionSquash melds the commit into the previous one, combining messages.
	ActionSquash
	// ActionFixup melds the commit into the previous one, discarding its message.
	ActionFixup
	// ActionDrop removes the commit entirely.
	ActionDrop
)

// String returns the git-todo verb for the action.
func (a RebaseAction) String() string {
	switch a {
	case ActionPick:
		return "pick"
	case ActionReword:
		return "reword"
	case ActionSquash:
		return "squash"
	case ActionFixup:
		return "fixup"
	case ActionDrop:
		return "drop"
	default:
		return "pick"
	}
}

// RebaseCommit describes a single commit eligible for rebasing.
type RebaseCommit struct {
	Hash    string // full 40-char hash
	Short   string // abbreviated hash
	Subject string // first line of the commit message
	Author  string // author name
	Date    string // YYYY-MM-DD
}

// RebaseStep is a single user-chosen operation. NewMessage is used only for
// ActionReword.
type RebaseStep struct {
	Hash       string
	Action     RebaseAction
	NewMessage string
}

// RebasePlan discovers the range of commits that can be safely rebased.
//
// When the current branch has an upstream, the range is merge-base(@{upstream},
// HEAD)..HEAD. Otherwise it falls back to the most recent commits while always
// leaving the root commit untouched so a parent for the rebase base exists.
//
// Commits are returned in git-rebase-todo order (oldest first).
func RebasePlan() (commits []RebaseCommit, base string, hasUpstream bool, err error) {
	if _, uErr := exec.Command("git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}").Output(); uErr == nil {
		hasUpstream = true
	}

	if hasUpstream {
		out, mErr := exec.Command("git", "merge-base", "@{upstream}", "HEAD").Output()
		if mErr != nil {
			return nil, "", false, fmt.Errorf("git merge-base: %w", mErr)
		}
		base = strings.TrimSpace(string(out))
		commits, err = rebaseLogRange(base + "..HEAD")
		if err != nil {
			return nil, "", false, err
		}
		return commits, base, true, nil
	}

	// No upstream: fall back to the last N commits, always keeping the root.
	countOut, cErr := exec.Command("git", "rev-list", "--count", "HEAD").Output()
	if cErr != nil {
		return nil, "", false, fmt.Errorf("git rev-list --count: %w", cErr)
	}
	total, pErr := strconv.Atoi(strings.TrimSpace(string(countOut)))
	if pErr != nil {
		return nil, "", false, fmt.Errorf("parse commit count: %w", pErr)
	}
	n := total - 1
	if n > 20 {
		n = 20
	}
	if n <= 0 {
		// Only the root commit exists: nothing to tidy.
		return nil, "", false, nil
	}

	baseOut, bErr := exec.Command("git", "rev-parse", fmt.Sprintf("HEAD~%d", n)).Output()
	if bErr != nil {
		return nil, "", false, fmt.Errorf("git rev-parse: %w", bErr)
	}
	base = strings.TrimSpace(string(baseOut))

	commits, err = rebaseLogEntries(n)
	if err != nil {
		return nil, "", false, err
	}
	return commits, base, false, nil
}

// rebaseLogRange returns commits for a revision range (e.g. "base..HEAD"),
// reversed into oldest-first order.
func rebaseLogRange(rangeSpec string) ([]RebaseCommit, error) {
	out, err := exec.Command("git", "log",
		"--pretty=format:%H%x00%h%x00%s%x00%an%x00%ad",
		"--date=short",
		"--end-of-options",
		rangeSpec,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return parseRebaseCommits(out), nil
}

// rebaseLogEntries returns the most recent n commits, reversed into oldest-first
// order.
func rebaseLogEntries(n int) ([]RebaseCommit, error) {
	out, err := exec.Command("git", "log",
		fmt.Sprintf("-n%d", n),
		"--pretty=format:%H%x00%h%x00%s%x00%an%x00%ad",
		"--date=short",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	return parseRebaseCommits(out), nil
}

// parseRebaseCommits parses NUL-delimited git log output (newest first) and
// returns the commits in oldest-first order.
func parseRebaseCommits(out []byte) []RebaseCommit {
	var commits []RebaseCommit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 5)
		if len(parts) < 5 {
			continue
		}
		commits = append(commits, RebaseCommit{
			Hash:    parts[0],
			Short:   parts[1],
			Subject: parts[2],
			Author:  parts[3],
			Date:    parts[4],
		})
	}
	// git log is newest-first; rebase todo order is oldest-first.
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits
}

// BuildRebaseTodo builds a git-rebase-todo file from steps (in oldest-first
// order). rewordMsgPaths is a parallel slice giving the temp message-file path
// for each ActionReword step (empty otherwise).
//
// It is a pure function with no file or exec side effects so it can be unit
// tested in isolation.
func BuildRebaseTodo(steps []RebaseStep, rewordMsgPaths []string) (string, error) {
	if len(steps) == 0 {
		return "", fmt.Errorf("no rebase steps provided")
	}

	allDrop := true
	firstKeptSeen := false
	var lines []string
	for i, step := range steps {
		if step.Action != ActionDrop {
			allDrop = false
			if !firstKeptSeen {
				if step.Action == ActionSquash || step.Action == ActionFixup {
					return "", fmt.Errorf("the oldest kept commit cannot be squash/fixup")
				}
				firstKeptSeen = true
			}
		}

		switch step.Action {
		case ActionPick:
			lines = append(lines, "pick "+step.Hash)
		case ActionReword:
			var path string
			if i < len(rewordMsgPaths) {
				path = rewordMsgPaths[i]
			}
			if path == "" {
				return "", fmt.Errorf("reword step for %s is missing a message file path", step.Hash)
			}
			if strings.TrimSpace(step.NewMessage) == "" {
				return "", fmt.Errorf("reword step for %s has an empty message", step.Hash)
			}
			lines = append(lines, "pick "+step.Hash)
			lines = append(lines, "exec git commit --amend -F "+shellSingleQuote(path))
		case ActionSquash:
			lines = append(lines, "squash "+step.Hash)
		case ActionFixup:
			lines = append(lines, "fixup "+step.Hash)
		case ActionDrop:
			lines = append(lines, "drop "+step.Hash)
		default:
			lines = append(lines, "pick "+step.Hash)
		}
	}

	if allDrop {
		return "", fmt.Errorf("cannot drop all commits")
	}

	return strings.Join(lines, "\n") + "\n", nil
}

// RunInteractiveRebase drives a fully non-interactive interactive rebase from
// base using the given steps (oldest first). It returns the name of a backup
// ref pointing at the original HEAD so the rebase can always be recovered.
//
// The rebase is driven entirely without an editor: GIT_SEQUENCE_EDITOR copies
// our generated todo into place, GIT_EDITOR=true accepts default squash
// messages, and reword is handled by 'exec git commit --amend -F <file>' lines.
func RunInteractiveRebase(base string, steps []RebaseStep) (backupRef string, err error) {
	// (1) Refuse on a dirty working tree. Untracked files are ignored because a
	// rebase never touches them; only staged/unstaged tracked changes would be
	// clobbered, so those are what we guard against.
	statusOut, sErr := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output()
	if sErr != nil {
		return "", fmt.Errorf("git status: %w", sErr)
	}
	if strings.TrimSpace(string(statusOut)) != "" {
		return "", fmt.Errorf("commit or stash your changes before rebasing")
	}

	// (2) Capture original HEAD and create a backup ref.
	headOut, hErr := exec.Command("git", "rev-parse", "HEAD").Output()
	if hErr != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", hErr)
	}
	origHead := strings.TrimSpace(string(headOut))
	// A unix-second timestamp keeps the ref name human-readable, and a short
	// random suffix prevents two rebases in the same second from clobbering an
	// earlier backup (refs/gito/* is not reflogged by default).
	backupRef = fmt.Sprintf("refs/gito/rebase-backup/%d-%s", time.Now().Unix(), randomHex(4))
	if out, uErr := exec.Command("git", "update-ref", "--end-of-options", backupRef, origHead).CombinedOutput(); uErr != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}

	// (3) Write reword message files and build the parallel path slice.
	var tempFiles []string
	cleanup := func() {
		for _, f := range tempFiles {
			_ = os.Remove(f)
		}
	}
	rewordMsgPaths := make([]string, len(steps))
	for i, step := range steps {
		if step.Action != ActionReword {
			continue
		}
		f, cErr := os.CreateTemp("", "gito-reword-*.txt")
		if cErr != nil {
			cleanup()
			return "", fmt.Errorf("create reword message file: %w", cErr)
		}
		if _, wErr := f.WriteString(step.NewMessage); wErr != nil {
			_ = f.Close()
			cleanup()
			return "", fmt.Errorf("write reword message file: %w", wErr)
		}
		_ = f.Close()
		tempFiles = append(tempFiles, f.Name())
		rewordMsgPaths[i] = f.Name()
	}

	// (4) Build the todo (validates the plan before any rebase is attempted).
	todo, bErr := BuildRebaseTodo(steps, rewordMsgPaths)
	if bErr != nil {
		cleanup()
		return "", bErr
	}

	// (5) Write the todo to a temp file.
	todoFile, tErr := os.CreateTemp("", "gito-rebase-todo-*.txt")
	if tErr != nil {
		cleanup()
		return "", fmt.Errorf("create rebase todo file: %w", tErr)
	}
	todoPath := todoFile.Name()
	tempFiles = append(tempFiles, todoPath)
	if _, wErr := todoFile.WriteString(todo); wErr != nil {
		_ = todoFile.Close()
		cleanup()
		return "", fmt.Errorf("write rebase todo file: %w", wErr)
	}
	_ = todoFile.Close()

	// (6) Build and run the rebase non-interactively.
	cmd := exec.Command("git", "rebase", "-i", "--end-of-options", base)
	cmd.Env = append(os.Environ(),
		"GIT_SEQUENCE_EDITOR=cp "+shellSingleQuote(todoPath),
		"GIT_EDITOR=true",
		"GIT_TERMINAL_PROMPT=0",
	)
	out, runErr := cmd.CombinedOutput()
	if runErr != nil {
		// (8) Restore the repo, but report the state honestly. `git rebase
		// --abort` exits non-zero both when it fails to unwind a stuck rebase
		// AND when there is nothing to abort at all -- which is exactly what
		// happens when `git rebase -i` fails at launch (an invalid base, or a
		// failed GIT_SEQUENCE_EDITOR copy) leaving HEAD fully intact. So the
		// abort exit code alone cannot be trusted; disambiguate by inspecting
		// the real rebase state and HEAD.
		if !rebaseStateExists() {
			// No rebase state was ever created: the rebase failed before it
			// began, HEAD is untouched, and there is nothing to abort. Don't
			// send the user chasing an in-progress rebase that doesn't exist.
			cleanup()
			return "", fmt.Errorf("rebase failed and nothing was changed (backup ref %s): %s",
				backupRef, strings.TrimSpace(string(out)))
		}
		// A rebase really is in progress; try to unwind it.
		abortOut, abortErr := exec.Command("git", "rebase", "--abort").CombinedOutput()
		cleanup()
		if abortErr != nil && rebaseStateExists() {
			// Abort could not clean up: a rebase is genuinely still in
			// progress and the user needs the backup ref to recover.
			return "", fmt.Errorf("rebase failed and could not be aborted automatically; a rebase is still in progress. Recover with 'git rebase --abort' or 'git reset --hard %s' (backup ref). git rebase: %s; git rebase --abort: %s",
				backupRef,
				strings.TrimSpace(string(out)),
				strings.TrimSpace(string(abortOut)))
		}
		return "", fmt.Errorf("rebase failed and the repository was restored (backup ref %s): %s",
			backupRef, strings.TrimSpace(string(out)))
	}

	// (9) Success.
	cleanup()
	return backupRef, nil
}

// rebaseStateExists reports whether git currently has a rebase in progress,
// detected by the presence of the rebase-merge (interactive/merge backend) or
// rebase-apply (am backend) state directory under the git dir. This is the
// authoritative signal for "is a rebase actually underway", independent of any
// command's exit code.
func rebaseStateExists() bool {
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		out, err := exec.Command("git", "rev-parse", "--git-path", name).Output()
		if err != nil {
			continue
		}
		path := strings.TrimSpace(string(out))
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

// AbortRebase aborts an in-progress rebase.
func AbortRebase() error {
	out, err := exec.Command("git", "rebase", "--abort").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// randomHex returns n bytes of cryptographically random data as a hex string.
// It falls back to a nanosecond timestamp if the system RNG is unavailable so a
// caller always gets a non-empty, high-entropy suffix.
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}

// shellSingleQuote wraps s in single quotes, escaping any embedded single
// quotes, so it is safe to embed in a shell command (git runs GIT_SEQUENCE_EDITOR
// and exec lines through the shell). Temp paths won't contain quotes, but this
// keeps the helper robust.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
