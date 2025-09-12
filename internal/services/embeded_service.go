package services

import "vivu/internal/repositories"

type EmbededServiceInterface interface {
}

type EmbededService struct {
	embededRepo repositories.IPoiEmbededRepository
}

func NewEmbededService(embededRepo repositories.IPoiEmbededRepository) EmbededServiceInterface {
	return &EmbededService{
		embededRepo: embededRepo,
	}
}
