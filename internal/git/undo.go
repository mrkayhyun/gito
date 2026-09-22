package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// UndoInfo describes the last operation that can be undone.
type UndoInfo struct {
	// Current HEAD info.
	CurrentHash    string
	CurrentSubject string

	// Previous HEAD info (what we'd reset to).
	PreviousHash    string
	PreviousSubject string

	// The reflog action description (e.g. "commit", "merge", "rebase (finish)").
	Action string
}

// undoAction refuses checkouts: resetting to the previous HEAD after a branch
// switch would move the destination branch, not switch back to the source.
// Check at execution time as well as preview time since another Git command
// may have run while the confirmation screen was open.
func undoAction() (string, error) {
	out, err := exec.Command("git", "reflog", "-1", "--pretty=format:%gs").Output()
	if err != nil {
		return "", fmt.Errorf("git reflog: %w", err)
	}
	action := strings.TrimSpace(string(out))
	if strings.HasPrefix(action, "checkout:") {
		return "", fmt.Errorf("cannot undo a branch switch with reset; use git switch - to switch back, or gito reflog to select a recovery point")
	}
	return action, nil
}

// GetUndoInfo retrieves information about what "undo" would do.
// It looks at HEAD@{1} — the previous HEAD position — and reports it so the
// user can decide whether to proceed.
// Returns nil, nil if there is nothing to undo (e.g. only 1 reflog entry).
func GetUndoInfo() (*UndoInfo, error) {
	action, err := undoAction()
	if err != nil {
		return nil, err
	}
	// Current HEAD.
	curOut, err := exec.Command("git", "log", "-1",
		"--pretty=format:%H%x00%s").Output()
	if err != nil {
		return nil, fmt.Errorf("git log HEAD: %w", err)
	}
	curParts := strings.SplitN(strings.TrimSpace(string(curOut)), "\x00", 2)
	if len(curParts) < 2 {
		return nil, fmt.Errorf("could not parse current HEAD")
	}

	// Previous HEAD from reflog (HEAD@{1}).
	prevOut, err := exec.Command("git", "log", "-1",
		"--pretty=format:%H%x00%s",
		"--end-of-options", "HEAD@{1}").Output()
	if err != nil {
		return nil, nil // nothing to undo
	}
	prevParts := strings.SplitN(strings.TrimSpace(string(prevOut)), "\x00", 2)
	if len(prevParts) < 2 {
		return nil, nil
	}

	// If current == previous, nothing to undo.
	if curParts[0] == prevParts[0] {
		return nil, nil
	}

	return &UndoInfo{
		CurrentHash:     curParts[0],
		CurrentSubject:  curParts[1],
		PreviousHash:    prevParts[0],
		PreviousSubject: prevParts[1],
		Action:          action,
	}, nil
}

// RunUndo resets HEAD to HEAD@{1} using --soft so work is preserved in the
// index. This undoes the last commit/merge/rebase without destroying changes.
func RunUndo() error {
	if _, err := undoAction(); err != nil {
		return err
	}
	// Refuse on dirty tracked files (uncommitted staged changes are OK since
	// --soft only moves HEAD; but unstaged changes could conflict).
	statusOut, sErr := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output()
	if sErr != nil {
		return fmt.Errorf("git status: %w", sErr)
	}
	// Allow staged-only changes (X column set, Y column ' ') but reject unstaged modifications.
	for _, line := range strings.Split(string(statusOut), "\n") {
		if len(line) < 2 {
			continue
		}
		y := line[1]
		if y != ' ' && y != '?' {
			return fmt.Errorf("unstaged changes detected — commit or stash before undoing")
		}
	}

	out, err := exec.Command("git", "reset", "--soft", "HEAD@{1}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset --soft HEAD@{1}: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

// RunUndoHard resets HEAD to HEAD@{1} using --hard, discarding all changes.
func RunUndoHard() error {
	if _, err := undoAction(); err != nil {
		return err
	}
	out, err := exec.Command("git", "reset", "--hard", "HEAD@{1}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset --hard HEAD@{1}: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
