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

// ── FileStatus ────────────────────────────────────────────────────────────────

// FileStatus represents one entry from `git status --porcelain`.
// Staged = X column, Unstaged = Y column.
type FileStatus struct {
	Staged   byte
	Unstaged byte
	Path     string
	OldPath  string // set for renames
}

func (f FileStatus) IsStaged() bool {
	return f.Staged != ' ' && f.Staged != '?'
}

func (f FileStatus) IsUnstaged() bool {
	return f.Unstaged != ' ' && f.Unstaged != '?'
}

func (f FileStatus) IsUntracked() bool {
	return f.Staged == '?' && f.Unstaged == '?'
}

func GetFileStatuses() ([]FileStatus, error) {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w", err)
	}
	var files []FileStatus
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		x, y := line[0], line[1]
		path := line[3:]
		oldPath := ""
		if strings.Contains(path, " -> ") {
			parts := strings.SplitN(path, " -> ", 2)
			oldPath, path = parts[0], parts[1]
		}
		files = append(files, FileStatus{Staged: x, Unstaged: y, Path: path, OldPath: oldPath})
	}
	return files, nil
}

func StageFile(path string) error {
	out, err := exec.Command("git", "add", "--", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func StageAll() error {
	out, err := exec.Command("git", "add", "-A").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func UnstageFile(path string) error {
	out, err := exec.Command("git", "restore", "--staged", "--", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func DiscardFile(path string) error {
	out, err := exec.Command("git", "restore", "--", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func GetFileDiff(path string, staged bool) (string, error) {
	args := []string{"diff", "--color=always"}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", path)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// ── Stash ─────────────────────────────────────────────────────────────────────

type StashEntry struct {
	Ref     string // e.g. "stash@{0}"
	Branch  string
	Subject string
}

func GetStashes() ([]StashEntry, error) {
	out, err := exec.Command("git", "stash", "list",
		"--pretty=format:%gd%x00%gs",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("git stash list: %w", err)
	}
	var entries []StashEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) < 2 {
			continue
		}
		ref := parts[0]
		subj := parts[1]
		branch := ""
		// "WIP on main: abc message"  or  "On main: message"
		if after, ok := strings.CutPrefix(subj, "WIP on "); ok {
			if idx := strings.Index(after, ":"); idx != -1 {
				branch = after[:idx]
				subj = strings.TrimSpace(after[idx+1:])
			}
		} else if after, ok := strings.CutPrefix(subj, "On "); ok {
			if idx := strings.Index(after, ":"); idx != -1 {
				branch = after[:idx]
				subj = strings.TrimSpace(after[idx+1:])
			}
		}
		entries = append(entries, StashEntry{Ref: ref, Branch: branch, Subject: subj})
	}
	return entries, nil
}

func StashPop(ref string) error {
	out, err := exec.Command("git", "stash", "pop", ref).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func StashApply(ref string) error {
	out, err := exec.Command("git", "stash", "apply", ref).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func StashDrop(ref string) error {
	out, err := exec.Command("git", "stash", "drop", ref).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func StashShow(ref string) (string, error) {
	out, err := exec.Command("git", "stash", "show", "--color=always", "-p", ref).Output()
	if err != nil {
		return "", fmt.Errorf("git stash show: %w", err)
	}
	return string(out), nil
}
