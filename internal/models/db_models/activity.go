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

	Journey    Journey           `gorm:"foreignKey:JourneyID"`
	Activities []JourneyActivity `gorm:"foreignKey:JourneyDayID"`
}

type JourneyActivity struct {
	BaseModel
	JourneyDayID  uuid.UUID
	Time          time.Time
	ActivityType  string
	SelectedPOIID uuid.UUID
	Notes         string

	JourneyDay  JourneyDay `gorm:"foreignKey:JourneyDayID"`
	SelectedPOI POI        `gorm:"foreignKey:SelectedPOIID"`
}
