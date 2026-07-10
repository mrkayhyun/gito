package git

import "testing"

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

// TestRefGuardsRejectOptionInjection proves the validation guards prevent
// option-injection names from creating or modifying any ref.
func TestRefGuardsRejectOptionInjection(t *testing.T) {
	cleanup := setupRepo(t)
	defer cleanup()

	// establish a known branch to rename from
	if err := CreateBranch("safe-branch", false); err != nil {
		t.Fatalf("CreateBranch setup: %v", err)
	}
	beforeBranches, _, _ := GetBranches()
	beforeTags, _ := GetTags()

	if err := CreateBranch("-D", true); err == nil {
		t.Errorf("CreateBranch(\"-D\", true) = nil, want error")
	}
	if err := RenameBranch("safe-branch", "--force"); err == nil {
		t.Errorf("RenameBranch(\"safe-branch\", \"--force\") = nil, want error")
	}
	if err := CreateBranchAt("-x", "HEAD"); err == nil {
		t.Errorf("CreateBranchAt(\"-x\", \"HEAD\") = nil, want error")
	}
	if err := CreateTag("-d", "", ""); err == nil {
		t.Errorf("CreateTag(\"-d\", \"\", \"\") = nil, want error")
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
