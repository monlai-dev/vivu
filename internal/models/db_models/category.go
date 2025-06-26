package db_models

import _ "github.com/google/uuid"

// Category represents a POI category as a separate table
// with a UUID primary key and a Name field.
type Category struct {
	BaseModel
	Name string `gorm:"unique;not null"`
	POIs []POI  `gorm:"foreignKey:CategoryID"`
}
