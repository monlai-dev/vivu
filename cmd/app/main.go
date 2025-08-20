package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.uber.org/fx"
	"log"
	"os"
	"vivu/cmd/fx/controllers_fx"
	"vivu/cmd/fx/db_fx"
	"vivu/cmd/fx/pois_fx"
	"vivu/cmd/fx/tags_fx"
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

		fx.Invoke(StartServer),
		fx.Provide(ProvideRouter),
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
			return nil
		},
	})
}

func ProvideRouter(
	poisController *controllers.POIsController,
	tagsController *controllers.TagController) *gin.Engine {

	r := gin.Default()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware())
	r.Use(middleware.TraceIDMiddleware())

	RegisterRoutes(r, poisController, tagsController)

	return r
}

func RegisterRoutes(r *gin.Engine,
	poisController *controllers.POIsController,
	tagsController *controllers.TagController) {

	poisgroup := r.Group("/pois")
	poisgroup.GET("/provinces/:provinceId", poisController.GetPoisByProvince)
	poisgroup.GET("/pois/:id", poisController.GetPoiById)

	tagsGroup := r.Group("/tags")
	tagsGroup.GET("/list-all", tagsController.ListAllTagsHandler)

}
