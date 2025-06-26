package services

type PromptServiceInterface interface {
	CreatePrompt(prompt string) (string, error)
}
