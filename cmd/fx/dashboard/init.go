package dashboard

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/api/controllers"
	"vivu/internal/repositories"
	"vivu/internal/services"
)

var Module = fx.Provide(
	provideDashboardRepo, provideDashboardService, provideDashboardController,
)

func provideDashboardRepo(db *gorm.DB) repositories.DashboardRepository {
	return repositories.NewDashboardRepository(db)
}

func provideDashboardService(dashboardRepo repositories.DashboardRepository) services.DashboardService {
	return services.NewDashboardService(dashboardRepo)
}

func provideDashboardController(dashboardService services.DashboardService) *controllers.DashboardController {
	return controllers.NewDashboardController(dashboardService)
}
