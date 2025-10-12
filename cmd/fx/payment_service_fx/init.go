package payment_service_fx

import (
	"go.uber.org/fx"
	"gorm.io/gorm"
	"log"
	"os"
	"vivu/internal/api/controllers"
	"vivu/internal/services"
)

var payOsCgf = services.PayOSConfig{
	ClientID:     os.Getenv("PAYOS_CLIENT_ID"),
	ApiKey:       os.Getenv("PAYOS_API_KEY"),
	ChecksumKey:  os.Getenv("PAYOS_CHECKSUM_KEY"),
	ProviderName: "payos",
}

var Module = fx.Provide(
	providePaymentService, provicePaymentController,
)

func providePaymentService(db *gorm.DB) services.PaymentService {
	instance, err := services.NewPaymentService(db, payOsCgf)
	if err != nil {
		log.Printf("Error initializing PaymentService: %v", err)
	}

	return instance
}

func provicePaymentController(paymentService services.PaymentService) *controllers.PaymentController {
	return controllers.NewPaymentController(paymentService)
}
