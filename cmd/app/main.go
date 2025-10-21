package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/fx"
	"log"
	"os"
	"path/filepath"
	"vivu/cmd/fx/account_fx"
	"vivu/cmd/fx/controllers_fx"
	"vivu/cmd/fx/dashboard"
	"vivu/cmd/fx/db_fx"
	"vivu/cmd/fx/distance_matrix_fx"
	"vivu/cmd/fx/feedback_fx"
	"vivu/cmd/fx/journey_fx"
	"vivu/cmd/fx/mail_fx"
	"vivu/cmd/fx/memcache_fx"
	"vivu/cmd/fx/payment_service_fx"
	"vivu/cmd/fx/poi_embedded_fx"
	"vivu/cmd/fx/pois_fx"
	"vivu/cmd/fx/prompt_fx"
	"vivu/cmd/fx/province_fx"
	"vivu/cmd/fx/tags_fx"
	docs "vivu/docs"
	"vivu/internal/api/controllers"
	"vivu/internal/infra"
	"vivu/internal/models/db_models"

	"vivu/pkg/middleware"
)

func init() {
	if err := loadDotEnv(); err != nil {
		log.Printf("No .env found (will use OS env...): %v", err)
	}
}

func loadDotEnv() error {
	// Try a few common relative locations first
	candidates := []string{".env", "../.env", "../../.env", "../../../.env"}

	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return godotenv.Load(p)
		}
	}

	// Fallback: walk up until we hit go.mod, then load .env there (project root)
	wd, _ := os.Getwd()
	dir := wd
	for i := 0; i < 10; i++ { // safety bound
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			envPath := filepath.Join(dir, ".env")
			if _, err := os.Stat(envPath); err == nil {
				return godotenv.Load(envPath)
			}
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return fmt.Errorf(".env not found from %q upward", wd)
}

// @title Vivu Travel API
// @version 1.0
// @description This is the API documentation for Vivu Travel Platform
// @host api.vivu-travel.site
// @BasePath /api
// @schemes https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token
func main() {
	app := fx.New(
		fx.Invoke(infra.InitPostgresql),
		db_fx.Module,
		pois_fx.Module,
		tags_fx.Module,
		controllers_fx.Module,
		prompt_fx.Module,
		poi_embedded_fx.Module,
		province_fx.Module,
		distance_matrix_fx.Module,
		account_fx.Module,
		journey_fx.Module,
		mail_fx.Module,
		memcache_fx.Module,
		payment_service_fx.Module,
		dashboard.Module,
		feedback_fx.Module,

		fx.Invoke(StartServer),
		fx.Provide(ProvideRouter),
		fx.Invoke(SetupSwagger),
		fx.Invoke(MigrateDB),
	)

	//errExcel := services.ExportPOIsToExcel(infra.GetPostgresql(), "exported_pois.xlsx")
	//if errExcel != nil {
	//	log.Println("Failed to export POIs to Excel:", errExcel)
	//}
	//services.NewOSClient()
	app.Run()
}

func StartServer(lc fx.Lifecycle, engine *gin.Engine) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				log.Println("Starting HTTP server at ${PORT}")
				if err := engine.Run(":" + os.Getenv("PORT")); err != nil {
					log.Fatalf("Failed to start server: %v", err)
				}

			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Println("Stopping HTTP server")
			infra.ClosePostgresql(infra.GetPostgresql())
			return nil
		},
	})
}

func ProvideRouter(
	poisController *controllers.POIsController,
	tagsController *controllers.TagController,
	promptController *controllers.PromptController,
	provinceController *controllers.ProvincesController,
	accountController *controllers.AccountController,
	journeyController *controllers.JourneyController,
	paymentController *controllers.PaymentController,
	dashboardController *controllers.DashboardController,
	feedbackController *controllers.FeedbackController) *gin.Engine {

	r := gin.Default()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.TraceIDMiddleware())

	RegisterRoutes(r, poisController, tagsController, promptController, provinceController, accountController, journeyController, paymentController, dashboardController, feedbackController)

	return r
}

func SetupSwagger(router *gin.Engine) {
	// read environment
	env := "prod" // "local" | "dev" | "prod"
	host := "api.vivu-travel.site"
	if host == "" {
		host = "api.vivu-travel.site"
	}

	// Defaults from annotations, then override per env
	// Annotations in main.go can stay generic.
	switch env {
	case "prod", "production":
		docs.SwaggerInfo.Host = host    // api.vivu-travel.site
		docs.SwaggerInfo.BasePath = "/" // matches your RegisterRoutes (no /api prefix)
		docs.SwaggerInfo.Schemes = []string{"https"}
	default:
		// local/dev: run swagger on http://localhost:<port>/swagger
		docs.SwaggerInfo.Host = "" // empty -> same origin
		docs.SwaggerInfo.BasePath = "/"
		docs.SwaggerInfo.Schemes = []string{"http"}
	}

	sg := router.Group("/swagger")
	sg.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'self' 'unsafe-inline' 'unsafe-eval'; img-src 'self' data:")
		c.Next()
	})

	sg.GET("/*any", ginSwagger.WrapHandler(
		swaggerFiles.Handler,
		ginSwagger.URL("/swagger/doc.json"),
		ginSwagger.PersistAuthorization(true),
	))
}

