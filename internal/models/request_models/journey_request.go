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
