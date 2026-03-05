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

func GetLog(n int) (string, error) {
	out, err := exec.Command("git", "log",
		fmt.Sprintf("--max-count=%d", n),
		"--pretty=format:%C(yellow)%h%Creset %C(cyan)%ad%Creset %s %C(dim)(%an)%Creset",
		"--date=short",
	).Output()
	if err != nil {
		return "", fmt.Errorf("git log: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
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
