package db_models

import "github.com/google/uuid"

type POIDetail struct {
	BaseModel
	POIID       uuid.UUID `gorm:"unique"`
	Description string
	Images      []string `gorm:"type:text[]"`
	Reviews     string   `gorm:"type:jsonb"`
}
