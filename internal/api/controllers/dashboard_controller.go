package controllers

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"strconv"
	"time"
	"vivu/internal/models/response_models"
	"vivu/internal/services"
	"vivu/pkg/utils"
)

type DashboardController struct {
	dashboardService services.DashboardService
}

func NewDashboardController(dashboardService services.DashboardService) *DashboardController {
	return &DashboardController{
		dashboardService: dashboardService,
	}
}

// GetDashboard godoc
// @Summary Get dashboard report
// @Description Fetch KPI blocks, revenue/new users/subscriptions series, plan mix, top destinations, and recent payments
// @Tags Dashboard
// @Accept json
// @Produce json
// @Param start    query string false "RFC3339 start (e.g. 2025-10-01T00:00:00Z)"
// @Param end      query string false "RFC3339 end   (e.g. 2025-10-19T23:59:59Z)"
// @Param last_days query int   false "Relative lookback in days (mutually exclusive with start/end). Default 30"
// @Param interval query string false "Bucket size: day | week | month (default: day)"
// @Param tz       query string false "IANA timezone for bucketing (default: Asia/Ho_Chi_Minh)"
// @Param currency query string false "ISO 4217 currency code for labeling (default: VND)"
// @Success 200 {object} utils.APIResponse
// @Failure 400 {object} utils.APIResponse
// @Failure 500 {object} utils.APIResponse
// @Security BearerAuth
// @Router /dashboard/stats [get]
func (p *DashboardController) GetDashboard(c *gin.Context) {
	// Defaults
	interval := c.DefaultQuery("interval", "day")
	tz := c.DefaultQuery("tz", "Asia/Ho_Chi_Minh")
	currency := c.DefaultQuery("currency", "VND")

	if !validInterval(interval) {
		utils.RespondError(c, http.StatusBadRequest, "interval must be one of: day, week, month")
		return
	}

	var (
		start, end time.Time
		err        error
	)

	startStr := c.Query("start")
	endStr := c.Query("end")
	lastDaysStr := c.Query("last_days")

	// Validate mutual exclusivity
	if lastDaysStr != "" && (startStr != "" || endStr != "") {
		utils.RespondError(c, http.StatusBadRequest, "provide either last_days or start/end (not both)")
		return
	}

	switch {
	case lastDaysStr != "":
		d, convErr := strconv.Atoi(lastDaysStr)
		if convErr != nil || d <= 0 {
			utils.RespondError(c, http.StatusBadRequest, "last_days must be a positive integer")
			return
		}
		end = time.Now().UTC()
		start = end.AddDate(0, 0, -d)

	default:
		// Parse explicit start/end, with sensible defaults
		if startStr != "" {
			start, err = time.Parse(time.RFC3339, startStr)
			if err != nil {
				utils.RespondError(c, http.StatusBadRequest, "start must be RFC3339 (e.g. 2025-10-01T00:00:00Z)")
				return
			}
		}
		if endStr != "" {
			end, err = time.Parse(time.RFC3339, endStr)
			if err != nil {
				utils.RespondError(c, http.StatusBadRequest, "end must be RFC3339 (e.g. 2025-10-19T23:59:59Z)")
				return
			}
		}
		if end.IsZero() {
			end = time.Now().UTC()
		}
		if start.IsZero() {
			start = end.AddDate(0, 0, -30) // default 30-day window
		}
	}

	// Normalize ordering
	if start.After(end) {
		start, end = end, start
	}

	tr := response_models.TimeRange{
		Start:    start,
		End:      end,
		Interval: interval,
		Timezone: tz,
	}

	report, svcErr := p.dashboardService.BuildDashboard(c.Request.Context(), tr, currency)
	if svcErr != nil {
		utils.HandleServiceError(c, svcErr)
		return
	}

	utils.RespondSuccess(c, report, "Dashboard data fetched successfully")
}

// ---- helpers ----

func validInterval(s string) bool {
	switch s {
	case "day", "week", "month":
		return true
	default:
		return false
	}
}
