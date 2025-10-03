package mail_fx

import (
	"go.uber.org/fx"
	"log"
	"os"
	"vivu/internal/services"
)

var Module = fx.Provide(provideMailService)

func provideMailService() services.IMailService {

	cfg := services.SMTPConfig{
		Host:       "smtp.gmail.com",
		Port:       587, // 587 for STARTTLS; use 465 with UseSSL=true for SMTPS
		Username:   "vivu.fpt.vn@gmail.com",
		Password:   os.Getenv("SMTP_PASSWORD"), // use app password if 2FA is enabled
		From:       "vivu.fpt.vn@gmail.com",
		FromName:   "Vivu",
		UseSSL:     false, // true if using port 465
		RequireTLS: true,

		AppName:    "Vivu",
		AppBaseURL: "https://yourapp.com",
	}

	mailService, err := services.NewSMTPMailService(cfg)

	log.Printf("SMTP_PASSWORD: %s", os.Getenv("SMTP_PASSWORD"))

	if err != nil {
		log.Printf("Failed to initialize SMTP mail service: %v", err)
	}

	return mailService
}
