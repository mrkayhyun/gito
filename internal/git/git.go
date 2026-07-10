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

// IsRepo reports whether the current working directory is inside a git work tree.
func IsRepo() bool {
	out, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "true"
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
	out, err := exec.Command("git", "show", "--color=always", "--stat", "-p", "--end-of-options", hash).Output()
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

// CommitAmend amends the most recent commit with a new message.
func CommitAmend(message string) error {
	out, err := exec.Command("git", "commit", "--amend", "-m", message).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// GetLastCommitSubject returns the subject line of HEAD, or "" if none.
func GetLastCommitSubject() string {
	out, err := exec.Command("git", "log", "-1", "--pretty=format:%s").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func SwitchBranch(name string) error {
	name = strings.TrimPrefix(name, "remotes/origin/")
	out, err := exec.Command("git", "checkout", "--end-of-options", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// CreateBranch creates a new branch. When checkout is true it also switches to it.
func CreateBranch(name string, checkout bool) error {
	if err := ValidateRefName(name); err != nil {
		return err
	}
	var out []byte
	var err error
	if checkout {
		// 'git checkout -b' consumes the next token as the new branch name, so a
		// '--end-of-options' guard cannot precede it. The name is already validated
		// by ValidateRefName above (it cannot start with '-'), so passing it as a
		// validated positional is safe against option injection.
		out, err = exec.Command("git", "checkout", "-b", name).CombinedOutput()
	} else {
		out, err = exec.Command("git", "branch", "--end-of-options", name).CombinedOutput()
	}
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteBranch deletes a local branch. force uses -D (discards unmerged commits).
func DeleteBranch(name string, force bool) error {
	name = strings.TrimPrefix(name, "remotes/origin/")
	flag := "-d"
	if force {
		flag = "-D"
	}
	out, err := exec.Command("git", "branch", flag, "--end-of-options", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// RenameBranch renames a branch (old -> new).
func RenameBranch(oldName, newName string) error {
	if err := ValidateRefName(newName); err != nil {
		return err
	}
	oldName = strings.TrimPrefix(oldName, "remotes/origin/")
	out, err := exec.Command("git", "branch", "-m", "--end-of-options", oldName, newName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// IsRemoteBranch reports whether the branch name refers to a remote-tracking branch.
func IsRemoteBranch(name string) bool {
	return strings.HasPrefix(name, "remotes/")
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
	out, err := exec.Command("git", "stash", "pop", "--end-of-options", ref).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func StashApply(ref string) error {
	out, err := exec.Command("git", "stash", "apply", "--end-of-options", ref).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func StashDrop(ref string) error {
	out, err := exec.Command("git", "stash", "drop", "--end-of-options", ref).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func StashShow(ref string) (string, error) {
	out, err := exec.Command("git", "stash", "show", "--color=always", "-p", "--end-of-options", ref).Output()
	if err != nil {
		return "", fmt.Errorf("git stash show: %w", err)
	}
	return string(out), nil
}

// ── Tag ───────────────────────────────────────────────────────────────────────

// TagEntry holds parsed fields for a single git tag.
type TagEntry struct {
	Name       string // tag name, e.g. "v1.2.0"
	Annotated  bool   // true for annotated tags, false for lightweight
	Date       string // YYYY-MM-DD of the tagged commit (author date)
	Subject    string // annotation message (annotated) or commit subject (lightweight)
	TargetHash string // abbreviated hash of the commit the tag points to
}

// GetTags returns all tags sorted by version (newest first).
func GetTags() ([]TagEntry, error) {
	// %(objecttype) is "tag" for annotated tags, "commit" for lightweight.
	// %(contents:subject) is the tag message for annotated tags (empty for lightweight),
	// so we fall back to the commit subject.
	format := strings.Join([]string{
		"%(refname:short)",
		"%(objecttype)",
		"%(*authordate:short)%(authordate:short)",
		"%(contents:subject)",
		"%(*objectname:short)%(objectname:short)",
		"%(subject)",
	}, "%00")

	out, err := exec.Command("git", "tag",
		"--sort=-version:refname",
		"--format="+format,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("git tag: %w", err)
	}

	var entries []TagEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 6)
		if len(parts) < 6 {
			continue
		}
		subject := parts[3]
		if subject == "" {
			subject = parts[5] // fall back to commit subject for lightweight tags
		}
		entries = append(entries, TagEntry{
			Name:       parts[0],
			Annotated:  parts[1] == "tag",
			Date:       parts[2],
			Subject:    subject,
			TargetHash: parts[4],
		})
	}
	return entries, nil
}

// CreateTag creates a tag at the given ref (defaults to HEAD when empty).
// If message is non-empty an annotated tag is created, otherwise a lightweight tag.
func CreateTag(name, message, ref string) error {
	if err := ValidateRefName(name); err != nil {
		return err
	}
	args := []string{"tag"}
	if strings.TrimSpace(message) != "" {
		args = append(args, "-a", "-m", message)
	}
	args = append(args, "--end-of-options", name)
	if strings.TrimSpace(ref) != "" {
		args = append(args, ref)
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteTag deletes a local tag.
func DeleteTag(name string) error {
	out, err := exec.Command("git", "tag", "-d", "--end-of-options", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ShowTag returns the annotation and diff for a tag.
func ShowTag(name string) (string, error) {
	out, err := exec.Command("git", "show", "--color=always", "--stat", "-p", "--end-of-options", name).Output()
	if err != nil {
		return "", fmt.Errorf("git show: %w", err)
	}
	return string(out), nil
}

// PushTag pushes a single tag to the given remote (defaults to "origin").
func PushTag(name, remote string) error {
	if strings.TrimSpace(remote) == "" {
		remote = "origin"
	}
	out, err := exec.Command("git", "push", "--end-of-options", remote, name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// DeleteRemoteTag deletes a tag on the given remote (defaults to "origin").
func DeleteRemoteTag(name, remote string) error {
	if strings.TrimSpace(remote) == "" {
		remote = "origin"
	}
	// Keep '--delete' before '--end-of-options' so it is still parsed as an
	// option, then guard BOTH the remote and the tag name as positionals. This
	// mirrors PushTag's 'git push --end-of-options <remote> <name>' so neither
	// operand can be interpreted as an option, even if remote selection ever
	// becomes user-driven.
	out, err := exec.Command("git", "push", "--delete", "--end-of-options", remote, name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ── Remote ──────────────────────────────────────────────────────────────────

// RemoteEntry describes a configured git remote.
type RemoteEntry struct {
	Name      string
	FetchURL  string
	PushURL   string
	Ahead     int  // commits HEAD is ahead of its upstream
	Behind    int  // commits HEAD is behind its upstream
	HasUpstrm bool // whether an upstream is configured for the current branch
}

// GetRemotes returns configured remotes with fetch/push URLs.
func GetRemotes() ([]RemoteEntry, error) {
	out, err := exec.Command("git", "remote", "-v").Output()
	if err != nil {
		return nil, fmt.Errorf("git remote: %w", err)
	}
	byName := map[string]*RemoteEntry{}
	var order []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name, url, kind := fields[0], fields[1], fields[2]
		r, ok := byName[name]
		if !ok {
			r = &RemoteEntry{Name: name}
			byName[name] = r
			order = append(order, name)
		}
		switch kind {
		case "(fetch)":
			r.FetchURL = url
		case "(push)":
			r.PushURL = url
		}
	}
	entries := make([]RemoteEntry, 0, len(order))
	for _, name := range order {
		entries = append(entries, *byName[name])
	}
	return entries, nil
}

// GetAheadBehind returns how many commits the current branch is ahead/behind its
// upstream. hasUpstream is false when no upstream tracking branch is configured.
func GetAheadBehind() (ahead, behind int, hasUpstream bool) {
	out, err := exec.Command("git", "rev-list", "--left-right", "--count", "@{upstream}...HEAD").Output()
	if err != nil {
		return 0, 0, false
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) != 2 {
		return 0, 0, false
	}
	// Parse both counts; on malformed input leave the value at 0 (behaviour
	// equivalent to the previous ignore-error path for well-formed output).
	if _, err := fmt.Sscanf(fields[0], "%d", &behind); err != nil {
		behind = 0
	}
	if _, err := fmt.Sscanf(fields[1], "%d", &ahead); err != nil {
		ahead = 0
	}
	return ahead, behind, true
}

// Fetch runs `git fetch` for the given remote (or all remotes when empty).
func Fetch(remote string) (string, error) {
	args := []string{"fetch", "--prune"}
	if strings.TrimSpace(remote) == "" {
		args = append(args, "--all")
	} else {
		// Guard the remote positional with '--end-of-options' so it can never be
		// parsed as an option, mirroring every other positional-taking wrapper
		// (PushTag, DeleteRemoteTag, etc.) as defense-in-depth.
		args = append(args, "--end-of-options", remote)
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		s = "(already up to date)"
	}
	return s, nil
}

// ── Diff (ref comparison) ─────────────────────────────────────────────────────

// GetRefs returns local branches and tags usable as diff endpoints.
func GetRefs() ([]string, error) {
	var refs []string

	bout, err := exec.Command("git", "branch", "--format=%(refname:short)").Output()
	if err != nil {
		return nil, fmt.Errorf("git branch: %w", err)
	}
	for _, l := range strings.Split(strings.TrimSpace(string(bout)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			refs = append(refs, l)
		}
	}

	tout, err := exec.Command("git", "tag", "--sort=-version:refname").Output()
	if err == nil {
		for _, l := range strings.Split(strings.TrimSpace(string(tout)), "\n") {
			if l = strings.TrimSpace(l); l != "" {
				refs = append(refs, l)
			}
		}
	}
	return refs, nil
}

// GetDiffBetween returns the colored diff between two refs (base..target).
func GetDiffBetween(base, target string) (string, error) {
	out, err := exec.Command("git", "diff", "--color=always", "--stat", "-p",
		"--end-of-options", base, target).Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

// ── Reflog ──────────────────────────────────────────────────────────────────

// ReflogEntry is one line of `git reflog`.
type ReflogEntry struct {
	Short    string // abbreviated hash
	Selector string // e.g. HEAD@{0}
	Action   string // e.g. "commit", "checkout: moving from ..."
	Subject  string
}

func GetReflog(n int) ([]ReflogEntry, error) {
	out, err := exec.Command("git", "reflog",
		fmt.Sprintf("--max-count=%d", n),
		"--pretty=format:%h%x00%gd%x00%gs%x00%s",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("git reflog: %w", err)
	}
	var entries []ReflogEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 4)
		if len(parts) < 4 {
			continue
		}
		entries = append(entries, ReflogEntry{
			Short:    parts[0],
			Selector: parts[1],
			Action:   parts[2],
			Subject:  parts[3],
		})
	}
	return entries, nil
}

// CreateBranchAt creates a new branch pointing at the given ref/hash.
// This is the non-destructive way to recover a commit found in the reflog.
func CreateBranchAt(name, ref string) error {
	if err := ValidateRefName(name); err != nil {
		return err
	}
	out, err := exec.Command("git", "branch", "--end-of-options", name, ref).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

// ── Blame ─────────────────────────────────────────────────────────────────────

// ListTrackedFiles returns all files tracked by git in the working tree.
func ListTrackedFiles() ([]string, error) {
	out, err := exec.Command("git", "ls-files").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var files []string
	for _, l := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			files = append(files, l)
		}
	}
	return files, nil
}

// GetBlame returns `git blame` output for a file with abbreviated commits.
func GetBlame(path string) (string, error) {
	out, err := exec.Command("git", "blame", "--date=short", "-c", "--", path).Output()
	if err != nil {
		return "", fmt.Errorf("git blame: %w", err)
	}
	return string(out), nil
}
