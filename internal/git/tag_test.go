package git

import (
	"os"
	"os/exec"
	"testing"
)

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// setupRepo creates a throwaway git repo with one commit and chdirs into it.
// It returns a cleanup func that restores the original working directory.
func setupRepo(t *testing.T) func() {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init", "-q")
	run(t, dir, "config", "user.email", "test@example.com")
	run(t, dir, "config", "user.name", "Test User")
	if err := os.WriteFile(dir+"/README.md", []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "initial commit")

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() { _ = os.Chdir(orig) }
}

func TestTagLifecycle(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// initially no tags
	tags, err := GetTags()
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(tags))
	}

	// lightweight tag
	if err := CreateTag("v0.1.0", "", ""); err != nil {
		t.Fatalf("CreateTag lightweight: %v", err)
	}
	// annotated tag
	if err := CreateTag("v0.2.0", "release 0.2.0", ""); err != nil {
		t.Fatalf("CreateTag annotated: %v", err)
	}

	tags, err = GetTags()
	if err != nil {
		t.Fatalf("GetTags: %v", err)
	}
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %+v", len(tags), tags)
	}

	// version sort: v0.2.0 should come first (newest)
	if tags[0].Name != "v0.2.0" {
		t.Errorf("expected v0.2.0 first, got %q", tags[0].Name)
	}

	byName := map[string]TagEntry{}
	for _, tg := range tags {
		byName[tg.Name] = tg
	}

	if !byName["v0.2.0"].Annotated {
		t.Errorf("v0.2.0 should be annotated")
	}
	if byName["v0.2.0"].Subject != "release 0.2.0" {
		t.Errorf("v0.2.0 subject = %q, want %q", byName["v0.2.0"].Subject, "release 0.2.0")
	}
	if byName["v0.1.0"].Annotated {
		t.Errorf("v0.1.0 should be lightweight")
	}
	// lightweight falls back to commit subject
	if byName["v0.1.0"].Subject != "initial commit" {
		t.Errorf("v0.1.0 subject = %q, want %q", byName["v0.1.0"].Subject, "initial commit")
	}
	for _, tg := range tags {
		if tg.TargetHash == "" {
			t.Errorf("%s has empty TargetHash", tg.Name)
		}
		if tg.Date == "" {
			t.Errorf("%s has empty Date", tg.Name)
		}
	}

	// ShowTag returns content
	out, err := ShowTag("v0.2.0")
	if err != nil {
		t.Fatalf("ShowTag: %v", err)
	}
	if out == "" {
		t.Errorf("ShowTag returned empty output")
	}

	// DeleteTag
	if err := DeleteTag("v0.1.0"); err != nil {
		t.Fatalf("DeleteTag: %v", err)
	}
	tags, err = GetTags()
	if err != nil {
		t.Fatalf("GetTags after delete: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "v0.2.0" {
		t.Fatalf("after delete expected only v0.2.0, got %+v", tags)
	}
}

func TestCreateTagRequiresName(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	if err := CreateTag("", "", ""); err == nil {
		t.Errorf("expected error for empty tag name")
	}
}
