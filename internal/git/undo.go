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

// GetUndoInfo retrieves information about what "undo" would do.
// It looks at HEAD@{1} — the previous HEAD position — and reports it so the
// user can decide whether to proceed.
// Returns nil, nil if there is nothing to undo (e.g. only 1 reflog entry).
func GetUndoInfo() (*UndoInfo, error) {
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

	// Get the reflog action text for HEAD@{0}.
	actionOut, _ := exec.Command("git", "reflog", "-1",
		"--pretty=format:%gs").Output()
	action := strings.TrimSpace(string(actionOut))

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
	out, err := exec.Command("git", "reset", "--hard", "HEAD@{1}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git reset --hard HEAD@{1}: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
