// cmd/fx/prompt_fx/module.go
package prompt_fx

import (
	"fmt"
	"log"
	"os"
	"strings"
	"vivu/internal/api/controllers"
	"vivu/internal/repositories"
	"vivu/internal/services"
	"vivu/pkg/utils"

	"go.uber.org/fx"
)

var Module = fx.Provide(
	ProvideEmbeddingClient,
	ProvidePromptService,
	ProvidePromptController)

// EmbeddingConfig holds configuration for embedding clients
type EmbeddingConfig struct {
	Provider string
	APIKey   string
	Model    string
}

// ProvideEmbeddingClient creates an embedding client based on environment variables
func ProvideEmbeddingClient() (utils.EmbeddingClientInterface, error) {
	config := getEmbeddingConfig()

	log.Printf("Initializing %s embedding client with model: %s", config.Provider, config.Model)

	switch strings.ToLower(config.Provider) {
	case "openai":
		return utils.NewOpenAIEmbeddingClient(config.APIKey, config.Model), nil
	case "gemini":
		client, err := utils.NewGeminiEmbeddingClient(config.APIKey, config.Model)
		if err != nil {
			return nil, fmt.Errorf("failed to create Gemini client: %w", err)
		}
		return client, nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s. Use 'openai' or 'gemini'", config.Provider)
	}
}

// ProvidePromptService creates the prompt service with all dependencies
func ProvidePromptService(
	poisService services.POIServiceInterface,
	tagService services.TagServiceInterface,
	aiService utils.EmbeddingClientInterface,
	embededRepo repositories.IPoiEmbededRepository,
	poisRepo repositories.POIRepository,
) services.PromptServiceInterface {
	return services.NewPromptService(
		poisService,
		tagService,
		aiService,
		embededRepo,
		poisRepo,
	)
}

// ProvidePromptController creates the prompt controller
func ProvidePromptController(
	promptService services.PromptServiceInterface,
) *controllers.PromptController {
	return controllers.NewPromptController(promptService)
}

// getEmbeddingConfig reads configuration from environment variables
func getEmbeddingConfig() EmbeddingConfig {
	provider := getEnvWithDefault("EMBEDDING_PROVIDER", "gemini") // Default to free Gemini

	var apiKey, model string

	switch strings.ToLower(provider) {
	case "openai":
		apiKey = os.Getenv("OPENAI_API_KEY")
		model = getEnvWithDefault("OPENAI_MODEL", "text-embedding-3-small")
		if apiKey == "" {
			log.Fatal("OPENAI_API_KEY is required when using OpenAI provider")
		}
	case "gemini":
		apiKey = os.Getenv("GEMINI_API_KEY")
		model = getEnvWithDefault("GEMINI_MODEL", "gemini-1.5-flash")
		if apiKey == "" {
			log.Fatal("GEMINI_API_KEY is required when using Gemini provider")
		}
	}

	return EmbeddingConfig{
		Provider: provider,
		APIKey:   apiKey,
		Model:    model,
	}
}

// getEnvWithDefault returns environment variable or default value
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
