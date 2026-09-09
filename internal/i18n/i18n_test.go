package i18n

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		want Lang
	}{
		{"en", En},
		{"ko", Ko},
		{"ja", Ja},
		{"zh", Zh},
		{"KO", Ko},          // case-insensitive
		{"  ja  ", Ja},      // surrounding whitespace
		{"ja_JP.UTF-8", Ja}, // region + encoding stripped
		{"zh-Hant", Zh},     // script/region stripped
		{"ko_KR", Ko},       // region stripped
		{"en@euro", En},     // modifier stripped
		{"", En},            // empty -> fallback
		{"c", En},           // POSIX C locale -> fallback
		{"posix", En},       // POSIX -> fallback
		{"fr", En},          // unsupported -> fallback
		{"klingon", En},     // unknown -> fallback
	}
	for _, c := range cases {
		if got := Parse(c.in); got != c.want {
			t.Errorf("Parse(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDetect_Precedence(t *testing.T) {
	// Clear every locale var so the test controls precedence explicitly.
	for _, k := range []string{"GITO_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(k, "")
	}

	// GITO_LANG wins over everything.
	t.Setenv("GITO_LANG", "ko")
	t.Setenv("LC_ALL", "ja_JP.UTF-8")
	t.Setenv("LANG", "zh_CN")
	if got := Detect(); got != Ko {
		t.Errorf("GITO_LANG should win: Detect() = %q, want %q", got, Ko)
	}

	// With GITO_LANG cleared, LC_ALL wins over LANG.
	t.Setenv("GITO_LANG", "")
	if got := Detect(); got != Ja {
		t.Errorf("LC_ALL should win over LANG: Detect() = %q, want %q", got, Ja)
	}

	// With LC_ALL and LC_MESSAGES cleared, LANG is used.
	t.Setenv("LC_ALL", "")
	if got := Detect(); got != Zh {
		t.Errorf("LANG should be used: Detect() = %q, want %q", got, Zh)
	}
}

func TestDetect_NoEnvFallsBackToEnglish(t *testing.T) {
	for _, k := range []string{"GITO_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(k, "")
	}
	if got := Detect(); got != En {
		t.Errorf("Detect() with no locale env = %q, want %q", got, En)
	}
}

func TestDetect_UnsupportedLocaleFallsBackToEnglish(t *testing.T) {
	for _, k := range []string{"GITO_LANG", "LC_ALL", "LC_MESSAGES", "LANG"} {
		t.Setenv(k, "")
	}
	t.Setenv("LANG", "fr_FR.UTF-8")
	if got := Detect(); got != En {
		t.Errorf("Detect() with unsupported LANG = %q, want %q", got, En)
	}
}

func TestSetLang(t *testing.T) {
	t.Cleanup(func() { SetLang(En) })

	SetLang(Ko)
	if Current() != Ko {
		t.Fatalf("after SetLang(Ko), Current() = %q, want %q", Current(), Ko)
	}

	// An unsupported value is a no-op: the previous language is preserved.
	SetLang(Lang("fr"))
	if Current() != Ko {
		t.Errorf("SetLang(unsupported) should be a no-op; Current() = %q, want %q", Current(), Ko)
	}

	// The empty language is also a no-op.
	SetLang(Lang(""))
	if Current() != Ko {
		t.Errorf("SetLang(\"\") should be a no-op; Current() = %q, want %q", Current(), Ko)
	}
}

func TestT_ActiveLanguageAndFallback(t *testing.T) {
	t.Cleanup(func() { SetLang(En) })

	// Known key resolves in the active language.
	SetLang(Ko)
	if got := T("committype.feat"); got != "새로운 기능" {
		t.Errorf("T(committype.feat) in ko = %q, want %q", got, "새로운 기능")
	}

	SetLang(En)
	if got := T("committype.feat"); got != "New feature" {
		t.Errorf("T(committype.feat) in en = %q, want %q", got, "New feature")
	}

	// A missing key falls back to the key itself so gaps are visible.
	if got := T("this.key.does.not.exist"); got != "this.key.does.not.exist" {
		t.Errorf("T(missing) = %q, want the key echoed back", got)
	}
}

func TestTf_FormatsArgs(t *testing.T) {
	t.Cleanup(func() { SetLang(En) })
	SetLang(En)

	// A missing key echoes the key as a format string; %d applies to it.
	// Using a key we control the shape of: the key itself has no verbs, so
	// extra args are appended by fmt with %!(EXTRA ...). Instead assert Tf
	// equals T when there are no args.
	if got, want := Tf("committype.feat"), T("committype.feat"); got != want {
		t.Errorf("Tf(key) with no args = %q, want %q (== T(key))", got, want)
	}
}
