package controllers

import (
	"context"
	"github.com/gin-gonic/gin"
	"net/http"
	"vivu/internal/models/request_models"
	"vivu/internal/services"
	"vivu/pkg/utils"
)

type PromptController struct {
	promptService services.PromptServiceInterface
}

func NewPromptController(promptService services.PromptServiceInterface) *PromptController {
	return &PromptController{
		promptService: promptService,
	}
}

func (p *PromptController) CreatePromptHandler(c *gin.Context) {
	var req request_models.UserInputWildcard
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request format")
		return
	}

	ctx := context.Background()

	createdPrompt, err := p.promptService.CreateAIPlan(ctx, req.Prompt)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, createdPrompt, "Travel plan created successfully")
}
