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

	PASSKEY_ENABLED         bool     `env:"PASSKEY_ENABLED" envDefault:"false"`
	PASSKEY_RP_ID           string   `env:"PASSKEY_RP_ID"`
	PASSKEY_RP_DISPLAY_NAME string   `env:"PASSKEY_RP_DISPLAY_NAME" envDefault:"Simple Chirp"`
	PASSKEY_RP_ORIGINS      []string `env:"PASSKEY_RP_ORIGINS" envSeparator:","`
	SESSION_COOKIE_SECURE   bool     `env:"SESSION_COOKIE_SECURE" envDefault:"true"`

	SEMTECH_UDP_ENABLED   bool   `env:"SEMTECH_UDP_ENABLED" envDefault:"false"`
	SEMTECH_UDP_BIND_HOST string `env:"SEMTECH_UDP_BIND_HOST" envDefault:"0.0.0.0"`
	SEMTECH_UDP_BIND_PORT int    `env:"SEMTECH_UDP_BIND_PORT" envDefault:"1700"`

	DEVICE_SESSION_KEY_ENCRYPTION_KEY string `env:"DEVICE_SESSION_KEY_ENCRYPTION_KEY"`
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
	if config.SEMTECH_UDP_ENABLED {
		if config.SEMTECH_UDP_BIND_PORT < 1 || config.SEMTECH_UDP_BIND_PORT > 65535 {
			return Config{}, fmt.Errorf("SEMTECH_UDP_BIND_PORT must be between 1 and 65535")
		}
		if err := ValidateSessionKeyEncryptionKey(config.DEVICE_SESSION_KEY_ENCRYPTION_KEY); err != nil {
			return Config{}, err
		}
	}
	if config.PASSKEY_ENABLED {
		if config.PASSKEY_RP_ID == "" || len(config.PASSKEY_RP_ORIGINS) == 0 {
			return Config{}, fmt.Errorf("PASSKEY_RP_ID and PASSKEY_RP_ORIGINS are required when passkeys are enabled")
		}
	}

	return config, nil
}
