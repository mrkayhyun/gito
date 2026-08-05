package i18n

import (
	"regexp"
	"sort"
	"testing"
)

// verbRe matches the fmt verbs the catalog actually uses. Tf feeds every value
// through fmt.Sprintf, so a translation that loses or gains a verb prints
// %!s(MISSING) or %!(EXTRA ...) at runtime.
var verbRe = regexp.MustCompile(`%[sqdv]`)

// translations are the non-canonical catalogs that must mirror en exactly.
var translations = map[Lang]map[string]string{Ko: ko, Ja: ja, Zh: zh}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestCatalogKeySetsMatchEnglish(t *testing.T) {
	for lang, m := range translations {
		var missing, extra []string
		for _, k := range sortedKeys(en) {
			if _, ok := m[k]; !ok {
				missing = append(missing, k)
			}
		}
		for _, k := range sortedKeys(m) {
			if _, ok := en[k]; !ok {
				extra = append(extra, k)
			}
		}
		if len(missing) > 0 {
			t.Errorf("locale %s is missing %d key(s) present in en: %v", lang, len(missing), missing)
		}
		if len(extra) > 0 {
			t.Errorf("locale %s has %d key(s) absent from en: %v", lang, len(extra), extra)
		}
	}
}

func TestCatalogHasNoEmptyValues(t *testing.T) {
	all := map[Lang]map[string]string{En: en, Ko: ko, Ja: ja, Zh: zh}
	for lang, m := range all {
		for _, k := range sortedKeys(m) {
			if m[k] == "" {
				t.Errorf("locale %s: key %q has an empty value", lang, k)
			}
		}
	}
}

func TestCatalogFormatVerbParity(t *testing.T) {
	for lang, m := range translations {
		for _, k := range sortedKeys(en) {
			want := len(verbRe.FindAllString(en[k], -1))
			if want == 0 {
				continue
			}
			v, ok := m[k]
			if !ok {
				continue // reported by TestCatalogKeySetsMatchEnglish
			}
			if got := len(verbRe.FindAllString(v, -1)); got != want {
				t.Errorf("locale %s: key %q has %d format verb(s) but en has %d (en=%q, %s=%q)",
					lang, k, got, want, en[k], lang, v)
			}
		}
	}
}

// TestKeyHintVocabularyPresent pins the chrome.go hint vocabulary so the
// footer, help overlay and confirm bar never fall back to raw dotted keys.
func TestKeyHintVocabularyPresent(t *testing.T) {
	vocabulary := []string{
		"key.move", "key.top_bottom", "key.scroll", "key.page", "key.select",
		"key.run", "key.quick_select", "key.detail", "key.diff", "key.filter",
		"key.back", "key.quit", "key.close", "key.help", "key.stage_toggle",
		"key.stage_all", "key.discard", "key.switch", "key.create", "key.rename",
		"key.delete", "key.force_delete", "key.pop", "key.apply", "key.drop",
		"key.push", "key.remote_delete", "key.fetch", "key.fetch_all",
		"key.refresh", "key.branch_here", "key.view_blame", "key.next",
		"key.cancel", "key.confirm", "key.field", "key.amend", "key.edit",
		"help.keys_title",
	}
	for lang, m := range map[Lang]map[string]string{En: en, Ko: ko, Ja: ja, Zh: zh} {
		for _, k := range vocabulary {
			if m[k] == "" {
				t.Errorf("locale %s: hint key %q is missing", lang, k)
			}
		}
	}
}

// TestTFallsBackToEnglish documents the resolution order the parity tests
// protect: a translated value when present, English otherwise, key last.
func TestTFallsBackToEnglish(t *testing.T) {
	prev := Current()
	defer SetLang(prev)

	SetLang(Ko)
	if got := T("key.quit"); got != ko["key.quit"] {
		t.Errorf("T(key.quit) in ko = %q, want %q", got, ko["key.quit"])
	}
	if got := T("no.such.key.exists"); got != "no.such.key.exists" {
		t.Errorf("T of an unknown key = %q, want the key itself", got)
	}
}
