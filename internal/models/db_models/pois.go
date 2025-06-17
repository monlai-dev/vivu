package db_models

import "github.com/google/uuid"

type POI struct {
	BaseModel
	Name         string
	Latitude     float64
	Longitude    float64
	DistrictID   uuid.UUID
	Category     string
	Status       string
	OpeningHours string
	ContactInfo  string

	Details    POIDetail
	Tags       []Tag             `gorm:"many2many:poi_tags"`
	Activities []JourneyActivity `gorm:"foreignKey:SelectedPOIID"`
	CheckIns   []CheckIn
}
