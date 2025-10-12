package request_models

type CreatePaymentRequest struct {
	PlanCode string `json:"plan_code" binding:"required"`
}
