package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrkayhyun/gito/internal/i18n"
)

// chdir switches into dir for the duration of the test and restores the
// previous working directory on cleanup.
func chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// isolateHome points os.UserHomeDir at a temp dir so a real ~/.config/gito
// config on the developer's machine cannot leak into the test.
func isolateHome(t *testing.T, home string) {
	t.Helper()
	t.Setenv("HOME", home)
	// Windows resolves the home dir from USERPROFILE.
	t.Setenv("USERPROFILE", home)
}

func TestLoad_DefaultsWhenNoConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	isolateHome(t, t.TempDir())

	cfg := Load()

	if len(cfg.CommitTypes) == 0 {
		t.Fatal("expected default commit types when no config present")
	}
	if cfg.CommitTypes[0].Key != "feat" {
		t.Errorf("first default commit type = %q, want %q", cfg.CommitTypes[0].Key, "feat")
	}
	// The conventional-commits default set has 7 entries.
	if got, want := len(cfg.CommitTypes), 7; got != want {
		t.Errorf("default commit type count = %d, want %d", got, want)
	}
}

func TestLoad_ProjectLocalTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := t.TempDir()
	isolateHome(t, home)

	// A home config that should be shadowed by the project-local one.
	homeCfgDir := filepath.Join(home, ".config", "gito")
	if err := os.MkdirAll(homeCfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(homeCfgDir, "config.json"),
		`{"commit_types":[{"key":"home","label":"home only"}]}`)

	writeFile(t, filepath.Join(dir, "gito.json"),
		`{"commit_types":[{"key":"proj","label":"project only"}]}`)

	cfg := Load()

	if len(cfg.CommitTypes) != 1 || cfg.CommitTypes[0].Key != "proj" {
		t.Fatalf("expected project-local config to win, got %+v", cfg.CommitTypes)
	}
}

func TestLoad_HomeConfigUsedWhenNoProjectConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	home := t.TempDir()
	isolateHome(t, home)

	homeCfgDir := filepath.Join(home, ".config", "gito")
	if err := os.MkdirAll(homeCfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(homeCfgDir, "config.json"),
		`{"commit_types":[{"key":"home","label":"home only"}]}`)

	cfg := Load()

	if len(cfg.CommitTypes) != 1 || cfg.CommitTypes[0].Key != "home" {
		t.Fatalf("expected home config to be used, got %+v", cfg.CommitTypes)
	}
}

func TestLoad_LangOverrideApplied(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	isolateHome(t, t.TempDir())

	// Restore global i18n language after the test to avoid cross-test leakage.
	t.Cleanup(func() { i18n.SetLang(i18n.Parse("en")) })

	writeFile(t, filepath.Join(dir, "gito.json"), `{"lang":"ko"}`)

	_ = Load()

	if got := i18n.Current(); got != i18n.Parse("ko") {
		t.Errorf("i18n language after load = %v, want %v", got, i18n.Parse("ko"))
	}
}

func TestLoad_InvalidJSONFallsBackToDefaults(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	isolateHome(t, t.TempDir())

	writeFile(t, filepath.Join(dir, "gito.json"), `{not valid json`)

	cfg := Load()

	// Malformed project config is ignored; defaults fill in.
	if len(cfg.CommitTypes) == 0 {
		t.Fatal("expected defaults when project config is malformed")
	}
	if cfg.CommitTypes[0].Key != "feat" {
		t.Errorf("first commit type = %q, want default %q", cfg.CommitTypes[0].Key, "feat")
	}
}

func TestLoad_CustomCommitTypesPreserved(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	isolateHome(t, t.TempDir())

	writeFile(t, filepath.Join(dir, "gito.json"),
		`{"commit_types":[{"key":"wip","label":"work in progress"},{"key":"hotfix","label":"urgent fix"}]}`)

	cfg := Load()

	if len(cfg.CommitTypes) != 2 {
		t.Fatalf("commit type count = %d, want 2", len(cfg.CommitTypes))
	}
	if cfg.CommitTypes[1].Key != "hotfix" || cfg.CommitTypes[1].Label != "urgent fix" {
		t.Errorf("second commit type = %+v, want {hotfix urgent fix}", cfg.CommitTypes[1])
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
