package db_models

import "github.com/google/uuid"

type POI struct {
	BaseModel
	Name         string
	Latitude     float64
	Longitude    float64
	ProvinceID   uuid.UUID
	CategoryID   uuid.UUID
	Category     Category `gorm:"foreignKey:CategoryID"`
	Status       string
	OpeningHours string
	ContactInfo  string
	Description  string
	Address      string
	Province     Province          // Add this relationship
	Details      POIDetail         `gorm:"foreignKey:POIID"`
	Tags         []*Tag            `gorm:"many2many:poi_tags"`
	Activities   []JourneyActivity `gorm:"foreignKey:SelectedPOIID"`
	CheckIns     []CheckIn
}
