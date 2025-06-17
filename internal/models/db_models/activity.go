package db_models

import (
	"github.com/google/uuid"
	"time"
)

type JourneyDay struct {
	BaseModel
	JourneyID uuid.UUID
	Date      time.Time
	DayNumber int

	Activities []JourneyActivity
}

type JourneyActivity struct {
	BaseModel
	JourneyDayID  uuid.UUID
	Time          time.Time
	ActivityType  string
	SelectedPOIID uuid.UUID
	Notes         string

	//Recommendations []ActivityRecommendation
}
