package ai

import "testing"

// TestNewOrcaRouterFromEnv pins the env-config surface: the required API key,
// the two optional defaults, and normalization of the base URL. t.Setenv
// auto-restores each var and forbids t.Parallel, so these cases are isolated.
func TestNewOrcaRouterFromEnv(t *testing.T) {
	t.Run("missing api key is an error", func(t *testing.T) {
		t.Setenv("ORCAROUTER_API_KEY", "")
		t.Setenv("GITO_AI_MODEL", "")
		t.Setenv("ORCAROUTER_BASE_URL", "")
		if _, err := NewOrcaRouterFromEnv(); err == nil {
			t.Fatal("expected error when ORCAROUTER_API_KEY is unset, got nil")
		}
	})

	t.Run("whitespace-only api key is treated as missing", func(t *testing.T) {
		t.Setenv("ORCAROUTER_API_KEY", "   ")
		if _, err := NewOrcaRouterFromEnv(); err == nil {
			t.Fatal("expected error for whitespace-only key, got nil")
		}
	})

	t.Run("defaults applied when optionals unset", func(t *testing.T) {
		t.Setenv("ORCAROUTER_API_KEY", "sk-test")
		t.Setenv("GITO_AI_MODEL", "")
		t.Setenv("ORCAROUTER_BASE_URL", "")
		c, err := NewOrcaRouterFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.APIKey != "sk-test" {
			t.Errorf("APIKey = %q, want %q", c.APIKey, "sk-test")
		}
		if c.Model != defaultOrcaRouterModel {
			t.Errorf("Model = %q, want default %q", c.Model, defaultOrcaRouterModel)
		}
		if c.BaseURL != defaultOrcaRouterBaseURL {
			t.Errorf("BaseURL = %q, want default %q", c.BaseURL, defaultOrcaRouterBaseURL)
		}
		if c.HTTPClient == nil {
			t.Error("HTTPClient is nil, want a configured client")
		}
	})

	t.Run("overrides applied and trimmed", func(t *testing.T) {
		t.Setenv("ORCAROUTER_API_KEY", "  sk-live  ")
		t.Setenv("GITO_AI_MODEL", "  custom/model  ")
		t.Setenv("ORCAROUTER_BASE_URL", "  https://proxy.example.com/v2/  ")
		c, err := NewOrcaRouterFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c.APIKey != "sk-live" {
			t.Errorf("APIKey = %q, want trimmed %q", c.APIKey, "sk-live")
		}
		if c.Model != "custom/model" {
			t.Errorf("Model = %q, want trimmed %q", c.Model, "custom/model")
		}
		// Trailing slash(es) and surrounding whitespace must be stripped so URL
		// joining does not produce a double slash.
		if c.BaseURL != "https://proxy.example.com/v2" {
			t.Errorf("BaseURL = %q, want %q", c.BaseURL, "https://proxy.example.com/v2")
		}
	})
}
