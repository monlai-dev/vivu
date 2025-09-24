package response_models

import (
	"github.com/google/uuid"
)

// Top-level payload returned to FE
type JourneyDetailResponse struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	StartDate    string    `json:"start_date"`    // RFC3339 date/time
	EndDate      string    `json:"end_date"`      // RFC3339 date/time
	DurationDays int       `json:"duration_days"` // computed (inclusive or exclusive—your call; see mapper)
	IsShared     bool      `json:"is_shared"`
	IsCompleted  bool      `json:"is_completed"`

	// Quick stats
	TotalDays       int `json:"total_days"`
	TotalActivities int `json:"total_activities"`

	// Plan details
	Days []JourneyDayResponse `json:"days"`
}

// One day in the journey
type JourneyDayResponse struct {
	ID         uuid.UUID               `json:"id"`
	DayNumber  int                     `json:"day_number"`
	Date       string                  `json:"date"` // RFC3339 date
	Activities []JourneyActivityDetail `json:"activities"`
}

// Activity inside a day
type JourneyActivityDetail struct {
	ID           uuid.UUID   `json:"id"`
	Time         string      `json:"time"` // RFC3339 date/time
	ActivityType string      `json:"activity_type"`
	Notes        string      `json:"notes,omitempty"`
	SelectedPOI  *POISummary `json:"selected_poi,omitempty"`
}

// Minimal POI info that's useful on UI
type POISummary struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address,omitempty"`
	Latitude  float64   `json:"latitude,omitempty"`
	Longitude float64   `json:"longitude,omitempty"`
	Status    string    `json:"status,omitempty"`
}
