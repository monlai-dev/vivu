package db_models

import (
	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
	"time"
)

type PoiEmbedding struct {
	PoiID       string `gorm:"primaryKey;column:poi_id"`
	Name        string
	Description string
	ProvinceID  string
	CategoryID  string          // stores the UUID of the category
	Tags        pq.StringArray  `gorm:"type:text[]"`
	Embedding   pgvector.Vector `gorm:"type:vector(1536)"`
	CreatedAt   time.Time       `gorm:"autoCreateTime"`
}
