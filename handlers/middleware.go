package handlers

import (
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/toyama-pj/simple-kvs-registory/lib"
)

func (con *Controller) AccessLogMiddlewareHandler(c fiber.Ctx) error {
	startTime := time.Now()

	var reqBody interface{}
	if c.Method() == fiber.MethodPost || c.Method() == fiber.MethodPut {
		if err := c.Bind().Body(&reqBody); err != nil {
			reqBody = make(map[string]interface{})
		}
	}
	err := c.Next()

	status := c.Response().StatusCode()
	latency := float32(time.Since(startTime).Seconds())

	accessLog := lib.AccessLog{
		Time:        startTime,
		Endpoint:    c.Path(),
		IPAddr:      c.IP(),
		RequestType: c.Method(),
		StatusCode:  status,
		ProcessTime: latency,
		RequestBody: reqBody,
	}

	log.Printf("[%s] %s %s %d %fms\n", accessLog.IPAddr, accessLog.RequestType, accessLog.Endpoint, accessLog.StatusCode, accessLog.ProcessTime*1000)

	// Save to DB asynchronously
	con.ReturnLibController().SaveAccessLogAsync(accessLog)

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
	if err == nil {
		tokenRecord.ExpiresAt = time.Now().Add(time.Hour * 24)
		err = con.DB.Save(&tokenRecord).Error
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(
				lib.NewRFCErrorResponse(
					lib.ErrorDatabaseError,
					"Database Error",
					fiber.StatusInternalServerError,
					"failed to update authorization token.",
					c.Path(),
				),
			)
		}
		c.Locals("userId", tokenRecord.UserID)
		return c.Next()
	}

	var writeTokenRecord lib.WriteAccessToken
	if err := con.DB.Where("token = ?", tokenString).Where("expires_at > ?", time.Now()).Where("deleted_at IS NULL").First(&writeTokenRecord).Error; err == nil {
		c.Locals("writeAccessTokenNamespaceId", writeTokenRecord.NameSpaceID)
		return c.Next()
	}

	return c.Status(fiber.StatusUnauthorized).JSON(
		lib.NewRFCUnauthorizedErrorResponse(
			"invalid authorization token.",
			c.Path(),
		),
	)

}

func NotFoundMiddlewareHandler(c fiber.Ctx) error {
	return c.Status(fiber.StatusNotFound).SendString("Not Found")
}

func NotImplementedMiddlewareHandler(c fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).SendString("Not Implemented")
}