func MigrateDB() {
	db := infra.GetPostgresql()
	infra.MigratePostgresql(db,
		db_models.POIDetail{},
		db_models.POI{},
		db_models.Account{},
		db_models.Journey{},
		db_models.JourneyDay{},
		db_models.JourneyActivity{},
		db_models.Subscription{},
		db_models.Transaction{},
		db_models.Plan{},
		db_models.Feedback{})

}

func RegisterRoutes(r *gin.Engine,
	poisController *controllers.POIsController,
	tagsController *controllers.TagController,
	promptController *controllers.PromptController,
	provinceController *controllers.ProvincesController,
	accountController *controllers.AccountController,
	journeyController *controllers.JourneyController,
	paymentController *controllers.PaymentController,
	dashboardController *controllers.DashboardController,
	feedbackController *controllers.FeedbackController) {

	accountGroup := r.Group("/accounts")
	accountGroup.POST("/register", accountController.Register)
	accountGroup.POST("/login", accountController.Login)
	accountGroup.POST("/forgot-password", accountController.ForgotPassword)
	accountGroup.POST("/verify-otp", accountController.VerifyOtpToken)
	accountGroup.POST("/reset-password", accountController.ResetPasswordWithOtp)

	poisgroup := r.Group("/pois")
	poisgroup.GET("/provinces/:provinceId", poisController.GetPoisByProvince)
	poisgroup.GET("/pois-details/:id", poisController.GetPoiById)
	poisgroup.POST("/create-poi", poisController.CreatePoi)
	poisgroup.DELETE("/delete-poi/", poisController.DeletePoi)
	poisgroup.PUT("/update-poi/", poisController.UpdatePoi)
	poisgroup.GET("/list-pois", poisController.ListPois)

	tagsGroup := r.Group("/tags")
	tagsGroup.GET("/list-all", tagsController.ListAllTagsHandler)

	promptGroup := r.Group("/prompt", middleware.JWTAuthMiddleware())
	promptGroup.POST("/generate-plan", promptController.CreatePromptHandler)
	promptGroup.POST("/quiz/start", promptController.StartQuizHandler)
	promptGroup.POST("/quiz/answer", promptController.AnswerQuizHandler)
	promptGroup.POST("/quiz/plan-only", promptController.PlanOnlyHandler)

	provinceGroup := r.Group("/provinces", middleware.JWTAuthMiddleware())
	provinceGroup.GET("/list-all", provinceController.GetAllProvinces)
	provinceGroup.GET("/find-by-name/:province_name", provinceController.FindProvincesByName)
	provinceGroup.POST("/create", provinceController.CreateProvinceHandler)

	journeyGroup := r.Group("/journeys", middleware.JWTAuthMiddleware())
	journeyGroup.GET("/get-journey-by-userid", journeyController.GetJourneyByUserId)
	journeyGroup.GET("/get-details-info-of-journey-by-id/:journeyId", journeyController.GetDetailsInfoOfJourneyById)
	journeyGroup.POST("/add-poi-to-journey", journeyController.AddPoiToJourney)
	journeyGroup.POST("/remove-poi-from-journey", journeyController.RemovePoiFromJourney)
	journeyGroup.POST("/add-day-to-journey", journeyController.AddDayToJourney)

	paymentGroup := r.Group("/payments")
	paymentGroup.POST("/create-checkout", middleware.JWTAuthMiddleware(), paymentController.CreateCheckoutRequest)
	paymentGroup.POST("/webhook", paymentController.HandleWebhook)
	paymentGroup.GET("/plans", paymentController.GetListOfAvailablePlans)
	paymentGroup.GET("/subscription-details", middleware.JWTAuthMiddleware(), paymentController.GetSubscriptionDetails)

	dashboardGroup := r.Group("/dashboard", middleware.JWTAuthMiddleware())
	dashboardGroup.GET("/stats", dashboardController.GetDashboard)

	feedbackGroup := r.Group("/feedback")
	feedbackGroup.POST("/add", feedbackController.AddFeedback)
	feedbackGroup.GET("/list", feedbackController.ListFeedback)

}
