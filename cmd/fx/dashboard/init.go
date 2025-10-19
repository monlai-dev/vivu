package dashboard

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/repositories"
	"vivu/internal/services"
)

var Module = fx.Provide(
	provideDashboardRepo, provideDashboardService,
)

func provideDashboardRepo(db *gorm.DB) repositories.DashboardRepository {
	return repositories.NewDashboardRepository(db)
}

func provideDashboardService(dashboardRepo repositories.DashboardRepository) services.DashboardService {
	return services.NewDashboardService(dashboardRepo)
}
