package db_models

type Tag struct {
	BaseModel
	EnName string `gorm:"unique"`
	ViName string `gorm:"unique"`
	Icon   string
	POIs   []POI `gorm:"many2many:poi_tags"`
}
