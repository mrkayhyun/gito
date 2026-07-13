package git

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// UndoableOp describes a recent HEAD-moving operation discovered from the HEAD
// reflog. "Undoing" it means returning HEAD to the state that immediately
// preceded the operation (FromHash), which is exactly what the reflog records.
//
// Ops are derived by pairing consecutive reflog entries: the entry that produced
// the current state carries the operation's description, and the position stored
// by the next (older) entry is the state to return to.
type UndoableOp struct {
	Selector    string // selector of the entry that produced this state, e.g. "HEAD@{0}"
	Description string // reflog subject, e.g. "commit: add feature"
	Kind        string // classified verb: commit, amend, reset, rebase, merge, cherry-pick, revert, checkout, pull, clone, branch, other
	FromHash    string // full hash of the state BEFORE the op (the undo target)
	FromShort   string // abbreviated FromHash
	FromSubject string // commit subject at FromHash
	ToShort     string // abbreviated hash of the state AFTER the op (current for HEAD@{0})
}

// reflogRecord is one parsed HEAD reflog entry. It is unexported because it only
// feeds the pure buildUndoOps helper, which the tests exercise directly.
type reflogRecord struct {
	full          string // %H  full hash HEAD pointed at for this entry
	short         string // %h  abbreviated hash
	selector      string // %gd e.g. HEAD@{0}
	reflogSubject string // %gs e.g. "commit: msg", "reset: moving to X"
	commitSubject string // %s  subject of the commit this entry points at
}

// classifyUndoKind maps a raw reflog subject to a short, stable verb used for
// labelling and colouring in the UI. It is pure so it can be unit tested.
func classifyUndoKind(reflogSubject string) string {
	s := strings.TrimSpace(reflogSubject)
	switch {
	case strings.HasPrefix(s, "commit (amend)"):
		return "amend"
	case strings.HasPrefix(s, "commit"):
		return "commit"
	case strings.HasPrefix(s, "reset"):
		return "reset"
	case strings.HasPrefix(s, "rebase"):
		return "rebase"
	case strings.HasPrefix(s, "merge"):
		return "merge"
	case strings.HasPrefix(s, "cherry-pick"):
		return "cherry-pick"
	case strings.HasPrefix(s, "revert"):
		return "revert"
	case strings.HasPrefix(s, "pull"):
		return "pull"
	case strings.HasPrefix(s, "clone"):
		return "clone"
	case strings.HasPrefix(s, "branch"):
		return "branch"
	case strings.HasPrefix(s, "checkout"):
		return "checkout"
	default:
		return "other"
	}
}

// buildUndoOps pairs consecutive reflog entries (newest first) into undoable
// operations. Entry i describes the operation that produced state i; undoing it
// returns HEAD to entry i+1's recorded position. The oldest entry has no
// predecessor to return to, so it yields no op. It is a pure function with no
// side effects so it can be unit tested in isolation.
func buildUndoOps(entries []reflogRecord) []UndoableOp {
	var ops []UndoableOp
	for i := 0; i+1 < len(entries); i++ {
		cur := entries[i]
		prev := entries[i+1]
		ops = append(ops, UndoableOp{
			Selector:    cur.selector,
			Description: cur.reflogSubject,
			Kind:        classifyUndoKind(cur.reflogSubject),
			FromHash:    prev.full,
			FromShort:   prev.short,
			FromSubject: prev.commitSubject,
			ToShort:     cur.short,
		})
	}
	return ops
}

// RecentUndoableOps returns up to n recent HEAD-moving operations, newest first,
// each describing a state HEAD can be safely returned to.
func RecentUndoableOps(n int) ([]UndoableOp, error) {
	if n <= 0 {
		n = 100
	}
	out, err := exec.Command("git", "reflog",
		fmt.Sprintf("--max-count=%d", n),
		"--pretty=format:%H%x00%h%x00%gd%x00%gs%x00%s",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("git reflog: %w", err)
	}
	var entries []reflogRecord
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 5)
		if len(parts) < 5 {
			continue
		}
		entries = append(entries, reflogRecord{
			full:          parts[0],
			short:         parts[1],
			selector:      parts[2],
			reflogSubject: parts[3],
			commitSubject: parts[4],
		})
	}
	return buildUndoOps(entries), nil
}

