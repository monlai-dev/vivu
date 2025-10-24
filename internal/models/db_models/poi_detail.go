package db_models

import (
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type POIDetail struct {
	BaseModel
	POIID  uuid.UUID      `gorm:"type:uuid;not null"`
	Images pq.StringArray `gorm:"type:text[]"`
}

func (POIDetail) TableName() string {
	return "poi_details"
}
