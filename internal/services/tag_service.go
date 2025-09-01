package services

import (
	"context"
	"log"
	"vivu/internal/models/db_models"
	"vivu/internal/models/request_models"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type TagServiceInterface interface {
	GetAllTags(page int, pageSize int, ctx context.Context) ([]response_models.TagResponse, error)
	InsertTagTx(tag request_models.CreateTagRequest, ctx context.Context) error
}

type TagService struct {
	tagRepo repositories.TagRepositoryInterface
}

func (t *TagService) InsertTagTx(tag request_models.CreateTagRequest, ctx context.Context) error {
	createTag := db_models.Tag{
		EnName: tag.En,
		ViName: tag.Vi,
		Icon:   tag.Icon,
	}

	err := t.tagRepo.CreateTag(createTag, ctx)
	if err != nil {
		return utils.ErrDatabaseError
	}

	return nil
}

func (t *TagService) GetAllTags(page int, pageSize int, ctx context.Context) ([]response_models.TagResponse, error) {
	tags, err := t.tagRepo.GetAllTags(page, pageSize, ctx)
	if err != nil {
		//log the error for debugging
		log.Printf("Database error occurred: %v", err)
		return nil, utils.ErrDatabaseError
	}

	if len(tags) == 0 {
		return []response_models.TagResponse{}, utils.ErrTagNotFound
	}

	// Convert to response format
	tagResponses := make([]response_models.TagResponse, 0, len(tags))
	for _, tag := range tags {
		tagResponses = append(tagResponses, response_models.TagResponse{
			ID:   tag.ID.String(),
			En:   tag.EnName,
			Vi:   tag.ViName,
			Icon: tag.Icon,
		})
	}

	return tagResponses, nil
}

func NewTagService(tagRepo repositories.TagRepositoryInterface) TagServiceInterface {
	return &TagService{
		tagRepo: tagRepo,
	}
}
