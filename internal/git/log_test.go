package git

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestGetLogEntriesPaths(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	addCommit(t, "a.txt", "a\n", "add a")
	addCommit(t, "b.txt", "b\n", "add b")
	addCommit(t, "a.txt", "a2\n", "edit a")

	entries, err := GetLogEntries(10, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Subject)
	}
	if strings.Join(got, ",") != "edit a,add a" {
		t.Errorf("log a.txt = %v, want [edit a add a]", got)
	}

	// A path that looks like an option is still a pathspec after "--".
	if entries, err := GetLogEntries(10, "--all"); err != nil || len(entries) != 0 {
		t.Errorf("log -- --all = %v, %v; want no commits and no error", entries, err)
	}
}

func TestSearchLog(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	for _, f := range []string{"x1", "x2", "x3", "y1"} {
		addCommit(t, f+".txt", f, "touch "+f)
	}
	match := func(c CommitEntry) bool { return strings.Contains(c.Subject, "x") }

	all, err := SearchLog(match, 100)
	if err != nil || len(all) != 3 {
		t.Fatalf("SearchLog = %d results, %v; want 3", len(all), err)
	}
	if all[0].Subject != "touch x3" {
		t.Errorf("results should be newest first, got %q", all[0].Subject)
	}

	// Stopping early at the limit must not surface git's kill as an error.
	two, err := SearchLog(match, 2)
	if err != nil || len(two) != 2 {
		t.Errorf("SearchLog(limit 2) = %d results, %v; want 2, nil", len(two), err)
	}

	scoped, err := SearchLog(func(CommitEntry) bool { return true }, 100, "y1.txt")
	if err != nil || len(scoped) != 1 || scoped[0].Subject != "touch y1" {
		t.Errorf("SearchLog scoped to y1.txt = %+v, %v", scoped, err)
	}

	if none, err := SearchLog(match, 0); err != nil || none != nil {
		t.Errorf("limit 0 should return nothing, got %v, %v", none, err)
	}
}

func TestIsTrackedFile(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()
	if err := os.MkdirAll("dir", 0o755); err != nil {
		t.Fatal(err)
	}
	addCommit(t, "dir/in.txt", "in\n", "add dir")
	if err := os.WriteFile("untracked.txt", []byte("u\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	globName := runtime.GOOS != "windows" // '*' is not a legal Windows filename
	if globName {
		if err := os.WriteFile("*.md", []byte("glob\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	cases := map[string]bool{
		"README.md":     true,
		"dir/in.txt":    true,
		"./README.md":   true,
		"dir":           false, // directory, even though it contains tracked files
		"untracked.txt": false,
		"missing.txt":   false,
		"*.md":          false, // untracked file named like a glob must not match README.md
		"":              false,
	}
	if !globName {
		delete(cases, "*.md")
	}
	for path, want := range cases {
		if got := IsTrackedFile(path); got != want {
			t.Errorf("IsTrackedFile(%q) = %v, want %v", path, got, want)
		}
	}
}
