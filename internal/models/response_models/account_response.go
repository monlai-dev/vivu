package response_models

type AccountLoginResponse struct {
	Token             string `json:"token"`
	IsUserHavePremium bool   `json:"is_user_have_premium"`
}
