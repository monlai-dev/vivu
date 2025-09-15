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

	utils.RespondSuccess(c, gin.H{"token": token}, "Login successful")
}
