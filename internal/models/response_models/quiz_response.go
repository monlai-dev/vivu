package response_models

import (
	"time"
	"vivu/internal/models/request_models"
)

type QuizResponse struct {
	Questions    []request_models.QuizQuestion `json:"questions"`
	CurrentStep  int                           `json:"current_step"`
	TotalSteps   int                           `json:"total_steps"`
	SessionID    string                        `json:"session_id"`
	IsComplete   bool                          `json:"is_complete"`
	NextEndpoint string                        `json:"next_endpoint,omitempty"`
}

type QuizResultResponse struct {
	SessionID       string                       `json:"session_id"`
	UserProfile     TravelProfile                `json:"user_profile"`
	Itinerary       *TravelItinerary             `json:"itinerary"`
	Recommendations []PersonalizedRecommendation `json:"recommendations"`
}

type TravelProfile struct {
	TravelStyle   []string `json:"travel_style"`
	BudgetRange   string   `json:"budget_range"`
	Interests     []string `json:"interests"`
	Accommodation string   `json:"accommodation_preference"`
	DiningStyle   string   `json:"dining_style"`
	ActivityLevel string   `json:"activity_level"`
	Destination   string   `json:"destination"`
	Duration      int      `json:"duration"`
}

type PersonalizedRecommendation struct {
	Type        string    `json:"type"` // "must_visit", "hidden_gem", "local_favorite", "budget_friendly"
	Title       string    `json:"title"`
	Description string    `json:"description"`
	POI         TravelPOI `json:"poi"`
	Reason      string    `json:"reason"` // Why this is recommended based on quiz answers
}

type PlanOnly struct {
	Destination string        `json:"destination"`
	Duration    int           `json:"duration_days"`
	Days        []PlanOnlyDay `json:"days"`
	CreatedAt   time.Time     `json:"created_at"`
}

type PlanOnlyDay struct {
	Day        int                `json:"day"`
	Activities []PlanOnlyActivity `json:"activities"`
}

type PlanOnlyActivity struct {
	StartTime string `json:"start_time"` // "09:00"
	EndTime   string `json:"end_time"`   // "11:00"
	MainPOIID string `json:"main_poi_id"`
	// Optional future fields:
	// WindowStart, WindowEnd, AltPOIIDs []string

	MainPOI *POI `json:"main_poi,omitempty"`
}
