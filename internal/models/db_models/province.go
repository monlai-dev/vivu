package db_models

type Province struct {
	BaseModel
	Name string
	POIs []*POI `gorm:"foreignKey:ProvinceID"` // Explicit foreign key
}
