package repositories

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"vivu/internal/models/db_models"
)

type IPlanRepository interface {
	GetPlanInfoById(ctx context.Context, planID string) (*db_models.Plan, error)
	GetAllPlans(ctx context.Context) ([]db_models.Plan, error)
}

type PlanRepository struct {
	db *gorm.DB
}

func NewPlanRepository(db *gorm.DB) IPlanRepository {
	return &PlanRepository{db: db}
}

func (p PlanRepository) GetPlanInfoById(ctx context.Context, planID string) (*db_models.Plan, error) {

	var plan db_models.Plan
	err := p.db.WithContext(ctx).First(&plan, "id = ?", planID).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &plan, nil
}

func (p PlanRepository) GetAllPlans(ctx context.Context) ([]db_models.Plan, error) {

	var plans []db_models.Plan
	err := p.db.WithContext(ctx).Find(&plans).Error

	if err != nil {
		return nil, err
	}

	return plans, nil
}
