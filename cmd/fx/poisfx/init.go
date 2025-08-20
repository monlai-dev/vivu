package poisfx

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/repositories"
	"vivu/internal/services"
)

var Module = fx.Provide(
	providePoisRepo, providePoisService)

func providePoisRepo(db *gorm.DB) repositories.POIRepository {
	return repositories.NewPOIRepository(db)
}

func providePoisService(poiRepo repositories.POIRepository) services.POIServiceInterface {
	return services.NewPOIService(poiRepo)
}
