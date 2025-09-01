package repositories

import (
	"context"
	"gorm.io/gorm"
	"vivu/internal/models/db_models"
)

type ProvinceRepository interface {
	InsertTx(province *db_models.Province, ctx context.Context) (string, error)
	UpdateTx(province *db_models.Province, ctx context.Context) error
	GetListOfProvinces(ctx context.Context, page int, pageSize int) ([]db_models.Province, error)
	SearchByKeyword(ctx context.Context, keyword string, page int, pageSize int) ([]db_models.Province, error)
	FindRevelantProvinceIdByGivenName(ctx context.Context, name string) (*db_models.Province, error)
}

type provinceRepository struct {
	db *gorm.DB
}

func (p *provinceRepository) FindRevelantProvinceIdByGivenName(ctx context.Context, name string) (*db_models.Province, error) {

	var pois *db_models.Province
	err := p.db.WithContext(ctx).
		Where("LOWER(name) like ?", name).
		Find(&pois).Error

	if err != nil {
		return nil, err
	}

	return pois, nil

}

func NewProvinceRepository(db *gorm.DB) ProvinceRepository {
	return &provinceRepository{db: db}
}

func (p *provinceRepository) InsertTx(province *db_models.Province, ctx context.Context) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (p *provinceRepository) UpdateTx(province *db_models.Province, ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (p *provinceRepository) GetListOfProvinces(ctx context.Context, page int, pageSize int) ([]db_models.Province, error) {
	//TODO implement me
	panic("implement me")
}

func (p *provinceRepository) SearchByKeyword(ctx context.Context, keyword string, page int, pageSize int) ([]db_models.Province, error) {
	//TODO implement me
	panic("implement me")
}
