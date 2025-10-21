package services

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"vivu/internal/models/db_models"
	"vivu/internal/repositories"
)

type FeedbackServiceInterface interface {
	AddFeedback(ctx context.Context, userID uuid.UUID, comment string, rating int) error
	GetFeedback(ctx context.Context, page, pageSize int) ([]db_models.Feedback, error)
}

type FeedbackService struct {
	feedbackRepo repositories.FeedbackRepositoryInterface
}

func NewFeedbackService(feedbackRepo repositories.FeedbackRepositoryInterface) FeedbackServiceInterface {
	return &FeedbackService{feedbackRepo: feedbackRepo}
}

func (s *FeedbackService) AddFeedback(ctx context.Context, userID uuid.UUID, comment string, rating int) error {
	if rating < 1 || rating > 5 {
		return errors.New("rating must be between 1 and 5")
	}

	feedback := &db_models.Feedback{
		UserID:  userID,
		Comment: comment,
		Rating:  rating,
	}

	return s.feedbackRepo.CreateFeedback(ctx, feedback)
}

func (s *FeedbackService) GetFeedback(ctx context.Context, page, pageSize int) ([]db_models.Feedback, error) {
	return s.feedbackRepo.ListFeedback(ctx, page, pageSize)
}
