package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type CommitGroup struct {
	Title string   `json:"title"`
	Body  string   `json:"body"`
	Files []string `json:"files"`
}

type Provider interface {
	GenerateCommitMessage(ctx context.Context, diff string) (string, error)
	GroupChanges(ctx context.Context, diff string, files []string) ([]CommitGroup, error)
}

type ProviderConfig struct {
	Name   string // "groq" or "cerebras"
	APIKey string
	Model  string
}

var providerDefaults = map[string]struct {
	BaseURL string
	Model   string
}{
	"groq": {
		BaseURL: "https://api.groq.com/openai/v1",
		Model:   "llama-3.3-70b-versatile",
	},
	"cerebras": {
		BaseURL: "https://api.cerebras.ai/v1",
		Model:   "qwen-3-235b-a22b-instruct-2507",
	},
}

type openaiProvider struct {
	client *openai.Client
	model  string
}

func NewProvider(cfg ProviderConfig) (Provider, error) {
	defaults, ok := providerDefaults[cfg.Name]
	if !ok {
		return nil, fmt.Errorf("unknown provider %q. Choose: groq, cerebras", cfg.Name)
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key not configured. Run: gca config set api-key <your-key>")
	}

	model := cfg.Model
	if model == "" {
		model = defaults.Model
	}

	clientCfg := openai.DefaultConfig(cfg.APIKey)
	clientCfg.BaseURL = defaults.BaseURL

	return &openaiProvider{
		client: openai.NewClientWithConfig(clientCfg),
		model:  model,
	}, nil
}

func (p *openaiProvider) GenerateCommitMessage(ctx context.Context, diff string) (string, error) {
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: FormatUserMessage(diff)},
		},
		MaxTokens:   1024,
		Temperature: 0.3,
	})
	if err != nil {
		return "", fmt.Errorf("AI request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("AI returned empty response")
	}

	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}

func (p *openaiProvider) GroupChanges(ctx context.Context, diff string, files []string) ([]CommitGroup, error) {
	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: p.model,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: splitSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: FormatSplitMessage(diff, files)},
		},
		MaxTokens:   4096,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, fmt.Errorf("AI request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("AI returned empty response")
	}

	raw := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Extract JSON if wrapped in markdown fences
	raw = extractJSON(raw)

	var result struct {
		Commits []CommitGroup `json:"commits"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, fmt.Errorf("AI returned invalid JSON: %w", err)
	}
	if len(result.Commits) == 0 {
		return nil, fmt.Errorf("AI returned no commit groups")
	}

	return result.Commits, nil
}

func extractJSON(s string) string {
	// Strip markdown code fences if present
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	// Find the outermost JSON object
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
