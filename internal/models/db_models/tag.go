package db_models

import "github.com/google/uuid"

type Tag struct {
	BaseModel
	Name string `gorm:"unique"`
	POIs []POI  `gorm:"many2many:poi_tags"`
}

type POITag struct {
	POIID uuid.UUID `gorm:"primaryKey"`
	TagID uuid.UUID `gorm:"primaryKey"`
}
