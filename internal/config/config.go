// Package config loads gito configuration from:
//  1. ./gito.json  (project-local, takes precedence)
//  2. ~/.config/gito/config.json
//
// Example gito.json:
//
//	{
//	  "commit_types": [
//	    {"key": "feat",  "label": "feat   새 기능"},
//	    {"key": "fix",   "label": "fix    버그 수정"}
//	  ]
//	}
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type CommitType struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type Config struct {
	CommitTypes []CommitType `json:"commit_types"`
}

var defaults = []CommitType{
	{"feat", "feat      새로운 기능"},
	{"fix", "fix       버그 수정"},
	{"docs", "docs      문서 변경"},
	{"style", "style     코드 스타일"},
	{"refactor", "refactor  리팩토링"},
	{"test", "test      테스트"},
	{"chore", "chore     빌드/설정"},
}

func Load() Config {
	if cfg, ok := loadFile("gito.json"); ok {
		return cfg
	}
	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".config", "gito", "config.json")
		if cfg, ok := loadFile(path); ok {
			return cfg
		}
	}
	return Config{CommitTypes: defaults}
}

func loadFile(path string) (Config, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, false
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil || len(cfg.CommitTypes) == 0 {
		return Config{}, false
	}
	return cfg, true
}
