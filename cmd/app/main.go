package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"log"
	"os"
	"vivu/cmd/fx/dbfx"
	"vivu/cmd/fx/poisfx"
	"vivu/cmd/fx/tagsfx"
	"vivu/internal/api/controllers"
	"vivu/internal/infra"
	"vivu/pkg/middleware"
)

func main() {
	app := fx.New(
		fx.Invoke(infra.InitPostgresql),
		dbfx.Module,
		poisfx.Module,
		tagsfx.Module,

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

	RegisterRoutes(r, poisController, tagsController)

	return r
}

func RegisterRoutes(r *gin.Engine,
	poisController *controllers.POIsController,
	tagsController *controllers.TagController) {

	poisgroup := r.Group("/pois")
	poisgroup.GET("/:provinceId", poisController.GetPoisByProvince)
	poisgroup.GET("/:id", poisController.GetPoiById)

	tagsGroup := r.Group("/tags")
	tagsGroup.GET("/tags", tagsController.ListAllTagsHandler)

}
