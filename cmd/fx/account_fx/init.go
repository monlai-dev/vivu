package account_fx

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/repositories"
	"vivu/internal/services"
)

var Module = fx.Provide(
	provideAccountService, provideAccountRepo)

func provideAccountRepo(db *gorm.DB) repositories.AccountRepository {
	return repositories.NewAccountRepository(db)
}

func provideAccountService(accountRepo repositories.AccountRepository) services.AccountService {

	return services.NewAccountService(accountRepo)
}
