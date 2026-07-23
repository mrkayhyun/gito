// Package i18n provides lightweight localization for gito's terminal UI.
//
// It supports four languages — English (en), Korean (ko), Japanese (ja) and
// Chinese (zh) — and auto-detects the active language from the environment.
//
// Detection precedence (first non-empty wins):
//  1. GITO_LANG                (explicit override, e.g. "ko", "ja_JP")
//  2. LC_ALL / LC_MESSAGES     (POSIX locale category overrides)
//  3. LANG                     (default POSIX locale)
//
// A gito.json "lang" field can further override the detected language at
// startup via SetLang(Parse(...)). When no supported language is found the
// package falls back to English.
package i18n

import (
	"fmt"
	"os"
	"strings"
)

// Lang is a supported UI language, identified by its ISO 639-1 code.
type Lang string

const (
	En Lang = "en"
	Ko Lang = "ko"
	Ja Lang = "ja"
	Zh Lang = "zh"
)

// current holds the active language. It defaults to English so that code paths
// that never call Init (such as unit tests) still resolve to a valid catalog.
var current = En

// Init detects the UI language from the environment and sets it as current.
// It is safe to call multiple times; the last call wins.
func Init() {
	current = Detect()
}

// SetLang overrides the active language. Passing an empty/unsupported value is
// a no-op, preserving whatever was previously detected.
func SetLang(l Lang) {
	switch l {
	case En, Ko, Ja, Zh:
		current = l
	}
}

// Current returns the active language.
func Current() Lang { return current }

// Detect resolves the UI language from environment variables, following the
// precedence documented on the package. It never fails: unknown locales map to
// English.
func Detect() Lang {
	for _, key := range []string{"GITO_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			if l, ok := parse(v); ok {
				return l
			}
		}
	}
	return En
}

// Parse converts a locale-ish string (e.g. "ko", "ja_JP.UTF-8", "zh-Hant")
// into a supported Lang, falling back to English when unrecognized.
func Parse(s string) Lang {
	if l, ok := parse(s); ok {
		return l
	}
	return En
}

// parse extracts the leading language subtag and maps it to a supported Lang.
// The bool result reports whether the input matched a supported language.
func parse(s string) (Lang, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" || s == "c" || s == "posix" {
		return En, false
	}
	// Strip encoding/modifier/region: "ja_JP.UTF-8" -> "ja", "zh-Hant" -> "zh".
	for _, sep := range []string{".", "@", "_", "-"} {
		if i := strings.Index(s, sep); i >= 0 {
			s = s[:i]
		}
	}
	switch s {
	case "en":
		return En, true
	case "ko":
		return Ko, true
	case "ja":
		return Ja, true
	case "zh":
		return Zh, true
	}
	return En, false
}

// T returns the message for the given key in the active language. If the key
// is missing for that language it falls back to English, and finally to the
// key itself so that missing translations are visible rather than blank.
func T(key string) string {
	if m, ok := catalog[current]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := catalog[En][key]; ok {
		return s
	}
	return key
}

// Tf is T followed by fmt.Sprintf, for messages that embed runtime values.
func Tf(key string, args ...any) string {
	return fmt.Sprintf(T(key), args...)
}
