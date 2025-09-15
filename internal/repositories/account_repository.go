package repositories

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"vivu/internal/models/db_models"
)

type AccountRepository interface {
	InsertTx(account *db_models.Account, ctx context.Context) error
	FindById(ctx context.Context, id string) (*db_models.Account, error)
	FindByEmailAndPassword(ctx context.Context, email, password string) (*db_models.Account, error)
	FindByEmail(ctx context.Context, email string) (*db_models.Account, error)
}

type accountRepository struct {
	db *gorm.DB
}

func (a *accountRepository) FindByEmail(ctx context.Context, email string) (*db_models.Account, error) {

	var account db_models.Account
	err := a.db.WithContext(ctx).First(&account, "email = ?", email).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &account, nil
}

func NewAccountRepository(db *gorm.DB) AccountRepository {
	return &accountRepository{
		db: db,
	}
}

func (a *accountRepository) InsertTx(account *db_models.Account, ctx context.Context) error {
	return a.db.WithContext(ctx).Create(account).Error
}

func (a *accountRepository) FindById(ctx context.Context, id string) (*db_models.Account, error) {
	var account db_models.Account
	err := a.db.WithContext(ctx).First(&account, "id = ?", id).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &account, nil
}

func (a *accountRepository) FindByEmailAndPassword(ctx context.Context, email, password string) (*db_models.Account, error) {
	var account db_models.Account
	err := a.db.WithContext(ctx).
		Where("email = ? AND password_hash = ?", email, password).
		First(&account).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &account, nil
}
