package git

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrStalePatch means the displayed diff no longer matches the repository.
var ErrStalePatch = errors.New("diff changed since preview; refresh before applying a hunk")

// DiffHunk is one complete unified diff hunk and its line in the displayed diff.
type DiffHunk struct {
	Text      string
	StartLine int
}

// FilePatch is a snapshot of a file's index or working tree diff.
// An empty Hunks slice means the diff is view-only.
type FilePatch struct {
	Path    string
	Staged  bool
	Content string
	Hunks   []DiffHunk
	root    string
	header  string
}

func GetFilePatch(path string, staged bool) (FilePatch, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return FilePatch{}, fmt.Errorf("git repository root: %w", err)
	}
	return getFilePatchAt(strings.TrimSuffix(string(out), "\n"), path, staged)
}

func getFilePatchAt(root, path string, staged bool) (FilePatch, error) {
	args := []string{"--literal-pathspecs", "diff", "--no-ext-diff", "--no-textconv", "--no-color",
		"--no-renames", "--src-prefix=a/", "--dst-prefix=b/", "--line-prefix=",
		"--unified=3", "--inter-hunk-context=0", "--diff-algorithm=myers", "--no-indent-heuristic",
		"--output-indicator-new=+", "--output-indicator-old=-", "--output-indicator-context= "}
	if staged {
		args = append(args, "--cached")
	}
	args = append(args, "--", path)
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return FilePatch{}, fmt.Errorf("git diff: %w", err)
	}
	diff := FilePatch{Path: path, Staged: staged, Content: string(out), root: root}
	lines := strings.SplitAfter(diff.Content, "\n")
	var header strings.Builder
	files := 0
	regularFile := false
	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			files++
		}
		if strings.HasPrefix(line, "@@ ") {
			diff.Hunks = append(diff.Hunks, DiffHunk{StartLine: i})
		}
		if len(diff.Hunks) == 0 {
			header.WriteString(line)
			if strings.HasPrefix(line, "index ") {
				fields := strings.Fields(line)
				regularFile = len(fields) == 3 && (fields[2] == "100644" || fields[2] == "100755")
			}
		}
	}
	diff.header = header.String()
	// New/deleted files, mode changes, symlinks, submodules and combined
	// conflict diffs cannot be safely represented as a plain text hunk here.
	if files != 1 || !regularFile || strings.Contains(diff.header, "new file mode ") || strings.Contains(diff.header, "deleted file mode ") {
		diff.Hunks = nil
	}
	for i := range diff.Hunks {
		end := len(lines)
		if i+1 < len(diff.Hunks) {
			end = diff.Hunks[i+1].StartLine
		}
		diff.Hunks[i].Text = strings.Join(lines[diff.Hunks[i].StartLine:end], "")
	}
	return diff, nil
}

// ApplyHunk changes only the index. It rechecks the preview and applies a single
// complete hunk, reversing the patch when unstaging. git apply validates context
// and updates the index under Git's index lock; the working file is untouched.
func ApplyHunk(diff FilePatch, selected int) error {
	if selected < 0 || selected >= len(diff.Hunks) || diff.root == "" {
		return fmt.Errorf("no selectable hunk")
	}
	current, err := getFilePatchAt(diff.root, diff.Path, diff.Staged)
	if err != nil {
		return err
	}
	if current.Content != diff.Content {
		return ErrStalePatch
	}
	if selected >= len(current.Hunks) {
		return fmt.Errorf("no selectable hunk")
	}
	args := []string{"apply", "--cached", "--whitespace=nowarn"}
	if diff.Staged {
		args = append(args, "--reverse")
	}
	args = append(args, "-")
	cmd := exec.Command("git", args...)
	cmd.Dir = diff.root
	cmd.Stdin = strings.NewReader(current.header + current.Hunks[selected].Text)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git apply hunk: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
