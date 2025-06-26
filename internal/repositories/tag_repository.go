package repositories

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"vivu/internal/models/db_models"
)

type TagRepositoryInterface interface {
	CreateTag(tag db_models.Tag, ctx context.Context) error
	GetTagByID(tagID string) (*db_models.Tag, error)
	GetAllTags(page int, pageSize int, ctx context.Context) ([]db_models.Tag, error)
}

func NewTagRepository(db *gorm.DB) TagRepositoryInterface {
	return &TagRepository{db: db}
}

type TagRepository struct {
	db *gorm.DB
}

func (t TagRepository) CreateTag(tag db_models.Tag, ctx context.Context) error {

	return t.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).Create(&tag).Error; err != nil {
			return err
		}

		return nil
	})

}

func (t TagRepository) GetTagByID(tagID string) (*db_models.Tag, error) {

	var tag db_models.Tag
	err := t.db.WithContext(context.Background()).Where("id = ?", tagID).First(&tag).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, err
	}
	return &tag, nil
}

func (t TagRepository) GetAllTags(page int, pageSize int, ctx context.Context) ([]db_models.Tag, error) {

	var tags []db_models.Tag
	err := t.db.WithContext(ctx).Scopes(func(db *gorm.DB) *gorm.DB {
		offset := (page - 1) * pageSize
		return db.Offset(offset).Limit(pageSize)
	}).Find(&tags).Error
	if err != nil {
		return nil, err
	}
	return tags, nil
}
