package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"vivu/internal/models/request_models"
	"vivu/internal/services"
	"vivu/pkg/utils"
)

type FeedbackController struct {
	feedbackService services.FeedbackServiceInterface
}

func NewFeedbackController(feedbackService services.FeedbackServiceInterface) *FeedbackController {
	return &FeedbackController{feedbackService: feedbackService}
}

// AddFeedback godoc
// @Summary Add feedback
// @Description Add a comment and rating for the app
// @Tags Feedback
// @Accept json
// @Produce json
// @Param request body request_models.AddFeedbackRequest true "Feedback payload"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Router /feedback/add [post]
func (f *FeedbackController) AddFeedback(c *gin.Context) {
	var req request_models.AddFeedbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid user ID")
		return
	}

	err = f.feedbackService.AddFeedback(c.Request.Context(), userID, req.Comment, req.Rating)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, nil, "Feedback added successfully")
}

// ListFeedback godoc
// @Summary List feedback
// @Description Get a paginated list of feedback
// @Tags Feedback
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(10) minimum(1) maximum(100)
// @Success 200 {array} db_models.Feedback
// @Router /feedback/list [get]
func (f *FeedbackController) ListFeedback(c *gin.Context) {
	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "10")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		utils.RespondError(c, http.StatusBadRequest, "Invalid page number")
		return
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		utils.RespondError(c, http.StatusBadRequest, "Invalid page size")
		return
	}

	feedbacks, err := f.feedbackService.GetFeedback(c.Request.Context(), page, pageSize)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, feedbacks, "Feedback fetched successfully")
}
