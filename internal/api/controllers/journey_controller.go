package controllers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"net/http"
	"strconv"
	"time"
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

// UpdateSelectedPoiInActivity godoc
// @Summary Update selected POI in activity
// @Description Update the selected POI in an activity with the given start and end times
// @Tags Journey
// @Accept json
// @Produce json
// @Param request body request_models.UpdatePoiInActivityRequest true "Activity ID, POI ID, Start Time, End Time"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Security BearerAuth
// @Router /journeys/update-poi-in-activity [post]
func (j *JourneyController) UpdateSelectedPoiInActivity(c *gin.Context) {
	var req request_models.UpdatePoiInActivityRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.ActivityID == "" || req.CurrentPoiID == "" {
		utils.RespondError(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	startTime, err := time.Parse(time.RFC3339, req.StartTime)
	if err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid start time format")
		return
	}

	endTime, err := time.Parse(time.RFC3339, req.EndTime)
	if err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid end time format")
		return
	}

	activityID, err := uuid.Parse(req.ActivityID)
	if err != nil {
		utils.RespondError(c, http.StatusBadRequest, "Invalid activity ID")
		return
	}

	err = j.journeyService.UpdateSelectedPoiInActivity(c.Request.Context(), activityID, req.CurrentPoiID, startTime, endTime)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, nil, "POI updated successfully")
}

// AddDayToJourney godoc
// @Summary Add a day to a journey
// @Description Add a new day to a specific journey
// @Tags Journey
// @Accept json
// @Produce json
// @Param request body request_models.AddDayToJourneyRequest true "Journey ID"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Security BearerAuth
// @Router /journeys/add-day-to-journey [post]
func (j *JourneyController) AddDayToJourney(c *gin.Context) {
	var req request_models.AddDayToJourneyRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.JourneyID == "" {
		utils.RespondError(c, http.StatusBadRequest, "Journey ID is required")
		return
	}

	newDayID, err := j.journeyService.AddDayToJourney(c.Request.Context(), req.JourneyID)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, gin.H{"new_day_id": newDayID}, "Day added to journey successfully")
}

// UpdateJourneyWindow godoc
// @Summary Update journey window
// @Description Update the start and end dates of a journey, scaling the journey days accordingly
// @Tags Journey
// @Accept json
// @Produce json
// @Param request body request_models.UpdateJourneyWindowRequest true "Journey ID, Start Date, End Date"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Security BearerAuth
// @Router /journeys/update-journey-window [post]
func (j *JourneyController) UpdateJourneyWindow(c *gin.Context) {
	var req request_models.UpdateJourneyWindowRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.JourneyID == "" {
		utils.RespondError(c, http.StatusBadRequest, "journey_id, start, end are required (RFC3339)")
		return
	}

	id, added, removed, err := j.journeyService.UpdateJourneyWindow(
		c.Request.Context(), req.JourneyID, req.Start, req.End,
	)
	if err != nil {
		utils.HandleServiceError(c, err)
		return
	}

	utils.RespondSuccess(c, gin.H{
		"journey_id":        id,
		"days_added":        added,
		"days_removed_soft": removed, // soft-deleted
		"message":           "Journey days scaled to window",
	}, "Journey window updated")
}
