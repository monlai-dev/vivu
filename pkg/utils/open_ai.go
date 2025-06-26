package utils

import (
	"context"
	"github.com/pgvector/pgvector-go"
	openai "github.com/sashabaranov/go-openai"
	"os"
)

var client = openai.NewClient(os.Getenv("OPENAI_API_KEY"))

func GetEmbedding(text string) pgvector.Vector {
	resp, err := client.CreateEmbeddings(context.Background(), openai.EmbeddingRequest{
		Input: []string{text},
		Model: openai.SmallEmbedding3, // or "text-embedding-3-small"
	})
	if err != nil {
		panic(err)
	}
	return pgvector.NewVector(resp.Data[0].Embedding)
}
