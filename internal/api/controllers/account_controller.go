package controllers

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"vivu/internal/models/request_models"
	"vivu/internal/services"
	"vivu/pkg/utils"
)

type AccountController struct {
	accountService services.AccountServiceInterface
}

func NewAccountController(accountService services.AccountServiceInterface) *AccountController {
	return &AccountController{
		accountService: accountService,
	}
}

// Register godoc
// @Summary Register a new account
// @Description Create a new user account
// @Tags Accounts
// @Accept json
// @Produce json
// @Param request body request_models.SignUpRequest true "Account registration payload"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Router /accounts/register [post]
func (a *AccountController) Register(c *gin.Context) {
	var req request_models.SignUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	if err := a.accountService.CreateAccount(req); err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, nil, "Account created successfully")
}

// Login godoc
// @Summary Login to an account
// @Description Authenticate a user and return a token
// @Tags Accounts
// @Accept json
// @Produce json
// @Param request body request_models.LoginRequest true "Login payload"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Router /accounts/login [post]
func (a *AccountController) Login(c *gin.Context) {
	var req request_models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	ctx := context.Background()

	token, err := a.accountService.Login(req, ctx)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c,
		gin.H{"token": token.Token, "isUserHavePremium": token.IsUserHavePremium},
		"Login successful")
}

// ForgotPassword handles the forgot password functionality.
// @Summary Request a password reset
// @Description Sends a password reset link to the provided email if it exists
// @Tags Accounts
// @Accept json
// @Produce json
// @Param request body request_models.RequestForgotPassword true "Forgot password payload"
// @Success 200 {object} utils.APIResponse
// @Router /accounts/forgot-password [post]
func (a *AccountController) ForgotPassword(c *gin.Context) {
	var req request_models.RequestForgotPassword
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	err := a.accountService.ForgotPassword(req.Email)
	if err != nil {
		utils.HandleServiceError(c, err)
	}

	utils.RespondSuccess(c, nil, "If the email exists, a reset link has been sent")
}

// VerifyOtpToken handles the verification of an OTP token.
// @Summary Verify an OTP token
// @Description Validates the provided OTP token for account verification
// @Tags Accounts
// @Accept json
// @Produce json
// @Param request body request_models.RequestVerifyOtpToken true "OTP token verification payload"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Router /accounts/verify-otp [post]
func (a *AccountController) VerifyOtpToken(c *gin.Context) {
	var req request_models.RequestVerifyOtpToken
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	err := a.accountService.VerifyOtpToken(req)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, nil, "Otp token verified successfully")
}

// ResetPasswordWithOtp handles the password reset using an OTP token.
// @Summary Reset password with OTP
// @Description Resets the user's password using a valid OTP token
// @Tags Accounts
// @Accept json
// @Produce json
// @Param request body request_models.ForgotPasswordRequest true "Password reset payload"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Router /accounts/reset-password [post]
func (a *AccountController) ResetPasswordWithOtp(c *gin.Context) {
	var req request_models.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	_, err := a.accountService.VerifyAndConsumeResetToken(req)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, nil, "Password has been reset successfully")
}

// GetAllAccounts godoc
// @Summary Get all accounts
// @Description Fetch a list of all user accounts
// @Tags Accounts
// @Accept json
// @Produce json
// @Success 200 {object} utils.APIResponse
// @Security BearerAuth
// @Router /accounts/all [get]
func (a *AccountController) GetAllAccounts(c *gin.Context) {

	accounts, err := a.accountService.GetAllAccounts(context.Background())
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, accounts, "Accounts fetched successfully")
}
