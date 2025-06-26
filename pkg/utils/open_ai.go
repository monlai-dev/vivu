package utils

import (
	"context"
	"fmt"

	"github.com/pgvector/pgvector-go"
	openai "github.com/sashabaranov/go-openai"
)

type EmbeddingClientInterface interface {
	GetEmbedding(ctx context.Context, text string) (pgvector.Vector, error)
	GetEmbeddings(ctx context.Context, texts []string) ([]pgvector.Vector, error)
}

type OpenAIEmbeddingClient struct {
	client *openai.Client
	model  string
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
