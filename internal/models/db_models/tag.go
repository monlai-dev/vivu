package db_models

type Tag struct {
	BaseModel
	Name   string `gorm:"unique"`
	ViName string `gorm:"unique"`
	Icon   string
	POIs   []POI `gorm:"many2many:poi_tags"`
}
