package request_models

import "github.com/google/uuid"

type CreatePaymentRequest struct {
	UserId   uuid.UUID `json:"user_id" binding:"required,uuid4"`
	PlanCode string    `json:"plan_code" binding:"required"`
}
