package distance_matrix_fx

import (
	"go.uber.org/fx"
	"vivu/internal/services"
)

var Module = fx.Provide(provideMatrixRepo)

func provideMatrixRepo() services.DistanceMatrixService {
	return services.NewMapboxMatrixClient(services.NewInMemoryPairCache())
}
