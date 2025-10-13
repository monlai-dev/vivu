package response_models

import (
	_ "encoding/json"
	"github.com/google/uuid"
	_ "vivu/internal/models/db_models"
)

type SubscriptionPlan struct {
	ID              uuid.UUID `json:"id"`                    // Unique identifier
	Code            string    `json:"code"`                  // e.g., "basic", "pro_monthly", "pro_yearly"
	Name            string    `json:"name"`                  // Plan name
	Description     *string   `json:"description,omitempty"` // Optional description
	BackgroundImage string    `json:"background_image"`      // Background image URL
	Period          string    `json:"period"`                // "month" | "year"
	Price           int64     `json:"price"`                 // Formatted price, e.g., "$9.99"
	Currency        string    `json:"currency"`              // "USD", "VND"
	TrialDays       int32     `json:"trial_days"`            // Number of trial days
	IsActive        bool      `json:"is_active"`             // Whether the plan is active
	Features        []string  `json:"features,omitempty"`    // List of features
}

type CreateCheckoutResponse struct {
	OrderCode    int64  `json:"order_code"`
	Amount       int64  `json:"amount"`
	PaymentURL   string `json:"payment_url"`
	ProviderName string `json:"provider"`
}

type SubscriptionStatusResponse struct {
	AccountID uuid.UUID `json:"account_id"`
	PlanCode  string    `json:"plan_code"`
	Status    string    `json:"status"`
	StartsAt  int64     `json:"starts_at"`
	EndsAt    int64     `json:"ends_at"`
	AutoRenew bool      `json:"auto_renew"`
}
