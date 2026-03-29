package ai

import (
	"context"
	"fmt"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

type Provider interface {
	GenerateCommitMessage(ctx context.Context, diff string) (string, error)
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
