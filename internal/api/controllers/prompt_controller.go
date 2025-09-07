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

	createdPrompt, err := p.promptService.CreateNarrativeAIPlan(ctx, req.Prompt)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, createdPrompt, "Travel plan created successfully")
}

func (p *PromptController) StartQuizHandler(c *gin.Context) {
	var req request_models.QuizStartRequest // { "user_id": "u123" }
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == "" {
		utils.RespondError(c, http.StatusBadRequest, "user_id is required")
		return
	}
	resp, err := p.promptService.StartTravelQuiz(c.Request.Context(), req.UserID)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}
	utils.RespondSuccess(c, resp, "Quiz started")
}

func (p *PromptController) AnswerQuizHandler(c *gin.Context) {
	var req request_models.QuizRequest // { "session_id": "...", "answers": {...} }
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
		utils.RespondError(c, http.StatusBadRequest, "session_id is required")
		return
	}
	resp, err := p.promptService.ProcessQuizAnswer(c.Request.Context(), req)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}
	utils.RespondSuccess(c, resp, "Answer accepted")
}

// POST /api/quiz/plan-only
func (p *PromptController) PlanOnlyHandler(c *gin.Context) {
	var req request_models.PlanOnlyRequest // { "session_id": "..." }
	if err := c.ShouldBindJSON(&req); err != nil || req.SessionID == "" {
		utils.RespondError(c, http.StatusBadRequest, "session_id is required")
		return
	}
	plan, err := p.promptService.GeneratePlanOnly(c.Request.Context(), req.SessionID)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}
	utils.RespondSuccess(c, plan, "Plan-only generated")
}
