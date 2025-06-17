package db_models

type District struct {
	BaseModel
	Name     string
	Province string

	POIs []POI
}
