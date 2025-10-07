package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"vivu/internal/models/request_models"
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

// AddPoiToJourney godoc
// @Summary Add POI to journey
// @Description Add a point of interest (POI) to a specific journey with optional start and end times
// @Tags Journey
// @Accept json
// @Produce json
// @Param request body request_models.AddPoiToJourneyRequest true "Journey ID, POI ID, Start Time, End Time"
// @Success 200 {object} utils.APIResponse
// @Security BearerAuth
// @Example {json} Request Body Example:
//
//	{
//	  "journey_id": "123e4567-e89b-12d3-a456-426614174000",
//	  "poi_id": "poi-001",
//	  "start_time": "2023-10-01T10:00:00Z",
//	  "end_time": "2023-10-01T18:00:00Z"
//	}
//
// @Router /journeys/add-poi-to-journey [post]
func (j *JourneyController) AddPoiToJourney(c *gin.Context) {

	var req request_models.AddPoiToJourneyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.JourneyID == "" || req.PoiID == "" {
		utils.RespondError(c, http.StatusBadRequest, "JourneyID and PoiID are required")
		return
	}

	err := j.journeyService.AddPoiToJourneyWithGivenStartAndEndDate(c.Request.Context(), req.JourneyID, req.PoiID, req.StartTime, *req.EndTime)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, nil, "POI added to journey successfully")
}

// RemovePoiFromJourney godoc
// @Summary Remove POI from journey
// @Description Remove a point of interest (POI) from a specific journey
// @Tags Journey
// @Accept json
// @Produce json
// @Param request body request_models.RemovePoiFromJourneyRequest true "Journey ID, POI ID"
// @Success 200 {object} utils.APIResponse
// @Security BearerAuth
// @Router /journeys/remove-poi-from-journey [post]
func (j *JourneyController) RemovePoiFromJourney(c *gin.Context) {
	var req request_models.RemovePoiFromJourneyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.JourneyID == "" || req.PoiID == "" {
		utils.RespondError(c, http.StatusBadRequest, "JourneyID and PoiID are required")
		return
	}

	err := j.journeyService.RemovePoiFromJourney(c.Request.Context(), req.JourneyID, req.PoiID)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, nil, "POI removed from journey successfully")
}
