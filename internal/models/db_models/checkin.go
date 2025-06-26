package db_models

import "github.com/google/uuid"

type CheckIn struct {
	BaseModel
	AccountID uuid.UUID // Change from UserID
	JourneyID uuid.UUID
	POIID     uuid.UUID
	Notes     string

	Account Account `gorm:"foreignKey:AccountID"`
	Journey Journey `gorm:"foreignKey:JourneyID"`
	POI     POI     `gorm:"foreignKey:POIID"`
	Photos  []Photo `gorm:"foreignKey:CheckInID"`
}
