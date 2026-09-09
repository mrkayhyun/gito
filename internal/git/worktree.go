package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeEntry describes a single git worktree.
type WorktreeEntry struct {
	Path   string // absolute path of the worktree
	Head   string // abbreviated hash of HEAD
	Branch string // branch name (empty for detached HEAD)
	Bare   bool   // true if the worktree is bare
}

// GetWorktrees returns all configured worktrees.
func GetWorktrees() ([]WorktreeEntry, error) {
	out, err := exec.Command("git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}
	var entries []WorktreeEntry
	var current *WorktreeEntry
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			if current != nil {
				entries = append(entries, *current)
			}
			current = &WorktreeEntry{Path: strings.TrimPrefix(line, "worktree ")}
		} else if strings.HasPrefix(line, "HEAD ") && current != nil {
			h := strings.TrimPrefix(line, "HEAD ")
			if len(h) > 7 {
				current.Head = h[:7]
			} else {
				current.Head = h
			}
		} else if strings.HasPrefix(line, "branch ") && current != nil {
			branch := strings.TrimPrefix(line, "branch ")
			// Strip refs/heads/ prefix.
			branch = strings.TrimPrefix(branch, "refs/heads/")
			current.Branch = branch
		} else if line == "bare" && current != nil {
			current.Bare = true
		} else if strings.HasPrefix(line, "detached") && current != nil {
			current.Branch = "(detached)"
		}
	}
	if current != nil {
		entries = append(entries, *current)
	}
	return entries, nil
}

// AddWorktree creates a new worktree at the given path for the given branch.
// If newBranch is true, a new branch is created from HEAD.
func AddWorktree(path, branch string, newBranch bool) error {
	if err := ValidateRefName(branch); err != nil {
		return err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	var args []string
	if newBranch {
		args = []string{"worktree", "add", "-b", branch, "--end-of-options", absPath}
	} else {
		args = []string{"worktree", "add", "--end-of-options", absPath, branch}
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveWorktree removes a worktree at the given path.
// If force is true, it uses --force to remove even if dirty.
func RemoveWorktree(path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, "--", path)
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
