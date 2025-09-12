package repositories

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"vivu/internal/models/db_models"
	"vivu/pkg/utils"
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

func NewProvinceRepository(db *gorm.DB) ProvinceRepository {
	return &provinceRepository{db: db}
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

func (p *provinceRepository) InsertTx(province *db_models.Province, ctx context.Context) (string, error) {
	if err := p.db.WithContext(ctx).Create(province).Error; err != nil {
		return "", utils.ErrDatabaseError
	}
	return province.ID.String(), nil
}

func (p *provinceRepository) UpdateTx(province *db_models.Province, ctx context.Context) error {
	return p.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Save(province)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("failed to update province: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return nil
	})
}

func (p *provinceRepository) GetListOfProvinces(ctx context.Context, page int, pageSize int) ([]db_models.Province, error) {
	var provinces []db_models.Province
	offset := (page - 1) * pageSize

	err := p.db.WithContext(ctx).
		Offset(offset).
		Limit(pageSize).
		Find(&provinces).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get list of provinces: %w", err)
	}

	return provinces, nil
}

func (p *provinceRepository) SearchByKeyword(ctx context.Context, keyword string, page int, pageSize int) ([]db_models.Province, error) {
	if strings.TrimSpace(keyword) == "" {
		return nil, fmt.Errorf("keyword cannot be empty")
	}

	var provinces []db_models.Province
	offset := (page - 1) * pageSize
	searchTerm := "%" + strings.ToLower(strings.TrimSpace(keyword)) + "%"

	err := p.db.WithContext(ctx).
		Where("LOWER(name) LIKE ?", searchTerm).
		Offset(offset).
		Limit(pageSize).
		Find(&provinces).Error

	if err != nil {
		return nil, fmt.Errorf("failed to search provinces by keyword: %w", err)
	}

	return provinces, nil
}
