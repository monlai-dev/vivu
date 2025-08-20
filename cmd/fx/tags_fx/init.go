package tagsfx

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/repositories"
	"vivu/internal/services"
)

var Module = fx.Provide(
	provideTagsRepo, provideTagsService)

func provideTagsRepo(db *gorm.DB) repositories.TagRepositoryInterface {
	return repositories.NewTagRepository(db)
}

func provideTagsService(tagRepo repositories.TagRepositoryInterface) services.TagServiceInterface {
	return services.NewTagService(tagRepo)
}
