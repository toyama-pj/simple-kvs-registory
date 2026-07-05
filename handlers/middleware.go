package handlers

import (
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/toyama-pj/simple-kvs-registory/lib"
)

type AccessLog struct {
	Time        time.Time         `json:"time"`
	Endpoint    string            `gorm:"type:varchar" json:"endpoint"`
	IPAddr      string            `gorm:"type:varchar" json:"ip_addr"`
	RequestType string            `gorm:"type:varchar" json:"request_type"`
	StatusCode  int               `json:"status_code"`
	ProcessTime float32           `json:"process_time"`
	RequestBody map[string]string `gorm:"serializer:json;type:varchar" json:"request_body"`
}

func AccessLogMiddlewareHandler(c fiber.Ctx) error {
	startTime := time.Now()

	var reqBody map[string]string
	if c.Method() == fiber.MethodPost || c.Method() == fiber.MethodPut {
		if err := c.Bind().Body(&reqBody); err != nil {
			reqBody = make(map[string]string)
		}
	}
	err := c.Next()

	status := c.Response().StatusCode()
	latency := float32(time.Since(startTime).Seconds())

	accessLog := AccessLog{
		Time:        startTime,
		Endpoint:    c.Path(),
		IPAddr:      c.IP(),
		RequestType: c.Method(),
		StatusCode:  status,
		ProcessTime: latency,
		RequestBody: reqBody,
	}

	log.Println(&accessLog)

	return err
}

func (con *Controller) AuthenticationMiddlewareHandler(c fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(
			lib.NewRFCUnauthorizedErrorResponse(
				"authorization header is not set.",
				c.Path(),
			),
		)
	}

	if !strings.HasPrefix(authHeader, "Bearer ") {
		return c.Status(fiber.StatusUnauthorized).JSON(
			lib.NewRFCUnauthorizedErrorResponse(
				"invalid authorization header format.",
				c.Path(),
			),
		)
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(
			lib.NewRFCUnauthorizedErrorResponse(
				"authorization header is empty.",
				c.Path(),
			),
		)
	}

	var tokenRecord lib.UserBearerToken
	err := con.DB.Where("token = ?", tokenString).Where("expires_at > ?", time.Now()).Where("deleted_at IS NULL").First(&tokenRecord).Error
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(
			lib.NewRFCUnauthorizedErrorResponse(
				"invalid authorization token.",
				c.Path(),
			),
		)
	}

	tokenRecord.ExpiresAt = time.Now().Add(time.Hour * 24)
	err = con.DB.Save(&tokenRecord).Error
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			lib.NewRFCErrorResponse(
				fiber.StatusInternalServerError,
				"err/login/database_error_update_token",
				"failed to update authorization token.",
				c.Path(),
			),
		)
	}

	c.Locals("userID", tokenRecord.UserID)
	return c.Next()
}

func NotFoundMiddlewareHandler(c fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).SendString("Not Found")
}

func NotImplementedMiddlewareHandler(c fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).SendString("Not Implemented")
}
