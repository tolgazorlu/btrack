package ai

import (
	"context"
	"fmt"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type GeminiProvider struct {
	apiKey string
	model  string
}

func NewGeminiProvider(apiKey, model string) *GeminiProvider {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	return &GeminiProvider{apiKey: apiKey, model: model}
}

func (p *GeminiProvider) Name() string { return "Google Gemini" }

func (p *GeminiProvider) Complete(ctx context.Context, prompt string) (string, error) {
	client, err := genai.NewClient(ctx, option.WithAPIKey(p.apiKey))
	if err != nil {
		return "", fmt.Errorf("gemini: create client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel(p.model)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text("You are a developer productivity assistant. Be concise and professional.")},
	}
	model.MaxOutputTokens = ptrInt32(512)

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("gemini: %w", err)
	}
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("gemini: empty response")
	}
	text, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return "", fmt.Errorf("gemini: unexpected part type")
	}
	return string(text), nil
}

func ptrInt32(v int32) *int32 { return &v }
