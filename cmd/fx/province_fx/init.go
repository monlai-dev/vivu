package province_fx

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/repositories"
	"vivu/internal/services"
)

var Module = fx.Provide(
	NewProvinceService, NewProvinceRepo)

func NewProvinceService(repo repositories.ProvinceRepository) services.ProvinceServiceInterface {
	return services.NewProvinceService(repo)
}

func NewProvinceRepo(db *gorm.DB) repositories.ProvinceRepository {
	return repositories.NewProvinceRepository(db)
}
