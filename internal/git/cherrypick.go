package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// CherryPickCommit holds a commit from another branch that can be cherry-picked.
type CherryPickCommit struct {
	Hash    string // full 40-char hash
	Short   string // abbreviated hash
	Subject string // first line of the commit message
	Author  string // author name
	Date    string // YYYY-MM-DD
}

// GetCherryPickCandidates returns commits on sourceBranch that are NOT on the
// current branch. This is equivalent to `git log HEAD..<source> --reverse`.
// Commits are returned oldest-first.
func GetCherryPickCandidates(sourceBranch string, maxCount int) ([]CherryPickCommit, error) {
	if err := ValidateRefName(sourceBranch); err != nil {
		return nil, err
	}
	if maxCount <= 0 {
		maxCount = 50
	}
	out, err := exec.Command("git", "log",
		fmt.Sprintf("--max-count=%d", maxCount),
		"--pretty=format:%H%x00%h%x00%s%x00%an%x00%ad",
		"--date=short",
		"--end-of-options",
		"HEAD.."+sourceBranch,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("git log HEAD..%s: %w", sourceBranch, err)
	}
	var commits []CherryPickCommit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 5)
		if len(parts) < 5 {
			continue
		}
		commits = append(commits, CherryPickCommit{
			Hash:    parts[0],
			Short:   parts[1],
			Subject: parts[2],
			Author:  parts[3],
			Date:    parts[4],
		})
	}
	// git log returns newest first; reverse to get oldest first.
	for i, j := 0, len(commits)-1; i < j; i, j = i+1, j-1 {
		commits[i], commits[j] = commits[j], commits[i]
	}
	return commits, nil
}

// RunCherryPick cherry-picks one or more commits (by hash) onto the current
// branch. If a conflict occurs it auto-aborts and returns an error.
// Commits should be provided in the order they should be applied (oldest first).
func RunCherryPick(hashes []string) error {
	if len(hashes) == 0 {
		return fmt.Errorf("no commits to cherry-pick")
	}

	// Refuse on dirty working tree (tracked files only).
	statusOut, sErr := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output()
	if sErr != nil {
		return fmt.Errorf("git status: %w", sErr)
	}
	if strings.TrimSpace(string(statusOut)) != "" {
		return fmt.Errorf("commit or stash your changes before cherry-picking")
	}

	args := []string{"cherry-pick", "--end-of-options"}
	args = append(args, hashes...)
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		// Auto-abort on conflict.
		abortOut, abortErr := exec.Command("git", "cherry-pick", "--abort").CombinedOutput()
		if abortErr != nil {
			return fmt.Errorf("cherry-pick failed and could not be aborted: %s; abort: %s",
				strings.TrimSpace(string(out)),
				strings.TrimSpace(string(abortOut)))
		}
		return fmt.Errorf("cherry-pick failed (auto-aborted, no changes applied): %s",
			strings.TrimSpace(string(out)))
	}
	return nil
}

// GetLocalBranches returns only local branch names (no remotes).
func GetLocalBranches() ([]string, string, error) {
	out, err := exec.Command("git", "branch", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, "", fmt.Errorf("git branch: %w", err)
	}
	currentOut, _ := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	current := strings.TrimSpace(string(currentOut))

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l := strings.TrimSpace(line); l != "" {
			branches = append(branches, l)
		}
	}
	return branches, current, nil
}
