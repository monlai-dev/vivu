package services

import (
	"context"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type TagServiceInterface interface {
	GetAllTags(page int, pageSize int, ctx context.Context) ([]response_models.TagResponse, error)
}

type TagService struct {
	tagRepo repositories.TagRepository
}

func (t *TagService) GetAllTags(page int, pageSize int, ctx context.Context) ([]response_models.TagResponse, error) {
	tags, err := t.tagRepo.GetAllTags(page, pageSize, ctx)
	if err != nil {
		//log the error for debugging
		return nil, utils.ErrDatabaseError
	}

	// Handle empty result
	if len(tags) == 0 {
		return []response_models.TagResponse{}, utils.ErrTagNotFound
	}

	// Convert to response format
	tagResponses := make([]response_models.TagResponse, 0, len(tags))
	for _, tag := range tags {
		tagResponses = append(tagResponses, response_models.TagResponse{
			ID:   tag.ID.String(),
			En:   tag.Name,
			Vi:   tag.ViName,
			Icon: tag.Icon,
		})
	}

	return tagResponses, nil
}

func NewTagService(tagRepo repositories.TagRepository) TagServiceInterface {
	return &TagService{
		tagRepo: tagRepo,
	}
}
