package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// logFormat emits one commit per line as NUL-separated fields; %s (subject)
// never contains a newline, so lines map 1:1 to commits.
const logFormat = "--pretty=format:%H%x00%h%x00%ad%x00%s%x00%an"

// logArgs builds a `git log` invocation. Paths are pathspecs and always go
// after "--", so a path that looks like an option or a ref is never parsed
// as one.
func logArgs(extra []string, paths []string) []string {
	args := append([]string{"log", logFormat, "--date=short"}, extra...)
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	return args
}

func parseLogLine(line string) (CommitEntry, bool) {
	parts := strings.SplitN(line, "\x00", 5)
	if len(parts) < 5 {
		return CommitEntry{}, false
	}
	return CommitEntry{
		Hash:    parts[0],
		Short:   parts[1],
		Date:    parts[2],
		Subject: parts[3],
		Author:  parts[4],
	}, true
}

// GetLogEntries returns the n most recent commits reachable from HEAD,
// optionally limited to commits touching paths.
func GetLogEntries(n int, paths ...string) ([]CommitEntry, error) {
	if !HasCommits() {
		return nil, nil
	}
	out, err := exec.Command("git", logArgs([]string{fmt.Sprintf("--max-count=%d", n)}, paths)...).Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	var entries []CommitEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if e, ok := parseLogLine(line); ok {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// SearchLog walks the whole history reachable from HEAD (optionally limited
// to paths) and returns up to limit commits for which match returns true,
// newest first. git's output is streamed, and git is stopped as soon as the
// limit is reached, so searching a huge history for a common term stays cheap.
func SearchLog(match func(CommitEntry) bool, limit int, paths ...string) ([]CommitEntry, error) {
	if !HasCommits() || limit <= 0 {
		return nil, nil
	}
	cmd := exec.Command("git", logArgs(nil, paths)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}

	var found []CommitEntry
	stopped := false
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for sc.Scan() {
		e, ok := parseLogLine(sc.Text())
		if !ok || !match(e) {
			continue
		}
		found = append(found, e)
		if len(found) >= limit {
			stopped = true
			_ = cmd.Process.Kill()
			break
		}
	}
	scanErr := sc.Err()
	waitErr := cmd.Wait()
	if stopped {
		return found, nil // killed on purpose; its exit status is meaningless
	}
	if scanErr != nil {
		return found, fmt.Errorf("git log: %w", scanErr)
	}
	if waitErr != nil {
		return found, fmt.Errorf("git log: %w", waitErr)
	}
	return found, nil
}

// IsTrackedFile reports whether path (relative to the working directory) is a
// regular file tracked by git. Directories are rejected even though
// `git ls-files` would match files beneath them, and the path is matched
// literally so glob characters cannot widen it.
func IsTrackedFile(path string) bool {
	if path == "" {
		return false
	}
	if fi, err := os.Stat(path); err != nil || !fi.Mode().IsRegular() {
		return false
	}
	return exec.Command("git", "--literal-pathspecs", "ls-files", "--error-unmatch", "--", path).Run() == nil
}
