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
	"vivu/cmd/fx/db_fx"
	"vivu/cmd/fx/distance_matrix_fx"
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

		fx.Invoke(StartServer),
		fx.Provide(ProvideRouter),
		fx.Invoke(SetupSwagger),
		fx.Invoke(MigrateDB),
	)

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
	accountController *controllers.AccountController) *gin.Engine {

	r := gin.Default()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.TraceIDMiddleware())

	RegisterRoutes(r, poisController, tagsController, promptController, provinceController, accountController)

	return r
}

func SetupSwagger(router *gin.Engine) {

	docs.SwaggerInfo.Title = "Vivu API"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Schemes = []string{"http"} // local

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

func MigrateDB() {
	db := infra.GetPostgresql()
	infra.MigratePostgresql(db, db_models.POIDetail{}, db_models.POI{}, db_models.Account{})
}

func RegisterRoutes(r *gin.Engine,
	poisController *controllers.POIsController,
	tagsController *controllers.TagController,
	promptController *controllers.PromptController,
	provinceController *controllers.ProvincesController,
	accountController *controllers.AccountController) {

	accountGroup := r.Group("/accounts")
	accountGroup.POST("/register", accountController.Register)
	accountGroup.POST("/login", accountController.Login)

	poisgroup := r.Group("/pois")
	poisgroup.GET("/provinces/:provinceId", poisController.GetPoisByProvince)
	poisgroup.GET("/pois-details/:id", poisController.GetPoiById)
	poisgroup.POST("/create-poi", poisController.CreatePoi)

	tagsGroup := r.Group("/tags")
	tagsGroup.GET("/list-all", tagsController.ListAllTagsHandler)

	promptGroup := r.Group("/prompt", middleware.JWTAuthMiddleware())
	promptGroup.POST("/generate-plan", promptController.CreatePromptHandler)
	promptGroup.POST("/quiz/start", promptController.StartQuizHandler)
	promptGroup.POST("/quiz/answer", promptController.AnswerQuizHandler)
	promptGroup.POST("/quiz/plan-only", promptController.PlanOnlyHandler)

	provinceGroup := r.Group("/provinces")
	provinceGroup.GET("/list-all", provinceController.GetAllProvinces, middleware.RoleMiddleware("admin"))
}
