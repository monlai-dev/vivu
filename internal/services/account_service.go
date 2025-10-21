package services

import (
	"context"
	"fmt"
	"log"
	"time"
	"vivu/internal/models/db_models"
	"vivu/internal/models/request_models"
	"vivu/internal/models/response_models"
	"vivu/internal/repositories"
	mem "vivu/pkg/memcache"
	"vivu/pkg/utils"
)

type AccountServiceInterface interface {
	Login(request request_models.LoginRequest, ctx context.Context) (response_models.AccountLoginResponse, error)
	CreateAccount(request request_models.SignUpRequest) error
	ForgotPassword(email string) error
	VerifyAndConsumeResetToken(resetRequest request_models.ForgotPasswordRequest) (string, error)
	VerifyOtpToken(request request_models.RequestVerifyOtpToken) error
	IsUserHaveSubscription(accountID string) (bool, error)
	GetAllAccounts(ctx context.Context) ([]response_models.AccountResponse, error)
}

type AccountService struct {
	accountRepo  repositories.AccountRepository
	mailService  IMailService
	resetStore   mem.ResetTokenStore // inject this
	resetTTL     time.Duration       // e.g., 1 * time.Hour
	publicAppURL string
}

func (a *AccountService) GetAllAccounts(ctx context.Context) ([]response_models.AccountResponse, error) {
	accounts, err := a.accountRepo.GetAllAccounts(ctx)
	if err != nil {
		return nil, utils.ErrDatabaseError
	}

	var accountResponses []response_models.AccountResponse
	for _, account := range accounts {
		accountResponses = append(accountResponses, response_models.AccountResponse{
			ID:                   account.ID.String(),
			Name:                 account.Name,
			Email:                account.Email,
			Role:                 account.Role,
			SubscriptionSnapshot: account.SubscriptionSnapshot,
		})
	}

	return accountResponses, nil
}

func (a *AccountService) IsUserHaveSubscription(accountID string) (bool, error) {

	account, err := a.accountRepo.FindById(context.Background(), accountID)
	if err != nil {
		return false, utils.ErrDatabaseError
	}
	if account == nil {
		return false, utils.ErrAccountNotFound
	}

	fmt.Printf("Account details: %+v\n", account)

	// Check if the account has active subscriptions
	for _, sub := range account.Subs {
		if sub.Status == db_models.SubStatusActive {
			return true, nil
		}
	}

	return false, nil
}

func (a *AccountService) VerifyOtpToken(request request_models.RequestVerifyOtpToken) error {

	email, tokenValid := a.resetStore.Peek(request.Token)

	log.Printf("Verifying OTP token: %s for email: %s, valid: %v", request.Token, email, tokenValid)

	if tokenValid && email == request.Email {
		return nil
	}

	return utils.ErrInvalidToken
}

func NewAccountService(accountRepo repositories.AccountRepository, mailService IMailService, resetStore mem.ResetTokenStore) AccountServiceInterface {
	return &AccountService{
		accountRepo:  accountRepo,
		mailService:  mailService,
		resetStore:   resetStore,
		resetTTL:     time.Hour,
		publicAppURL: "https://vivu.com",
	}
}

func (a *AccountService) Login(request request_models.LoginRequest, ctx context.Context) (response_models.AccountLoginResponse, error) {

	startTime := time.Now()

	account, err := a.accountRepo.FindByEmail(ctx, request.Email)
	if err != nil {
		return response_models.AccountLoginResponse{}, utils.ErrDatabaseError
	}

	log.Printf("Login process took %s", time.Since(startTime))

	if account == nil {
		return response_models.AccountLoginResponse{}, utils.ErrAccountNotFound
	}

	err = utils.ComparePasswords(account.PasswordHash, request.Password)
	if err != nil {
		return response_models.AccountLoginResponse{}, utils.ErrInvalidCredentials
	}

	token, err := utils.CreateToken(account.ID, account.Role)
	if err != nil {
		return response_models.AccountLoginResponse{}, utils.ErrInvalidCredentials
	}

	isUserHavePremium, err := a.IsUserHaveSubscription(account.ID.String())

	if err != nil {
		return response_models.AccountLoginResponse{}, utils.ErrDatabaseError
	}

	return response_models.AccountLoginResponse{
		Token:             token,
		IsUserHavePremium: isUserHavePremium,
	}, nil
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

	go func() {
		err := a.mailService.SendMailToNotifyUser(newAccount.Email, "Welcome to Vivu", "Your account is ready. Explore features and let us know if you need help!", "click here", "https://vivu.com/login")
		if err != nil {
			log.Printf("Failed to send welcome email to %s: %v", newAccount.Email, err)
		} else {
			log.Printf("Welcome email sent to %s", newAccount.Email)
		}
	}()

	return nil
}

func (a *AccountService) ForgotPassword(email string) error {
	// 1) Check account
	account, err := a.accountRepo.FindByEmail(context.Background(), email)
	if err != nil {
		return utils.ErrDatabaseError
	}
	if account == nil {
		return utils.ErrAccountNotFound
	}

	// 2) Generate token
	resetToken, err := utils.GenerateOtpCode(6)
	if err != nil {
		return utils.ErrThirdService
	}

	// 3) Cache the token (token -> accountID) with TTL
	a.resetStore.Set(resetToken, account.Email, a.resetTTL)

	go func() {

		err := a.mailService.SendMailToResetPassword(
			account.Email,
			resetToken,
		)
		if err != nil {
			log.Printf("Failed to send password reset email to %s: %v", account.Email, err)
		}
	}()

	log.Printf("Password reset email sent to %s", account.Email)
	return nil
}

func (a *AccountService) VerifyAndConsumeResetToken(resetRequest request_models.ForgotPasswordRequest) (string, error) {
	accountID := a.resetStore.Consume(resetRequest.Token)
	if accountID == "" {
		return "", utils.ErrInvalidToken
	}

	// Update the account with the new password
	hashedPassword, err := utils.HashPassword(resetRequest.NewPassword)
	if err != nil {
		return "", utils.ErrDatabaseError
	}

	err = a.accountRepo.UpdatePasswordByEmail(context.Background(), accountID, hashedPassword)
	if err != nil {
		return "", utils.ErrDatabaseError
	}

	return accountID, nil
}
