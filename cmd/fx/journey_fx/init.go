package journey_fx

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/repositories"
	"vivu/internal/services"
)

var Module = fx.Provide(provideJourneyRepo, provideJourneyService)

func provideJourneyRepo(db *gorm.DB) repositories.JourneyRepository {
	return repositories.NewJourneyRepository(db)
}

func provideJourneyService(journeyRepo repositories.JourneyRepository) services.JourneyServiceInterface {

	return services.NewJourneyService(journeyRepo)
}
