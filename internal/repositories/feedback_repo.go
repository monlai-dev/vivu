package repositories

import (
	"context"

	"gorm.io/gorm"
	"vivu/internal/models/db_models"
)

type FeedbackRepositoryInterface interface {
	CreateFeedback(ctx context.Context, feedback *db_models.Feedback) error
	ListFeedback(ctx context.Context, page, pageSize int) ([]db_models.Feedback, error)
}
type FeedbackRepository struct {
	db *gorm.DB
}

func NewFeedbackRepository(db *gorm.DB) *FeedbackRepository {
	return &FeedbackRepository{db: db}
}

func (r *FeedbackRepository) CreateFeedback(ctx context.Context, feedback *db_models.Feedback) error {
	return r.db.WithContext(ctx).Create(feedback).Error
}

func (r *FeedbackRepository) ListFeedback(ctx context.Context, page, pageSize int) ([]db_models.Feedback, error) {
	var feedbacks []db_models.Feedback
	err := r.db.WithContext(ctx).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Order("created_at DESC").
		Find(&feedbacks).Error
	return feedbacks, err
}
