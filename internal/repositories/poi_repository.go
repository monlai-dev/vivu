package repositories

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"strings"
	"vivu/internal/models/db_models"
)

type POIRepository interface {
	CreatePoi(ctx context.Context, poi *db_models.POI) (uuid.UUID, error)
	UpdatePoi(ctx context.Context, poi *db_models.POI) error
	Delete(ctx context.Context, id uuid.UUID) error

	GetByIDWithDetails(ctx context.Context, id string) (*db_models.POI, error)
	List(ctx context.Context, page, pageSize int) ([]db_models.POI, error)
	ListPoisByProvinceId(ctx context.Context, provinceID string, page, pageSize int) ([]db_models.POI, error)
	ListPoisByPoisId(ctx context.Context, ids []string) ([]*db_models.POI, error)

	SearchPOIsByName(ctx context.Context, name string) ([]*db_models.POI, error)
	SearchPOIsByKeywords(ctx context.Context, keywords []string) ([]*db_models.POI, error)
	FindPOIsByLocationNames(ctx context.Context, locations []string) ([]*db_models.POI, error)

	SearchPoiByNameAndProvince(ctx context.Context, name string, provinceID string) ([]*db_models.POI, error)
}

type poiRepository struct {
	db *gorm.DB
}

func (r *poiRepository) SearchPoiByNameAndProvince(ctx context.Context, name string, provinceID string) ([]*db_models.POI, error) {

	var pois []*db_models.POI

	// Clean and prepare search term
	searchTerm := "%" + strings.ToLower(strings.TrimSpace(name)) + "%"

	err := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("Category").
		Preload("Province").
		Where("LOWER(name) LIKE ?", searchTerm).
		Limit(50).
		Find(&pois).Error

	if err != nil {
		return nil, fmt.Errorf("failed to search POIs by name and province: %w", err)
	}

	return pois, nil
}

func (r *poiRepository) ListPoisByPoisId(ctx context.Context, ids []string) ([]*db_models.POI, error) {
	var pois []*db_models.POI
	err := r.db.WithContext(ctx).
		Preload("Details").
		Preload("Tags").
		Preload("Category").
		Preload("Province").
		Where("id in ?", ids).
		Find(&pois).Error

	if err != nil {
		return nil, err
	}

	return pois, nil
}

func (r *poiRepository) SearchPOIsByName(ctx context.Context, name string) ([]*db_models.POI, error) {
	var pois []*db_models.POI

	// Clean and prepare search term
	searchTerm := "%" + strings.ToLower(strings.TrimSpace(name)) + "%"

	err := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("Category").
		Preload("Province").
		Where("LOWER(name) LIKE ?", searchTerm).
		Limit(10).
		Find(&pois).Error

	if err != nil {
		return nil, fmt.Errorf("failed to search POIs by name: %w", err)
	}

	return pois, nil
}

func (r *poiRepository) SearchPOIsByKeywords(ctx context.Context, keywords []string) ([]*db_models.POI, error) {
	if len(keywords) == 0 {
		return nil, fmt.Errorf("no keywords provided")
	}

	var pois []*db_models.POI

	query := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("Category").
		Preload("Province").
		Joins("LEFT JOIN categories ON pois.category_id = categories.id")

	// Build OR conditions for each keyword
	var conditions []string
	var args []interface{}

	for _, keyword := range keywords {
		searchTerm := "%" + strings.ToLower(strings.TrimSpace(keyword)) + "%"
		conditions = append(conditions, "(LOWER(pois.name) LIKE ? OR LOWER(pois.description) LIKE ? OR LOWER(categories.name) LIKE ?)")
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	if len(conditions) > 0 {
		whereClause := strings.Join(conditions, " OR ")
		query = query.Where(whereClause, args...)
	}

	err := query.Limit(10).Find(&pois).Error
	if err != nil {
		return nil, fmt.Errorf("failed to search POIs by keywords: %w", err)
	}

	return pois, nil
}

func (r *poiRepository) FindPOIsByLocationNames(ctx context.Context, locations []string) ([]*db_models.POI, error) {
	if len(locations) == 0 {
		return nil, fmt.Errorf("no locations provided")
	}

	var pois []*db_models.POI

	query := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("Category").
		Preload("Province").
		Joins("LEFT JOIN provinces ON pois.province_id = provinces.id")

	// Build OR conditions for each location
	var conditions []string
	var args []interface{}

	for _, location := range locations {
		searchTerm := "%" + strings.ToLower(strings.TrimSpace(location)) + "%"
		// Search in POI name, address, and province name
		conditions = append(conditions, "(LOWER(pois.name) LIKE ? OR LOWER(pois.address) LIKE ? OR LOWER(provinces.name) LIKE ?)")
		args = append(args, searchTerm, searchTerm, searchTerm)
	}

	if len(conditions) > 0 {
		whereClause := strings.Join(conditions, " OR ")
		query = query.Where(whereClause, args...)
	}

	err := query.Limit(30).Find(&pois).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find POIs by location names: %w", err)
	}

	return pois, nil
}

func NewPOIRepository(db *gorm.DB) POIRepository {
	return &poiRepository{db: db}
}

func (r *poiRepository) CreatePoi(ctx context.Context, poi *db_models.POI) (uuid.UUID, error) {
	if err := r.db.WithContext(ctx).Create(poi).Error; err != nil {
		return uuid.Nil, err
	}
	return poi.ID, nil
}

func (r *poiRepository) UpdatePoi(ctx context.Context, poi *db_models.POI) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Save(poi)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return nil
			}
			return fmt.Errorf("failed to update POI: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		return nil
	})
}

func (r *poiRepository) Delete(ctx context.Context, id uuid.UUID) error {
	err := r.db.WithContext(ctx).Delete(&db_models.POI{}, "id = ?", id).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return nil
}

// ────────────────────────────────────────────────────────────────
// Read helpers follow the same pattern: default value + nil error
// when no rows are found.
// ────────────────────────────────────────────────────────────────

func (r *poiRepository) GetByIDWithDetails(ctx context.Context, id string) (*db_models.POI, error) {
	var poi db_models.POI
	err := r.db.WithContext(ctx).
		Preload("Details").
		Preload("Tags").
		Preload("Category").
		Preload("Province").
		First(&poi, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil // default model
		}
		return nil, err
	}
	return &poi, nil
}

func (r *poiRepository) List(ctx context.Context, page, pageSize int) ([]db_models.POI, error) {
	var pois []db_models.POI
	offset := (page - 1) * pageSize

	err := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("Category").
		Preload("Province").
		Offset(offset).
		Limit(pageSize).
		Find(&pois).Error

	if err != nil {
		return nil, err
	}
	return pois, nil
}

func (r *poiRepository) ListPoisByProvinceId(ctx context.Context, provinceID string, page, pageSize int) ([]db_models.POI, error) {
	var pois []db_models.POI
	offset := (page - 1) * pageSize

	err := r.db.WithContext(ctx).
		Preload("Tags").
		Preload("Category").
		Preload("Province").
		Preload("Details").
		Where("province_id = ?", provinceID).
		Offset(offset).
		Limit(pageSize).
		Find(&pois).Error
	if err != nil {
		return nil, err
	}
	return pois, nil
}
