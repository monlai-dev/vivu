package feedback_fx

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"vivu/internal/api/controllers"
	"vivu/internal/repositories"
	"vivu/internal/services"
)

var Module = fx.Provide(
	provideFeedbackRepo, provideFeedbackService, provideFeedbackController,
)

func provideFeedbackRepo(db *gorm.DB) repositories.FeedbackRepositoryInterface {
	return repositories.NewFeedbackRepository(db)
}

func provideFeedbackService(feedbackRepo repositories.FeedbackRepositoryInterface) services.FeedbackServiceInterface {
	return services.NewFeedbackService(feedbackRepo)
}

func provideFeedbackController(feedbackService services.FeedbackServiceInterface) *controllers.FeedbackController {
	return controllers.NewFeedbackController(feedbackService)
}
