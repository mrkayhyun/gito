package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultOrcaRouterBaseURL = "https://api.orcarouter.ai/v1"
	defaultOrcaRouterModel   = "orcarouter/auto"
	maxDiffBytes             = 60 * 1024
)

// Suggestion is an AI-generated Conventional Commit suggestion.
type Suggestion struct {
	Type    string `json:"type"`
	Scope   string `json:"scope"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// OrcaRouterClient calls OrcaRouter's OpenAI-compatible chat completions API.
type OrcaRouterClient struct {
	APIKey     string
	BaseURL    string
	Model      string
	HTTPClient *http.Client
}

// NewOrcaRouterFromEnv builds a client from environment variables.
// ORCAROUTER_API_KEY is required. GITO_AI_MODEL and ORCAROUTER_BASE_URL are optional.
func NewOrcaRouterFromEnv() (*OrcaRouterClient, error) {
	apiKey := strings.TrimSpace(os.Getenv("ORCAROUTER_API_KEY"))
	if apiKey == "" {
		return nil, errors.New("ORCAROUTER_API_KEY is not set")
	}

	model := strings.TrimSpace(os.Getenv("GITO_AI_MODEL"))
	if model == "" {
		model = defaultOrcaRouterModel
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("ORCAROUTER_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = defaultOrcaRouterBaseURL
	}

	return &OrcaRouterClient{
		APIKey:  apiKey,
		BaseURL: baseURL,
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: 35 * time.Second,
		},
	}, nil
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// GenerateCommitSuggestion sends only the staged diff supplied by the caller.
// Large diffs are truncated to keep latency and API cost bounded.
func (c *OrcaRouterClient) GenerateCommitSuggestion(ctx context.Context, diff string) (Suggestion, error) {
	if c == nil || strings.TrimSpace(c.APIKey) == "" {
		return Suggestion{}, errors.New("OrcaRouter API key is empty")
	}
	if strings.TrimSpace(diff) == "" {
		return Suggestion{}, errors.New("staged diff is empty")
	}

	if len(diff) > maxDiffBytes {
		diff = diff[:maxDiffBytes] + "\n\n[diff truncated by gito]"
	}

	prompt := `Analyze the staged git diff and propose one Conventional Commit message.
Return ONLY a JSON object with exactly these string fields:
{"type":"feat|fix|docs|style|refactor|test|chore","scope":"optional short scope","subject":"imperative subject, max 50 chars","body":"optional concise body"}
Do not use markdown fences. Do not include commentary. Base the answer only on the diff.`

	payload := chatRequest{
		Model: c.Model,
		Messages: []chatMessage{
			{Role: "system", Content: "You write precise Conventional Commit messages for software changes."},
			{Role: "user", Content: prompt + "\n\nSTAGED DIFF:\n" + diff},
		},
		Temperature: 0.2,
		MaxTokens:   300,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Suggestion{}, fmt.Errorf("encode OrcaRouter request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Suggestion{}, fmt.Errorf("build OrcaRouter request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "gito-ai-commit")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 35 * time.Second}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return Suggestion{}, fmt.Errorf("call OrcaRouter: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Suggestion{}, fmt.Errorf("read OrcaRouter response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Suggestion{}, fmt.Errorf("OrcaRouter returned %s: %s", resp.Status, strings.TrimSpace(string(respBody)))
	}

	var decoded chatResponse
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return Suggestion{}, fmt.Errorf("decode OrcaRouter response: %w", err)
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return Suggestion{}, errors.New(decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return Suggestion{}, errors.New("OrcaRouter returned no choices")
	}

	suggestion, err := parseSuggestion(decoded.Choices[0].Message.Content)
	if err != nil {
		return Suggestion{}, err
	}
	return suggestion, nil
}

func parseSuggestion(content string) (Suggestion, error) {
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var suggestion Suggestion
	if err := json.Unmarshal([]byte(content), &suggestion); err != nil {
		return Suggestion{}, fmt.Errorf("parse AI commit suggestion: %w", err)
	}

	suggestion.Type = strings.ToLower(strings.TrimSpace(suggestion.Type))
	suggestion.Scope = strings.TrimSpace(suggestion.Scope)
	suggestion.Subject = strings.TrimSpace(suggestion.Subject)
	suggestion.Body = strings.TrimSpace(suggestion.Body)
	if suggestion.Subject == "" {
		return Suggestion{}, errors.New("AI commit suggestion has an empty subject")
	}
	return suggestion, nil
}
