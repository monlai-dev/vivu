package poi_embedded_fx

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/repositories"
)

var Module = fx.Provide(
	provideEmbededRepo)

func provideEmbededRepo(db *gorm.DB) repositories.IPoiEmbededRepository {
	return repositories.NewPoiEmbededRepository(db)
}
