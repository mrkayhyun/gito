// Package config loads gito configuration from:
//  1. ./gito.json  (project-local, takes precedence)
//  2. ~/.config/gito/config.json
//
// Example gito.json:
//
//	{
//	  "lang": "en",
//	  "commit_types": [
//	    {"key": "feat",  "label": "feat   New feature"},
//	    {"key": "fix",   "label": "fix    Bug fix"}
//	  ]
//	}
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gito/internal/i18n"
)

type CommitType struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type Config struct {
	// Lang optionally overrides the auto-detected UI language ("en", "ko",
	// "ja", "zh"). When empty, the environment-detected language is used.
	Lang        string       `json:"lang"`
	CommitTypes []CommitType `json:"commit_types"`
}

// defaultCommitTypes builds the Conventional Commits default types with
// descriptions localized to the active i18n language. It is a function (not a
// package var) so labels reflect the language chosen at load time.
func defaultCommitTypes() []CommitType {
	keys := []string{"feat", "fix", "docs", "style", "refactor", "test", "chore"}
	types := make([]CommitType, len(keys))
	for i, k := range keys {
		types[i] = CommitType{
			Key:   k,
			Label: fmt.Sprintf("%-9s %s", k, i18n.T("committype."+k)),
		}
	}
	return types
}

func Load() Config {
	if cfg, ok := loadFile("gito.json"); ok {
		return finalize(cfg)
	}
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".config", "gito", "config.json")
		if cfg, ok := loadFile(path); ok {
			return finalize(cfg)
		}
	}
	return finalize(Config{})
}

// finalize applies a config-specified language override and fills in default
// commit types when none were provided. Applying the language before building
// defaults ensures the default labels are localized correctly.
func finalize(cfg Config) Config {
	if cfg.Lang != "" {
		i18n.SetLang(i18n.Parse(cfg.Lang))
	}
	if len(cfg.CommitTypes) == 0 {
		cfg.CommitTypes = defaultCommitTypes()
	}
	return cfg
}

func loadFile(path string) (Config, bool) {
	// #nosec G304 -- path is one of a fixed set of known config locations (./gito.json or ~/.config/gito/config.json), chosen by the loader, not attacker-controlled
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, false
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false
	}
	return cfg, true
}
