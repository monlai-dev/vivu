package db_models

import (
	"github.com/google/uuid"
	"time"
)

type Journey struct {
	BaseModel
	UserID      uuid.UUID
	Title       string
	StartDate   time.Time
	EndDate     time.Time
	IsShared    bool
	IsCompleted bool

	Days     []JourneyDay
	CheckIns []CheckIn
}
