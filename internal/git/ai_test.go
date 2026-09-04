package git

import (
	"os"
	"strings"
	"testing"
)

func TestGetStagedDiffOnlyIncludesStagedChanges(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	if err := os.WriteFile("README.md", []byte("hello staged\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, ".", "add", "README.md")

	if err := os.WriteFile("README.md", []byte("hello staged\nunstaged too\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	diff, err := GetStagedDiff()
	if err != nil {
		t.Fatalf("GetStagedDiff: %v", err)
	}
	if !strings.Contains(diff, "hello staged") {
		t.Fatalf("staged content missing from diff: %s", diff)
	}
	if strings.Contains(diff, "unstaged too") {
		t.Fatalf("unstaged content leaked into diff: %s", diff)
	}
}
