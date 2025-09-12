package db_fx

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/infra"
)

var Module = fx.Provide(
	provideDB)

func provideDB() *gorm.DB {
	return infra.InitPostgresql()
}
