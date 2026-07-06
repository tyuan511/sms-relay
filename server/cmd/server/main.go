package main

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sms-relay/server/internal/applog"
	"github.com/sms-relay/server/internal/config"
	"github.com/sms-relay/server/internal/crypto"
	"github.com/sms-relay/server/internal/db"
	"github.com/sms-relay/server/internal/handlers"
	"github.com/sms-relay/server/internal/middleware"
	"github.com/sms-relay/server/internal/migrate"
	"github.com/sms-relay/server/internal/services"
	"github.com/sms-relay/server/internal/sse"
	"github.com/sms-relay/server/internal/telegram"
)

func main() {
	applog.Init()
	cfg := config.Load()
	if cfg.DatabaseEncryptionKey == "" {
		applog.Error("startup", "DATABASE_ENCRYPTION_KEY is required")
		log.Fatal("DATABASE_ENCRYPTION_KEY is required")
	}
	if cfg.PasswordPepper == "" {
		applog.Error("startup", "PASSWORD_PEPPER is required")
		log.Fatal("PASSWORD_PEPPER is required")
	}

	clientIPResolver, err := middleware.NewClientIPResolver(cfg.TrustedProxyCIDRs)
	if err != nil {
		applog.Error("startup", "invalid TRUSTED_PROXY_CIDRS", "error", err)
		log.Fatalf("TRUSTED_PROXY_CIDRS: %v", err)
	}

	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0o755); err != nil {
		applog.Error("startup", "create data dir failed", "error", err)
		log.Fatalf("create data dir: %v", err)
	}

	sqlDB, err := sql.Open("sqlite3", cfg.DatabasePath+"?_foreign_keys=on&_journal_mode=WAL")
	if err != nil {
		applog.Error("startup", "open database failed", "error", err)
		log.Fatalf("open db: %v", err)
	}
	defer sqlDB.Close()

	if err := migrate.Up(sqlDB, cfg.MigrationsDir); err != nil {
		applog.Error("startup", "database migration failed", "error", err)
		log.Fatalf("migrate: %v", err)
	}

	encryptor, err := crypto.NewFieldEncryptor(cfg.DatabaseEncryptionKey)
	if err != nil {
		applog.Error("startup", "encryptor init failed", "error", err)
		log.Fatalf("encryptor: %v", err)
	}

	queries := db.New(sqlDB)
	hub := sse.NewHub()
	msgService := services.NewMessageService(queries, encryptor, hub)
	tgClient := telegram.NewClient()
	tgLinker := telegram.NewLinker(tgClient)
	h := handlers.New(handlers.Config{
		JWTSecret:      cfg.JWTSecret,
		PasswordPepper: cfg.PasswordPepper,
	}, queries, sqlDB, msgService, hub, encryptor, tgLinker, tgClient)

	authRateLimiter := middleware.NewAuthRateLimiter(clientIPResolver)
	authRateLimit := authRateLimiter.AuthRateLimit()

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"detail": err.Error()})
		},
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(middleware.HTTPErrorLogger())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigin,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PATCH,DELETE,OPTIONS",
		AllowCredentials: true,
	}))

	api := app.Group("/api/v1")
	api.Get("/health", h.Health)
	api.Post("/auth/register", h.Register)
	api.Post("/auth/login", authRateLimit, h.Login)
	api.Post("/auth/device", authRateLimit, h.AuthDevice)

	userAuth := middleware.UserJWTAuth(cfg.JWTSecret)
	deviceAuth := middleware.DeviceJWTAuth(cfg.JWTSecret)

	api.Post("/messages/inbound", deviceAuth, h.InboundMessage)
	api.Post("/devices/heartbeat", deviceAuth, h.DeviceHeartbeat)

	api.Get("/messages/stream", userAuth, h.StreamEvents)
	api.Get("/messages", userAuth, h.ListMessages)
	api.Get("/destinations", userAuth, h.ListDestinations)
	api.Post("/destinations", userAuth, h.CreateDestination)
	api.Post("/destinations/telegram/link", userAuth, h.StartTelegramLink)
	api.Get("/destinations/telegram/link/:id", userAuth, h.GetTelegramLinkStatus)
	api.Get("/destinations/:id/avatar", userAuth, h.DestinationAvatar)
	api.Patch("/destinations/:id", userAuth, h.UpdateDestination)
	api.Delete("/destinations/:id", userAuth, h.DeleteDestination)
	api.Get("/devices", userAuth, h.ListDevices)

	applog.Info("startup", "server listening", "addr", cfg.Addr())
	log.Fatal(app.Listen(cfg.Addr()))
}
