package controllers_fx

import (
	"go.uber.org/fx"
	"vivu/internal/api/controllers"
)

var Module = fx.Options(
	fx.Provide(controllers.NewPOIsController),
	fx.Provide(controllers.NewTagController))
