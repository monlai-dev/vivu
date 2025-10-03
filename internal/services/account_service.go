package services

import (
	"context"
	"fmt"
	"log"
	"time"
	"vivu/internal/models/db_models"
	"vivu/internal/models/request_models"
	"vivu/internal/repositories"
	mem "vivu/pkg/memcache"
	"vivu/pkg/utils"
)

type AccountServiceInterface interface {
	Login(request request_models.LoginRequest, ctx context.Context) (string, error)
	CreateAccount(request request_models.SignUpRequest) error
	ForgotPassword(email string) error
	VerifyAndConsumeResetToken(resetRequest request_models.ForgotPasswordRequest) (string, error)
}

type AccountService struct {
	accountRepo  repositories.AccountRepository
	mailService  IMailService
	resetStore   mem.ResetTokenStore // inject this
	resetTTL     time.Duration       // e.g., 1 * time.Hour
	publicAppURL string
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
	resetToken, err := utils.GenerateSecureToken(32)
	if err != nil {
		return utils.ErrThirdService
	}

	// 3) Cache the token (token -> accountID) with TTL
	a.resetStore.Set(resetToken, account.ID.String(), a.resetTTL)

	// 4) Send email (link carries the token)
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", a.publicAppURL, resetToken)
	go func() {
		err := a.mailService.SendMailToNotifyUser(
			account.Email,
			"Password Reset Request",
			"We received a request to reset your password. Click the link below to reset it.",
			"Reset Password",
			resetURL,
		)
		if err != nil {
			log.Printf("Failed to send password reset email to %s: %v", account.Email, err)
		}
	}()

	log.Printf("Password reset email sent to %s", account.Email)
	return nil
}

// Verify + consume token (single-use) when user submits the new password form.
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

	err = a.accountRepo.UpdatePasswordByID(context.Background(), accountID, hashedPassword)
	if err != nil {
		return "", utils.ErrDatabaseError
	}

	return accountID, nil
}
