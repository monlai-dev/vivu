package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/fx"
	"log"
	"os"
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
	"vivu/pkg/middleware"
)

func init() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env file, %v", err)
	}
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

		fx.Invoke(StartServer),
		fx.Provide(ProvideRouter),
		fx.Invoke(SetupSwagger),
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
	provinceController *controllers.ProvincesController) *gin.Engine {

	r := gin.Default()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.TraceIDMiddleware())

	RegisterRoutes(r, poisController, tagsController, promptController, provinceController)

	return r
}

func SetupSwagger(router *gin.Engine) {

	docs.SwaggerInfo.Title = "Vivu API"
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.BasePath = "/"
	docs.SwaggerInfo.Schemes = []string{"http"} // local

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
}

func RegisterRoutes(r *gin.Engine,
	poisController *controllers.POIsController,
	tagsController *controllers.TagController,
	promptController *controllers.PromptController,
	provinceController *controllers.ProvincesController) {

	poisgroup := r.Group("/pois")
	poisgroup.GET("/provinces/:provinceId", poisController.GetPoisByProvince)
	poisgroup.GET("/pois-details/:id", poisController.GetPoiById)

	tagsGroup := r.Group("/tags")
	tagsGroup.GET("/list-all", tagsController.ListAllTagsHandler)

	promptGroup := r.Group("/prompt")
	promptGroup.POST("/generate-plan", promptController.CreatePromptHandler)
	promptGroup.POST("/quiz/start", promptController.StartQuizHandler)
	promptGroup.POST("/quiz/answer", promptController.AnswerQuizHandler)
	promptGroup.POST("/quiz/plan-only", promptController.PlanOnlyHandler)

	provinceGroup := r.Group("/provinces")
	provinceGroup.GET("/list-all", provinceController.GetAllProvinces)
}
