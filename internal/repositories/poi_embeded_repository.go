package repositories

import (
	"github.com/pgvector/pgvector-go"
	"gorm.io/gorm"
	"vivu/internal/models/db_models"
)

type IPoiEmbededRepository interface {
	GetPoiEmbededByID(poiEmbededID int) (poiEmbeded db_models.PoiEmbedding, err error)
	GetListOfPoiEmbededByVector(vector pgvector.Vector, filter interface{}) (poiEmbededs []db_models.PoiEmbedding, err error)
	CreatePoiEmbeded(poiEmbeded db_models.PoiEmbedding) error
}

type PoiEmbededRepository struct {
	db *gorm.DB
}

func (p *PoiEmbededRepository) GetPoiEmbededByID(poiEmbededID int) (poiEmbeded db_models.PoiEmbedding, err error) {
	panic("implement me")
}

func (p *PoiEmbededRepository) GetListOfPoiEmbededByVector(vector pgvector.Vector, filter interface{}) ([]db_models.PoiEmbedding, error) {
	var results []db_models.PoiEmbedding

	vecStr := vector.String()

	query := `
        SELECT *, (1 - (embedding <=> $1)) as similarity
        FROM poi_embeddings
        WHERE (1 - (embedding <=> $1)) > 0.7  -- Only return results with >70% similarity
        ORDER BY embedding <=> $1  -- Cosine distance (closer to 0 is better)
        LIMIT 15
    `

	err := p.db.Raw(query, vecStr).Scan(&results).Error

	if err != nil {
		return nil, err
	}
	return results, nil
}

func (p *PoiEmbededRepository) CreatePoiEmbeded(poiEmbeded db_models.PoiEmbedding) error {
	return p.db.Create(&poiEmbeded).Error
}

func NewPoiEmbededRepository(db *gorm.DB) IPoiEmbededRepository {
	return &PoiEmbededRepository{
		db: db,
	}
}
