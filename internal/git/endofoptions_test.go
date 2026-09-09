package git

import (
	"strings"
	"testing"
)

// This file pins the SECOND half of gito's option-injection defense.
//
// SECURITY.md's threat model names three defence-in-depth mitigations for the
// primary attack surface (argument construction for `git`):
//
//   1. `--end-of-options` before user-influenced ref arguments,
//   2. `--` before pathspecs, and
//   3. `ValidateRefName` on ref-creating paths.
//
// TestRefGuardsRejectOptionInjection (validate_test.go) already pins mitigation
// (3) for the five ValidateRefName-guarded functions, asserting the rejection
// text comes from ValidateRefName specifically.
//
// The wrappers exercised HERE are the ones that deliberately DO NOT call
// ValidateRefName and rely solely on `--end-of-options` / `--` to neutralise a
// value that begins with '-'. Nothing previously proved that guard is actually
// present: if `--end-of-options` were dropped from, say, SwitchBranch, git would
// parse "-D" as the --delete flag rather than as a ref operand, and a naive
// "expect an error" test would still pass because the mangled command also
// errors. So each case asserts the FAILURE MODE is ref/pathspec resolution
// ("did not match", "unknown revision", "not a valid object", …) — the shape a
// value gets when git is *forced* to treat it as data — and NOT an
// unknown-/ambiguous-option usage error, which is what a leaked flag produces.
//
// It also asserts the injection had no side effect: the branch a leaked "-D"
// or "--force" would have deleted is still present afterwards.

// optionLeakMarkers are substrings that appear when git parses our payload as an
// OPTION (the failure we are guarding against). Their presence means the
// `--end-of-options` barrier is missing.
var optionLeakMarkers = []string{
	"unknown option",
	"unknown switch",
	"ambiguous option",
	"usage:",
	"error: option",
}

// assertNotOptionLeak fails the test when err looks like git parsed the payload
// as an option rather than as an operand. A nil error is always a failure here:
// every payload is a name git cannot resolve as a ref/pathspec, so a
// well-guarded wrapper must return SOME error.
func assertNotOptionLeak(t *testing.T, desc, payload string, err error) {
	t.Helper()
	if err == nil {
		t.Errorf("%s(%q) = nil, want a ref-resolution error (payload may have been executed as an option)", desc, payload)
		return
	}
	msg := strings.ToLower(err.Error())
	for _, m := range optionLeakMarkers {
		if strings.Contains(msg, m) {
			t.Errorf("%s(%q) error = %q — looks like the payload leaked as an OPTION; is `--end-of-options` missing?", desc, payload, err.Error())
			return
		}
	}
}

// TestEndOfOptionsNeutralisesInjection drives dash-leading option-injection
// payloads through every ref/ref-selector wrapper that guards with
// `--end-of-options` (rather than ValidateRefName) and proves the payload is
// forced into operand position instead of being parsed as a flag.
func TestEndOfOptionsNeutralisesInjection(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// A branch a leaked "-D"/"--delete"/"--force" could destroy. Its survival is
	// the side-effect assertion.
	if err := CreateBranch("victim-branch", false); err != nil {
		t.Fatalf("CreateBranch(victim-branch): %v", err)
	}

	// The most dangerous payloads: real git flags that DO something on the
	// commands under test (delete a branch, force an action, move HEAD).
	payloads := []string{"-D", "--force", "--delete", "-f", "-x"}

	cases := []struct {
		desc string
		run  func(name string) error
	}{
		// Branch selection / mutation without ValidateRefName.
		{"SwitchBranch", SwitchBranch},
		{"DeleteBranch(-d)", func(n string) error { return DeleteBranch(n, false) }},
		{"DeleteBranch(-D)", func(n string) error { return DeleteBranch(n, true) }},
		// Read paths that take a user-selected ref/hash.
		{"GetCommitDetail", func(n string) error { _, e := GetCommitDetail(n); return e }},
		{"ShowTag", func(n string) error { _, e := ShowTag(n); return e }},
		{"DeleteTag", DeleteTag},
		// Stash selectors.
		{"StashApply", StashApply},
		{"StashPop", StashPop},
		{"StashDrop", StashDrop},
		{"StashShow", func(n string) error { _, e := StashShow(n); return e }},
		// Ref-pair diff endpoints.
		{"GetDiffBetween(base)", func(n string) error { _, e := GetDiffBetween(n, "HEAD"); return e }},
	}

	for _, c := range cases {
		for _, p := range payloads {
			assertNotOptionLeak(t, c.desc, p, c.run(p))
		}
	}

	// Side-effect check: no leaked flag deleted victim-branch.
	branches, _, err := GetBranches()
	if err != nil {
		t.Fatalf("GetBranches: %v", err)
	}
	if !contains(branches, "victim-branch") {
		t.Errorf("victim-branch was removed — an option-injection payload took effect: %v", branches)
	}
}

// TestPathspecDashDashNeutralisesInjection covers the `--` (pathspec) barrier on
// path-taking wrappers: a path beginning with '-' must be treated as a pathspec,
// never as an option. StageFile/UnstageFile/DiscardFile add or restore, and a
// missing `--` would let "-A"/"--staged" etc. leak as flags.
func TestPathspecDashDashNeutralisesInjection(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	payloads := []string{"-A", "--all", "--staged", "-f"}

	cases := []struct {
		desc string
		run  func(path string) error
	}{
		{"StageFile", StageFile},
		{"UnstageFile", UnstageFile},
		{"DiscardFile", DiscardFile},
	}

	for _, c := range cases {
		for _, p := range payloads {
			// These operate on a pathspec that does not exist; with `--` present
			// git reports a pathspec/spec error, without it the flag would leak.
			assertNotOptionLeak(t, c.desc, p, c.run(p))
		}
	}
}
