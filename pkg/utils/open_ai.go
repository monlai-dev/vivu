package utils

import (
	"context"
	"fmt"
	"vivu/internal/models/request_models"

	"github.com/pgvector/pgvector-go"
	openai "github.com/sashabaranov/go-openai"
)

type EmbeddingClientInterface interface {
	GetEmbedding(ctx context.Context, text string) (pgvector.Vector, error)
	GetEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error)
	GenerateStructuredPlan(ctx context.Context, userPrompt string, pois []string, dayCount int) (string, error)
	GeneratePlanOnlyJSON(
		ctx context.Context,
		profile any, // your TravelProfile or a lightweight struct
		poiList []request_models.POISummary,
		dayCount int,
	) (string, error)
}

type OpenAIEmbeddingClient struct {
	client *openai.Client
	model  string
}

func (c *OpenAIEmbeddingClient) GeneratePlanOnlyJSON(ctx context.Context, profile any, poiList []request_models.POISummary, dayCount int) (string, error) {
	//TODO implement me
	panic("implement me")
}

func NewOpenAIEmbeddingClient(apiKey, model string) EmbeddingClientInterface {
	return &OpenAIEmbeddingClient{
		client: openai.NewClient(apiKey),
		model:  model,
	}
}

func (c *OpenAIEmbeddingClient) GetEmbedding(ctx context.Context, text string) (pgvector.Vector, error) {
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.EmbeddingModel(c.model),
	})
	if err != nil {
		return pgvector.Vector{}, fmt.Errorf("embedding request failed: %w", err)
	}
	if len(resp.Data) == 0 {
		return pgvector.Vector{}, fmt.Errorf("no embedding returned for input")
	}
	return pgvector.NewVector(resp.Data[0].Embedding), nil
}

func (c *OpenAIEmbeddingClient) GetEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("no input texts provided")
	}
	resp, err := c.client.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: texts,
		Model: openai.EmbeddingModel(c.model),
	})
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	if len(resp.Data) != len(texts) {
		return nil, fmt.Errorf("embedding count mismatch: got %d, expected %d", len(resp.Data), len(texts))
	}
	vectors := make([]pgvector.Vector, len(resp.Data))
	for i, data := range resp.Data {
		vectors[i] = pgvector.NewVector(data.Embedding)
	}
	return vectors, nil
}

func (c *OpenAIEmbeddingClient) GenerateStructuredPlan(ctx context.Context, userPrompt string, pois []string, dayCount int) (string, error) {
	var systemPrompt string
	if dayCount > 1 {
		systemPrompt = fmt.Sprintf(`You are a travel planner AI.

Generate a %d-day itinerary based on the user prompt and the list of POIs.

Each day should include:
- day: number
- date: optional
- activities: list of activity blocks
- Each activity includes: activity, start_time, end_time, main_poi_id, alternative_poi_ids, what_to_do

Only use the POI IDs provided. Do not invent new places.
Return valid JSON.`, dayCount)
	} else {
		systemPrompt = `You are a travel planner AI.

Given a list of Points of Interest (POIs), create a structured daily travel plan.

Return a JSON array. Each item should have:
- activity: a short title
- start_time, end_time: 24-hour format (e.g. "09:00")
- main_poi_id: the primary POI for the activity
- what_to_do: a short description of the recommended activity at the POI
- alternative_poi_ids: 2–3 optional POI IDs to swap in

Constraints:
- Plan should run between 08:00 and 20:00.
- Each activity ~1.5–3 hours.
- Only use provided POI IDs. Do not invent new locations.
- Return valid JSON.`
	}

	poiList := ""
	for _, poi := range pois {
		poiList += "- " + poi + "\n"
	}

	userMessage := fmt.Sprintf("User prompt: %s\n\nAvailable POIs:\n%s", userPrompt, poiList)

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
		Temperature: 0.7,
	})
	if err != nil {
		return "", err
	}
	return resp.Choices[0].Message.Content, nil
}
