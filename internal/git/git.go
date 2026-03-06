package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func GetStatus() (string, error) {
	out, err := exec.Command("git", "status", "--short").Output()
	if err != nil {
		return "", fmt.Errorf("git status: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CommitEntry holds parsed fields for a single git commit.
type CommitEntry struct {
	Hash    string // full 40-char hash
	Short   string // 7-char abbreviated hash
	Date    string // YYYY-MM-DD
	Subject string // first line of commit message
	Author  string // author name
}

func GetLogEntries(n int) ([]CommitEntry, error) {
	out, err := exec.Command("git", "log",
		fmt.Sprintf("--max-count=%d", n),
		"--pretty=format:%H%x00%h%x00%ad%x00%s%x00%an",
		"--date=short",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	var entries []CommitEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 5)
		if len(parts) < 5 {
			continue
		}
		entries = append(entries, CommitEntry{
			Hash:    parts[0],
			Short:   parts[1],
			Date:    parts[2],
			Subject: parts[3],
			Author:  parts[4],
		})
	}
	return entries, nil
}

func GetCommitDetail(hash string) (string, error) {
	out, err := exec.Command("git", "show", "--color=always", "--stat", "-p", hash).Output()
	if err != nil {
		return "", fmt.Errorf("git show: %w", err)
	}
	return string(out), nil
}

func GetBranches() ([]string, string, error) {
	out, err := exec.Command("git", "branch", "--all").Output()
	if err != nil {
		return nil, "", fmt.Errorf("git branch: %w", err)
	}
	var branches []string
	var current string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasSuffix(line, "/HEAD") {
			continue
		}
		if strings.HasPrefix(line, "* ") {
			current = strings.TrimPrefix(line, "* ")
			branches = append(branches, current)
		} else {
			branches = append(branches, line)
		}
	}
	return branches, current, nil
}

func Commit(message string) error {
	out, err := exec.Command("git", "commit", "-m", message).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func SwitchBranch(name string) error {
	name = strings.TrimPrefix(name, "remotes/origin/")
	out, err := exec.Command("git", "checkout", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}
