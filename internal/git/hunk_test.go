package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func hunkFixture() (string, string, string, string) {
	var original, changed, firstOnly, secondOnly strings.Builder
	for i := 1; i <= 30; i++ {
		line := fmt.Sprintf("line %02d\n", i)
		original.WriteString(line)
		if i == 2 || i == 25 {
			changed.WriteString(fmt.Sprintf("changed %02d\n", i))
		} else {
			changed.WriteString(line)
		}
		if i == 2 {
			firstOnly.WriteString("changed 02\n")
		} else {
			firstOnly.WriteString(line)
		}
		if i == 25 {
			secondOnly.WriteString("changed 25\n")
		} else {
			secondOnly.WriteString(line)
		}
	}
	return original.String(), changed.String(), firstOnly.String(), secondOnly.String()
}

func TestApplyHunkChangesOnlySelectedIndexHunk(t *testing.T) {
	for _, staged := range []bool{false, true} {
		t.Run(fmt.Sprintf("staged=%v", staged), func(t *testing.T) {
			cleanup := setupRepo(t)
			defer cleanup()
			original, changed, firstOnly, secondOnly := hunkFixture()
			path := "한글 file.txt"
			addCommit(t, path, original, "fixture")
			if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
				t.Fatal(err)
			}
			wantIndex := secondOnly
			if staged {
				run(t, ".", "add", "--", path)
				wantIndex = firstOnly
				changed += "extra unstaged work\n"
				if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			diff, err := GetFilePatch(path, staged)
			if err != nil {
				t.Fatal(err)
			}
			if len(diff.Hunks) != 2 {
				t.Fatalf("want two selectable hunks, got %d: %s", len(diff.Hunks), diff.Content)
			}
			if err := ApplyHunk(diff, 1); err != nil {
				t.Fatal(err)
			}
			index, err := exec.Command("git", "show", ":"+path).Output()
			if err != nil || string(index) != wantIndex {
				t.Fatalf("wrong index content: %q, error: %v", index, err)
			}
			working, err := os.ReadFile(path)
			if err != nil || string(working) != changed {
				t.Fatalf("working file was changed: %q, error: %v", working, err)
			}
		})
	}
}

func TestApplyHunkRejectsStaleDiff(t *testing.T) {
	for _, staged := range []bool{false, true} {
		t.Run(fmt.Sprintf("staged=%v", staged), func(t *testing.T) {
			cleanup := setupRepo(t)
			defer cleanup()
			original, changed, _, _ := hunkFixture()
			addCommit(t, "file.txt", original, "fixture")
			if err := os.WriteFile("file.txt", []byte(changed), 0o644); err != nil {
				t.Fatal(err)
			}
			if staged {
				run(t, ".", "add", "file.txt")
			}
			diff, err := GetFilePatch("file.txt", staged)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile("file.txt", []byte(changed+"external edit\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			want := original
			if staged {
				run(t, ".", "add", "file.txt")
				want = changed + "external edit\n"
			}
			if err := ApplyHunk(diff, 0); !errors.Is(err, ErrStalePatch) {
				t.Fatalf("must reject a diff changed since preview, got %v", err)
			}
			index, err := exec.Command("git", "show", ":file.txt").Output()
			if err != nil || string(index) != want {
				t.Fatalf("stale patch modified index: %q, %v", index, err)
			}
		})
	}
}

func TestApplyHunkPreservesOffsetsAndMissingFinalNewline(t *testing.T) {
	for _, staged := range []bool{false, true} {
		t.Run(fmt.Sprintf("staged=%v", staged), func(t *testing.T) {
			cleanup := setupRepo(t)
			defer cleanup()
			original, _, _, _ := hunkFixture()
			original = strings.TrimSuffix(original, "\n")
			firstOnly := "inserted line\n" + original
			secondOnly := strings.TrimSuffix(original, "line 30") + "new ending"
			changed := "inserted line\n" + secondOnly
			addCommit(t, "file.txt", original, "fixture")
			if err := os.WriteFile("file.txt", []byte(changed), 0o644); err != nil {
				t.Fatal(err)
			}
			want := secondOnly
			if staged {
				run(t, ".", "add", "file.txt")
				want = firstOnly
			}
			// User presentation settings must not change the patch format.
			run(t, ".", "config", "diff.noprefix", "true")
			run(t, ".", "config", "diff.context", "100")
			run(t, ".", "config", "diff.interHunkContext", "100")
			run(t, ".", "config", "diff.outputIndicatorNew", ">")
			diff, err := GetFilePatch("file.txt", staged)
			if err != nil {
				t.Fatal(err)
			}
			if len(diff.Hunks) != 2 {
				t.Fatalf("want two hunks: %+v", diff)
			}
			if err := ApplyHunk(diff, 1); err != nil {
				t.Fatal(err)
			}
			index, err := exec.Command("git", "show", ":file.txt").Output()
			if err != nil || string(index) != want {
				t.Fatalf("wrong index: %q, %v", index, err)
			}
		})
	}
}

func TestFilePatchUnsupportedChangesAreReadOnly(t *testing.T) {
	for _, kind := range []string{"added", "deleted", "binary", "rename"} {
		t.Run(kind, func(t *testing.T) {
			cleanup := setupRepo(t)
			defer cleanup()
			path := "README.md"
			switch kind {
			case "added":
				path = "new.txt"
				if err := os.WriteFile(path, []byte("new\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				run(t, ".", "add", path)
			case "deleted":
				run(t, ".", "rm", path)
			case "binary":
				if err := os.WriteFile(path, []byte("\x00binary\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				run(t, ".", "add", path)
			case "rename":
				run(t, ".", "mv", path, "renamed.txt")
				path = "renamed.txt"
			}
			diff, err := GetFilePatch(path, true)
			if err != nil {
				t.Fatal(err)
			}
			if len(diff.Hunks) != 0 || diff.Content == "" {
				t.Fatalf("want visible read-only diff, got %+v", diff)
			}
			if err := ApplyHunk(diff, 0); err == nil {
				t.Fatal("read-only diff was applied")
			}
		})
	}
}

func TestApplyHunkFromSubdirectoryUsesLiteralRootPath(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	original, changed, _, secondOnly := hunkFixture()
	path := "[file].txt"
	addCommit(t, path, original, "fixture")
	addCommit(t, "f.txt", original, "pathspec decoy")
	for _, name := range []string{path, "f.txt"} {
		if err := os.WriteFile(name, []byte(changed), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("sub", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir("sub"); err != nil {
		t.Fatal(err)
	}
	diff, err := GetFilePatch(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Hunks) != 2 {
		t.Fatalf("wrong path selected: %+v", diff)
	}
	if err := ApplyHunk(diff, 1); err != nil {
		t.Fatal(err)
	}
	index, err := exec.Command("git", "-C", root, "show", ":"+path).Output()
	if err != nil || string(index) != secondOnly {
		t.Fatalf("wrong index: %q, %v", index, err)
	}
	decoy, err := exec.Command("git", "-C", root, "show", ":f.txt").Output()
	if err != nil || string(decoy) != original {
		t.Fatalf("decoy modified: %q, %v", decoy, err)
	}
	working, err := os.ReadFile(filepath.Join(root, path))
	if err != nil || string(working) != changed {
		t.Fatalf("working file modified: %q, %v", working, err)
	}
}
