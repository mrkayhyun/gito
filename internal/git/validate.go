package git

import (
	"fmt"
	"strings"
)

// ValidateRefName validates a user-supplied git ref name (branch or tag).
//
// It is a pure function (no exec) so it stays fast and easily unit-testable.
// The primary goal is to prevent option injection: a name whose first
// non-space rune is '-' could be interpreted by git as a command-line option.
// It also rejects names that violate the basic git ref-name rules so callers
// fail early with a descriptive error instead of leaking a confusing git error.
func ValidateRefName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("invalid ref name %q: must not be empty", name)
	}

	// Option-injection guard: the first non-space rune must not be '-'.
	trimmedLeading := strings.TrimLeft(name, " ")
	if strings.HasPrefix(trimmedLeading, "-") {
		return fmt.Errorf("invalid ref name %q: must not start with '-'", name)
	}

	// Reject whitespace and ASCII control characters anywhere in the name.
	for _, r := range name {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return fmt.Errorf("invalid ref name %q: must not contain whitespace", name)
		}
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("invalid ref name %q: must not contain control characters", name)
		}
	}

	// Reject git ref metacharacters and forbidden sequences.
	for _, seq := range []string{"..", "@{"} {
		if strings.Contains(name, seq) {
			return fmt.Errorf("invalid ref name %q: must not contain %q", name, seq)
		}
	}
	for _, c := range []string{"~", "^", ":", "?", "*", "[", "\\"} {
		if strings.Contains(name, c) {
			return fmt.Errorf("invalid ref name %q: must not contain %q", name, c)
		}
	}

	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return fmt.Errorf("invalid ref name %q: must not start or end with '/'", name)
	}
	if strings.HasSuffix(name, ".lock") {
		return fmt.Errorf("invalid ref name %q: must not end with '.lock'", name)
	}

	// No path component may start with '.'.
	for _, comp := range strings.Split(name, "/") {
		if strings.HasPrefix(comp, ".") {
			return fmt.Errorf("invalid ref name %q: no path component may start with '.'", name)
		}
	}

	return nil
}
