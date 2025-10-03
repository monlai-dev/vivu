package services

import (
	"context"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type PlanServiceInterface interface {
	GetPlans() ([]string, error)
	GetPlanInfoById(ctx context.Context, planId string) (response_models.SubscriptionPlan, error)
}

func NewPlanService(planRepo repositories.IPlanRepository) PlanServiceInterface {
	return &PlanService{
		planRepo: planRepo,
	}
}

type PlanService struct {
	planRepo repositories.IPlanRepository
}

func (p *PlanService) GetPlans() ([]string, error) {
	//TODO implement me
	panic("implement me")
}

func (p *PlanService) GetPlanInfoById(ctx context.Context, planId string) (response_models.SubscriptionPlan, error) {

	plan, err := p.planRepo.GetPlanInfoById(ctx, planId)
	if err != nil {
		return response_models.SubscriptionPlan{}, utils.ErrDatabaseError
	}

	if plan == nil {
		return response_models.SubscriptionPlan{}, utils.RecordNotFound
	}

	result := response_models.SubscriptionPlan{
		ID:              plan.ID,
		Name:            plan.Name,
		Description:     plan.Description,
		BackgroundImage: plan.BackgroundImage,
		Price:           plan.PriceMinor,
		Currency:        plan.Currency,
		Period:          string(plan.Period),
		TrialDays:       plan.TrialDays,
		IsActive:        plan.IsActive,
	}

	return result, nil

}
