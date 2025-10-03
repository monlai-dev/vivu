package request_models

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type SignUpRequest struct {
	DisplayName string `json:"display_name" binding:"required,min=3,max=50"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=6"`
}

type ForgotPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
	Token       string `json:"token" binding:"required"`
}

type RequestForgotPassword struct {
	Email string `json:"email" binding:"required,email"`
}
