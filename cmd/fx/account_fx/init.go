package account_fx

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/repositories"
	"vivu/internal/services"
	mem "vivu/pkg/memcache"
)

var Module = fx.Provide(
	provideAccountService, provideAccountRepo)

func provideAccountRepo(db *gorm.DB) repositories.AccountRepository {
	return repositories.NewAccountRepository(db)
}

func provideAccountService(accountRepo repositories.AccountRepository, mailService services.IMailService, memcache mem.ResetTokenStore) services.AccountServiceInterface {
	return services.NewAccountService(accountRepo, mailService, memcache)
}
