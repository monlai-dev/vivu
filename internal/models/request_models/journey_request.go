package request_models

import "time"

type AddPoiToJourneyRequest struct {
	JourneyID string     `json:"journey_id" binding:"required,uuid4"`
	PoiID     string     `json:"poi_id" binding:"required,uuid4"`
	StartTime time.Time  `json:"start_time"`
	EndTime   *time.Time `json:"end_time"`
}

type RemovePoiFromJourneyRequest struct {
	JourneyID string `json:"journey_id" binding:"required,uuid4"`
	PoiID     string `json:"poi_id" binding:"required,uuid4"`
}

type UpdatePoiInActivityRequest struct {
	ActivityID   string `json:"activity_id" binding:"required"`
	CurrentPoiID string `json:"current_poi_id" binding:"required"`
	StartTime    string `json:"start_time" binding:"required"`
	EndTime      string `json:"end_time" binding:"required"`
}

type AddDayToJourneyRequest struct {
	JourneyID string `json:"journey_id" binding:"required"`
}

type UpdateJourneyWindowRequest struct {
	JourneyID string `json:"journey_id" binding:"required"`
	// RFC3339 (e.g., "2025-10-10T09:00:00+07:00")
	Start string `json:"start" binding:"required"`
	End   string `json:"end" binding:"required"`
}