// UndoChange is a single commit that a proposed undo would add or remove from
// the current history.
type UndoChange struct {
	Short   string
	Subject string
}

// UndoPreview summarises the effect of returning HEAD to a target state.
// Removed commits are currently reachable from HEAD and would be undone;
// Restored commits are reachable from the target and would come back.
type UndoPreview struct {
	Removed  []UndoChange
	Restored []UndoChange
	NoChange bool // true when the target already equals HEAD
}

// PreviewUndo computes which commits an undo would remove from and restore to
// the current branch, without touching the repository.
func PreviewUndo(op UndoableOp) (UndoPreview, error) {
	if strings.TrimSpace(op.FromHash) == "" {
		return UndoPreview{}, fmt.Errorf("operation has no recoverable target")
	}
	headOut, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return UndoPreview{}, fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	if strings.TrimSpace(string(headOut)) == op.FromHash {
		return UndoPreview{NoChange: true}, nil
	}

	removed, err := undoRangeCommits(op.FromHash + "..HEAD")
	if err != nil {
		return UndoPreview{}, err
	}
	restored, err := undoRangeCommits("HEAD.." + op.FromHash)
	if err != nil {
		return UndoPreview{}, err
	}
	return UndoPreview{Removed: removed, Restored: restored}, nil
}

// undoRangeCommits lists commits in a revision range as (short, subject) pairs.
func undoRangeCommits(rangeSpec string) ([]UndoChange, error) {
	out, err := exec.Command("git", "log",
		"--pretty=format:%h%x00%s",
		"--end-of-options",
		rangeSpec,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", rangeSpec, err)
	}
	var changes []UndoChange
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		if len(parts) < 2 {
			continue
		}
		changes = append(changes, UndoChange{Short: parts[0], Subject: parts[1]})
	}
	return changes, nil
}

// RunUndo returns HEAD to the state preceding op by doing a hard reset to
// op.FromHash. Before rewriting anything it refuses on a dirty tracked working
// tree (untracked files are ignored, mirroring the rebase guard) and records a
// backup ref pointing at the current HEAD so the undo itself can be undone.
func RunUndo(op UndoableOp) (backupRef string, err error) {
	if strings.TrimSpace(op.FromHash) == "" {
		return "", fmt.Errorf("operation has no recoverable target")
	}

	// (1) Refuse on a dirty working tree. Untracked files are ignored because a
	// hard reset never removes them; only staged/unstaged tracked changes would
	// be lost, so those are what we guard against.
	statusOut, sErr := exec.Command("git", "status", "--porcelain", "--untracked-files=no").Output()
	if sErr != nil {
		return "", fmt.Errorf("git status: %w", sErr)
	}
	if strings.TrimSpace(string(statusOut)) != "" {
		return "", fmt.Errorf("commit or stash your changes before undoing")
	}

	// (2) Capture the current HEAD and create a backup ref so the undo is itself
	// recoverable. refs/gito/* is not reflogged by default, so a timestamp plus a
	// short random suffix keeps two undos in the same second from colliding.
	headOut, hErr := exec.Command("git", "rev-parse", "HEAD").Output()
	if hErr != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", hErr)
	}
	origHead := strings.TrimSpace(string(headOut))
	if origHead == op.FromHash {
		return "", fmt.Errorf("HEAD is already at %s; nothing to undo", op.FromShort)
	}
	backupRef = fmt.Sprintf("refs/gito/undo-backup/%d-%s", time.Now().Unix(), undoRandomHex(4))
	if out, uErr := exec.Command("git", "update-ref", "--end-of-options", backupRef, origHead).CombinedOutput(); uErr != nil {
		return "", fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}

	// (3) Return HEAD to the pre-operation state. The backup ref above means the
	// original tip is always reachable even after this hard reset.
	if out, rErr := exec.Command("git", "reset", "--hard", "--end-of-options", op.FromHash).CombinedOutput(); rErr != nil {
		return "", fmt.Errorf("reset failed, repository unchanged (backup ref %s): %s",
			backupRef, strings.TrimSpace(string(out)))
	}
	return backupRef, nil
}

// undoRandomHex returns n bytes of cryptographically random data as a hex
// string, used to disambiguate backup ref names. It falls back to a nanosecond
// timestamp if the system RNG is unavailable so a non-empty suffix is always
// produced. It is named distinctly to avoid colliding with sibling helpers.
func undoRandomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
