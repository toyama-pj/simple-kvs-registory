package lib

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

type Config struct {
	DEVELOPMENT bool   `env:"DEVELOPMENT"`
	PRODUCTION  string `env:"PRODUCTION"`

	WEB_HOSTNAME string `env:"WEB_HOSTNAME"`
	WEB_PORT     int    `env:"WEB_PORT"`

	SMTP_HOST     string `env:"SMTP_HOST"`
	SMTP_PORT     int    `env:"SMTP_PORT"`
	SMTP_USERNAME string `env:"SMTP_USERNAME"`
	SMTP_PASSWORD string `env:"SMTP_PASSWORD"`

	DATABASE_PROVIDER string `env:"DATABASE_PROVIDER"`
	DATABASE_DSN      string `env:"DATABASE_DSN"`

	SWAGGER_BASIC string `env:"SWAGGER_BASIC"`
}

type Controller struct {
	DB     *gorm.DB
	Config Config
}

func NewController(db *gorm.DB, config Config) *Controller {
	return &Controller{DB: db, Config: config}
}

func ReadConfig(path string, fallbackToOSEnv bool) (Config, error) {
	if path != "" {
		err := godotenv.Load(path)
		if err != nil {
			if !fallbackToOSEnv {
				return Config{}, fmt.Errorf("failed to load env file and fallback is disabled: %w", err)
			}
		}
	}

	var config Config
	if err := env.Parse(&config); err != nil {
		return Config{}, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	return config, nil
}
