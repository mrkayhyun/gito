package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetStagedDiff returns the exact patch currently staged for commit.
// Unstaged and untracked changes are intentionally excluded.
func GetStagedDiff() (string, error) {
	out, err := exec.Command("git", "diff", "--cached", "--no-ext-diff", "--no-color", "--binary").Output()
	if err != nil {
		return "", fmt.Errorf("git diff --cached: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
