package db_models

type Account struct {
	BaseModel
	Name         string
	Email        string `gorm:"unique"`
	PasswordHash string
	Role         string `gorm:"default:'user'"`

	Journeys []Journey `gorm:"foreignKey:AccountID"`
	CheckIns []CheckIn `gorm:"foreignKey:AccountID"`
}
