package db_models

type Account struct {
	BaseModel
	Name         string
	Email        string `gorm:"unique"`
	PasswordHash string
	//Role??
	Journeys []Journey
	CheckIns []CheckIn
}
