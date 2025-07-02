package repositories

import (
	"errors"
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
		SELECT *
		FROM poi_embeddings
		ORDER BY embedding <#> $1
		LIMIT 15
	`

	err := p.db.Raw(query, vecStr).Scan(&results).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

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
