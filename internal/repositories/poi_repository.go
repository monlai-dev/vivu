package repositories

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"vivu/internal/models/db_models"
)

type POIRepository interface {
	CreatePoi(ctx context.Context, poi *db_models.POI) (uuid.UUID, error)
	UpdatePoi(ctx context.Context, poi *db_models.POI) error
	Delete(ctx context.Context, id uuid.UUID) error

	GetByIDWithDetails(ctx context.Context, id string) (*db_models.POI, error)
	List(ctx context.Context, page, pageSize int) ([]db_models.POI, error)
	ListPoisByProvinceId(ctx context.Context, provinceID string, page, pageSize int) ([]db_models.POI, error)
}

type poiRepository struct {
	db *gorm.DB
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
		Where("province_id = ?", provinceID).
		Offset(offset).
		Limit(pageSize).
		Preload("Details").
		Preload("Tags").
		Find(&pois).Error
	if err != nil {
		return nil, err
	}
	return pois, nil
}
