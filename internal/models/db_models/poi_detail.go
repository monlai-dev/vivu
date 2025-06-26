package db_models

import "github.com/google/uuid"

type POIDetail struct {
	BaseModel
	POIID   uuid.UUID `gorm:"type:uuid;not null"`
	Images  []string  `gorm:"type:text[]"`
	Reviews string    `gorm:"type:jsonb"`
}
