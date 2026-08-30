package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	swaggo "github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	staticMiddleware "github.com/gofiber/fiber/v3/middleware/static"
	"github.com/toyama-pj/simple-kvs-registory/handlers"
	"github.com/toyama-pj/simple-kvs-registory/lib"
	databaseDialect "github.com/toyama-pj/simple-kvs-registory/lib/db"
	"github.com/toyama-pj/simple-kvs-registory/semtech"
	"gorm.io/gorm"

	_ "github.com/toyama-pj/simple-kvs-registory/docs"
)

// @title			Simple KVS Registry API
// @version		0.1.0
// @BasePath		/api/v1
// @securityDefinitions.apikey	BearerAuth
// @in	header
// @name	Authorization
// @description	"Bearer " に続けて User Bearer Token または WriteAccessToken を指定します。WriteAccessToken は対象namespaceへのPOST /dataだけに使用できます。
func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	config, err := lib.ReadConfig(".env", true)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	dialect := databaseDialect.GetDatabaseDialector(config.DATABASE_PROVIDER, config.DATABASE_DSN)
	database, err := gorm.Open(dialect, &gorm.Config{})
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	if err := lib.MigrateSchema(database); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	log.Println("Database migration completed successfully.")

	if config.DEVELOPMENT {
		controller := lib.NewController(database, config)
		log.Println("Create Sample User: i@example.com")
		if _, err := controller.GetUserByMailAddress("i@example.com"); errors.Is(err, gorm.ErrRecordNotFound) {
			if err := controller.CreateUser("Test User", "i@example.com"); err != nil {
				return fmt.Errorf("create sample user: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("look up sample user: %w", err)
		}
	}

	app := buildApp(database, config)
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	webDone := make(chan error, 1)
	webAddress := fmt.Sprintf("%s:%d", config.WEB_HOSTNAME, config.WEB_PORT)
	go func() { webDone <- app.Listen(webAddress) }()

	var udpDone chan error
	if config.SEMTECH_UDP_ENABLED {
		udpDone = make(chan error, 1)
		udpServer := semtech.NewServer(database, config)
		go func() { udpDone <- udpServer.ListenAndServe(ctx) }()
	}

	var firstErr error
	webFinished := false
	udpFinished := false
	select {
	case <-ctx.Done():
		log.Println("Received shutdown signal")
	case err := <-webDone:
		webFinished = true
		if err != nil {
			firstErr = fmt.Errorf("web server: %w", err)
		}
	case err := <-udpDone:
		udpFinished = true
		if err != nil {
			firstErr = fmt.Errorf("Semtech UDP server: %w", err)
		}
	}

	stopSignals()
	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil && !errors.Is(err, fiber.ErrNotRunning) && firstErr == nil {
		firstErr = fmt.Errorf("shutdown web server: %w", err)
	}
	if !webFinished {
		if err := <-webDone; err != nil && firstErr == nil {
			firstErr = fmt.Errorf("web server: %w", err)
		}
	}
	if udpDone != nil && !udpFinished {
		if err := <-udpDone; err != nil && firstErr == nil {
			firstErr = fmt.Errorf("Semtech UDP server: %w", err)
		}
	}

	if config.DATABASE_PROVIDER == "duckdb" {
		log.Println("Flushing WAL to database...")
		if err := database.Exec("CHECKPOINT;").Error; err != nil && firstErr == nil {
			firstErr = fmt.Errorf("checkpoint DuckDB: %w", err)
		}
	}
	sqlDB, err := database.DB()
	if err != nil {
		if firstErr == nil {
			firstErr = fmt.Errorf("get database connection: %w", err)
		}
	} else if err := sqlDB.Close(); err != nil && firstErr == nil {
		firstErr = fmt.Errorf("close database: %w", err)
	}
	return firstErr
}

func buildApp(database *gorm.DB, config lib.Config) *fiber.App {
	controller := handlers.NewController(database, config)
	app := fiber.New(fiber.Config{BodyLimit: 1024 * 1024, ErrorHandler: handlers.GlobalErrorHandler})

	docsBasicMiddleware := basicauth.New(basicauth.Config{Users: map[string]string{"docs": "{SHA256}" + config.SWAGGER_BASIC}})
	app.Get("/docs/*", docsBasicMiddleware, swaggo.HandlerDefault)
	webHeaders := func(c fiber.Ctx) error {
		c.Set(fiber.HeaderContentSecurityPolicy, "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
		c.Set(fiber.HeaderXContentTypeOptions, "nosniff")
		c.Set(fiber.HeaderReferrerPolicy, "no-referrer")
		return c.Next()
	}
	app.Get("/", webHeaders, staticMiddleware.New("./web/index.html"))
	app.Get("/assets/*", webHeaders, staticMiddleware.New("./web/assets"))
	app.Use(controller.AccessLogMiddlewareHandler)

	v1 := app.Group("/api/v1")
	v1.Get("/", func(c fiber.Ctx) error { return c.SendString("ok!") })
	v1.Route("/auth/", controller.AuthHandlersSetup)
	v1.Route("/cfg/", controller.CfgHandlersSetup)
	v1.Route("/data/", controller.DataHandlersSetup)
	v1.Route("/organizations", controller.OrganizationHandlersSetup)
	v1.Route("/namespaces", controller.NamespaceHandlersSetup)
	v1.Route("/devices", controller.DeviceHandlersSetup)

	app.Use(handlers.NotFoundMiddlewareHandler)
	return app
}
