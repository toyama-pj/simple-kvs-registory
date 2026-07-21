package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	swaggo "github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/basicauth"
	"github.com/toyama-pj/simple-kvs-registory/handlers"
	"github.com/toyama-pj/simple-kvs-registory/lib"
	"github.com/toyama-pj/simple-kvs-registory/lib/db"
	"gorm.io/gorm"

	_ "github.com/toyama-pj/simple-kvs-registory/docs"
)

// @title			Simple KVS Registry API
// @version			0.0.1
// @BasePath		/api/v1
// @securityDefinitions.apikey	BearerAuth
// @in	header
// @name	Authorization
// @description	"Bearer " に続けて User Bearer Token または WriteAccessToken を指定します。WriteAccessToken は対象namespaceへのPOST /dataだけに使用できます。
func main() {
	// Configuration
	config, err := lib.ReadConfig(".env", true)
	if err != nil {
		panic(fmt.Sprintf("failed to read config: %s", err))
	}

	// DB Connection
	dialect := db.GetDatabaseDialector(config.DATABASE_PROVIDER, config.DATABASE_DSN)

	// DB Migration
	db, err := gorm.Open(dialect, &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("failed to connect to DB: %s", err))
	}

	err = lib.MigrateSchema(db)
	if err != nil {
		panic(fmt.Sprintf("failed to migrate db: %s", err))
	}
	log.Println("Database migration completed successfully.")

	// Database Instance
	con := handlers.NewController(db, config)

	if config.DEVELOPMENT == true {
		cont := lib.NewController(db, config)
		log.Println("Create Sample User: i@example.com")
		_, err := cont.GetUserByMailAddress("i@example.com")
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err := cont.CreateUser("Test User", "i@example.com")
			if err != nil {
				panic(fmt.Sprintf("failed to create sample user: %s", err))
			}
		} else if err != nil {
			panic(fmt.Sprintf("failed to create sample user: %s", err))
		}
	}

	// Web API
	app := fiber.New(fiber.Config{
		// IoT端末の暴走や巨大JSONによるメモリ枯渇を防ぐ。書き込みAPIの
		// 論理的な件数・値サイズ制限はハンドラー側でも検証する。
		BodyLimit:    1024 * 1024,
		ErrorHandler: handlers.GlobalErrorHandler,
	})

	docsBasicMiddleware := basicauth.New(basicauth.Config{
		Users: map[string]string{
			"docs": "{SHA256}" + config.SWAGGER_BASIC,
		},
	})

	app.Get("/docs/*", docsBasicMiddleware, swaggo.HandlerDefault)

	app.Use(con.AccessLogMiddlewareHandler)

	v1 := app.Group("/api/v1")

	v1.Get("/", func(c fiber.Ctx) error {
		return c.SendString("ok!")
	})

	v1.Route("/auth/", con.AuthHandlersSetup)
	v1.Route("/cfg/", con.CfgHandlersSetup)
	v1.Route("/data/", con.DataHandlersSetup)

	app.Use(handlers.NotFoundMiddlewareHandler)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Receive Shutdown Signal...")

		if err := app.Shutdown(); err != nil {
			log.Printf("Fiber shutdown failed: %s", err)
		}

		if config.DATABASE_PROVIDER == "duckdb" {
			log.Println("Flushing WAL to database...")
			if err := db.Exec("CHECKPOINT;").Error; err != nil {
				log.Printf("DuckDB checkpoint failed: %s", err)
			}
		}

		sqlDB, err := db.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				log.Printf("DB close failed: %s", err)
			} else {
				log.Println("Close Database with Write Ahead Logs")
			}
		}

		os.Exit(0)
	}()

	addr := fmt.Sprintf("%s:%d", config.WEB_HOSTNAME, config.WEB_PORT)
	if err := app.Listen(addr); err != nil {
		log.Printf("Server stopped: %s", err)
	}
}
