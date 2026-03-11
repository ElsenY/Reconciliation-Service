package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"github.com/reconciliation-service/internal/handler"
	"github.com/reconciliation-service/internal/helper/config"
	"github.com/reconciliation-service/internal/repository"
	"github.com/reconciliation-service/internal/service"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	port := config.GetEnv("APP_PORT", "3000")
	defaultSystem := config.GetEnv("DEFAULT_SYSTEM_CSV", "testdata/system_transactions.csv")
	defaultBanks := config.GetEnv("DEFAULT_BANK_CSV", "testdata/bank_bca.csv,testdata/bank_bni.csv")

	var bankPaths []string
	if defaultBanks != "" {
		for _, p := range strings.Split(defaultBanks, ",") {
			if trimmed := strings.TrimSpace(p); trimmed != "" {
				bankPaths = append(bankPaths, trimmed)
			}
		}
	}

	repo := repository.NewCSVTransactionRepository()
	svc := service.NewReconciliationService(repo)
	h := handler.NewReconciliationHandler(svc, defaultSystem, bankPaths)

	app := fiber.New(fiber.Config{
		BodyLimit: 50 * 1024 * 1024, // 50 MB
	})

	app.Use(logger.New())
	app.Use(recover.New())

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	h.RegisterRoutes(app)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("Reconciliation Service starting on %s", addr)
	log.Fatal(app.Listen(addr))
}
