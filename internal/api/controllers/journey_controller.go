package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"vivu/internal/services"
	"vivu/pkg/utils"
)

type JourneyController struct {
	journeyService services.JourneyServiceInterface
}

func NewJourneyController(journeyService services.JourneyServiceInterface) *JourneyController {
	return &JourneyController{
		journeyService: journeyService,
	}
}

// GetJourneyByUserId godoc
// @Summary Get journeys by user ID
// @Description Fetch a paginated list of journeys for the authenticated user
// @Tags Journey
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param pageSize query int false "Page size" default(5) minimum(1) maximum(100)
// @Success 200 {array} []response_models.JourneyResponse
// @Security BearerAuth
// @Router /journeys/get-journey-by-userid [get]
func (j *JourneyController) GetJourneyByUserId(c *gin.Context) {

	pageStr := c.DefaultQuery("page", "1")
	pageSizeStr := c.DefaultQuery("pageSize", "5")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		utils.RespondError(c, http.StatusBadRequest, "Invalid page number")
		return
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 100 {
		utils.RespondError(c, http.StatusBadRequest, "Invalid page size (must be 1-100)")
		return
	}

	userId := c.GetString("user_id")

	plans, err := j.journeyService.GetListOfJourneyByUserId(c.Request.Context(), page, pageSize, userId)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, plans, "Journey fetched successfully")
}

// GetDetailsInfoOfJourneyById godoc
// @Summary Get journey details by ID
// @Description Fetch detailed information about a specific journey by its ID
// @Tags Journey
// @Accept json
// @Produce json
// @Param journeyId path string true "Journey ID"
// @Success 200 {object} response_models.JourneyDetailResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 404 {object} utils.APIResponse
// @Security BearerAuth
// @Router /journeys/get-details-info-of-journey-by-id/{journeyId} [get]
func (j *JourneyController) GetDetailsInfoOfJourneyById(c *gin.Context) {
	journeyId := c.Param("journeyId")
	if journeyId == "" {
		utils.RespondError(c, http.StatusBadRequest, "Journey ID is required")
		return
	}

	journey, err := j.journeyService.GetDetailsInfoOfJourneyById(c.Request.Context(), journeyId)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, journey, "Journey details fetched successfully")
}
