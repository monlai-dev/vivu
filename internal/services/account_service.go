package services

import (
	"context"
	"log"
	"time"
	"vivu/internal/models/db_models"
	"vivu/internal/models/request_models"
	"vivu/internal/repositories"
	"vivu/pkg/utils"
)

type AccountServiceInterface interface {
	Login(request request_models.LoginRequest, ctx context.Context) (string, error)
	CreateAccount(request request_models.SignUpRequest) error
}

type AccountService struct {
	accountRepo repositories.AccountRepository
}

func NewAccountService(accountRepo repositories.AccountRepository) AccountServiceInterface {
	return &AccountService{
		accountRepo: accountRepo,
	}
}

func (a *AccountService) Login(request request_models.LoginRequest, ctx context.Context) (string, error) {

	startTime := time.Now()

	account, err := a.accountRepo.FindByEmail(ctx, request.Email)
	if err != nil {
		return "", utils.ErrDatabaseError
	}

	log.Printf("Login process took %s", time.Since(startTime))

	if account == nil {
		return "", utils.ErrAccountNotFound
	}

	err = utils.ComparePasswords(account.PasswordHash, request.Password)
	if err != nil {
		return "", utils.ErrInvalidCredentials
	}

	log.Printf("Password verification took %s", time.Since(startTime))

	token, err := utils.CreateToken(account.ID, account.Role)
	if err != nil {
		return "", utils.ErrInvalidCredentials
	}

	log.Printf("Token generation took %s", time.Since(startTime))

	return token, nil
}

func (a *AccountService) CreateAccount(request request_models.SignUpRequest) error {

	existingAccount, err := a.accountRepo.FindByEmail(context.Background(), request.Email)
	if err != nil {
		return utils.ErrDatabaseError
	}
	if existingAccount != nil {
		return utils.ErrEmailAlreadyExists
	}

	hashedPassword, err := utils.HashPassword(request.Password)
	if err != nil {
		return utils.ErrDatabaseError
	}

	newAccount := &db_models.Account{
		Name:         request.DisplayName,
		Email:        request.Email,
		PasswordHash: hashedPassword,
		Role:         "user", // default role
	}

	if err := a.accountRepo.InsertTx(newAccount, context.Background()); err != nil {
		return utils.ErrDatabaseError
	}

	return nil
}
