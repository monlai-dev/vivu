package response_models

import "gorm.io/datatypes"

type AccountLoginResponse struct {
	Token             string `json:"token"`
	IsUserHavePremium bool   `json:"is_user_have_premium"`
}

type AccountResponse struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Email                string         `json:"email"`
	Role                 string         `json:"role"`
	SubscriptionSnapshot datatypes.JSON `json:"subscription_snapshot"`
}
