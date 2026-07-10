package git

import (
	"strings"
	"testing"
)

func TestValidateRefName(t *testing.T) {
	valid := []string{
		"feat/x",
		"v1.0.0",
		"release-2",
		"feature/login",
		"main",
	}
	for _, name := range valid {
		if err := ValidateRefName(name); err != nil {
			t.Errorf("ValidateRefName(%q) = %v, want nil", name, err)
		}
	}

	invalid := []string{
		"",
		"  ",
		"-D",
		"--force",
		"has space",
		"a..b",
		"a~b",
		"a^b",
		"a:b",
		"a?b",
		"a*b",
		"a[b",
		"tip@{0}",
		".hidden",
		"foo.lock",
		"/lead",
		"trail/",
	}
	for _, name := range invalid {
		if err := ValidateRefName(name); err == nil {
			t.Errorf("ValidateRefName(%q) = nil, want error", name)
		}
	}
}

// validatorErrText is the distinctive substring every ValidateRefName error
// carries. git's own check-ref-format rejections use different wording ("is not
// a valid branch/tag name"), so asserting on this text proves the rejection came
// from ValidateRefName and not from git's built-in backstop.
const validatorErrText = "invalid ref name"

// TestRefGuardsRejectOptionInjection proves the ValidateRefName guards on the
// four ref-creating command functions actually run.
//
// It routes two kinds of bad names through every command function:
//   - dash-leading option-injection names ("-D", "--force", "-x", "-d"), and
//   - ref-metacharacter names git would otherwise reject ("a..b", "foo.lock",
//     ".hidden", "trail/").
//
// git independently rejects all of these once they land as positionals after
// '--end-of-options', so a test that only checked for "an error" would still
// pass even if the ValidateRefName calls were deleted (confirmed by the review).
// To make the test meaningful we assert the error carries ValidateRefName's
// distinctive text: if the guards are removed, git's own (differently-worded)
// rejection takes over and these assertions fail. It also verifies no branch or
// tag is created/modified on rejection.
func TestRefGuardsRejectOptionInjection(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// establish a known branch to rename from
	if err := CreateBranch("safe-branch", false); err != nil {
		t.Fatalf("CreateBranch setup: %v", err)
	}
	beforeBranches, _, _ := GetBranches()
	beforeTags, _ := GetTags()

	// Each case funnels an untrusted name into the validated argument of one
	// command function. Names that begin with '-' exercise the option-injection
	// guard; the ref-metacharacter names exercise the ref-format rules. In every
	// case the rejection must originate from ValidateRefName.
	commands := []struct {
		desc string
		run  func(name string) error
	}{
		{"CreateBranch(checkout)", func(n string) error { return CreateBranch(n, true) }},
		{"CreateBranch(no-checkout)", func(n string) error { return CreateBranch(n, false) }},
		{"RenameBranch", func(n string) error { return RenameBranch("safe-branch", n) }},
		{"CreateBranchAt", func(n string) error { return CreateBranchAt(n, "HEAD") }},
		{"CreateTag", func(n string) error { return CreateTag(n, "", "") }},
	}
	// Dash names git rejects on its own PLUS ref-metacharacter names git's
	// check-ref-format would reject with a *different* message than ours.
	badNames := []string{"-D", "--force", "-x", "-d", "a..b", "foo.lock", ".hidden", "trail/"}

	for _, cmd := range commands {
		for _, name := range badNames {
			err := cmd.run(name)
			if err == nil {
				t.Errorf("%s(%q) = nil, want error", cmd.desc, name)
				continue
			}
			if !strings.Contains(err.Error(), validatorErrText) {
				t.Errorf("%s(%q) error = %q, want it to contain %q (ValidateRefName guard missing?)",
					cmd.desc, name, err.Error(), validatorErrText)
			}
		}
	}

	afterBranches, _, _ := GetBranches()
	afterTags, _ := GetTags()

	if len(afterBranches) != len(beforeBranches) {
		t.Errorf("branch set changed: before %v, after %v", beforeBranches, afterBranches)
	}
	if !contains(afterBranches, "safe-branch") {
		t.Errorf("safe-branch missing after rejected rename: %v", afterBranches)
	}
	if len(afterTags) != len(beforeTags) {
		t.Errorf("tag set changed: before %d, after %d", len(beforeTags), len(afterTags))
	}
}
