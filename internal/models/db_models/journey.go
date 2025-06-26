package db_models

import (
	"github.com/google/uuid"
)

type Journey struct {
	BaseModel
	AccountID   uuid.UUID // Change from UserID
	Title       string
	StartDate   int64
	EndDate     int64
	IsShared    bool
	IsCompleted bool

	Account  Account      `gorm:"foreignKey:AccountID"`
	Days     []JourneyDay `gorm:"foreignKey:JourneyID"`
	CheckIns []CheckIn    `gorm:"foreignKey:JourneyID"`
}
