package handlers

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
)

func (con *Controller) AccessLogMiddlewareHandler(c fiber.Ctx) error {
	if c.Method() == fiber.MethodGet && (c.Path() == "/" || c.Path() == "/api/v1/") {
		return c.Next()
	}

	startTime := time.Now()

	var reqBody interface{}
	if c.Method() == fiber.MethodPost || c.Method() == fiber.MethodPut || c.Method() == fiber.MethodPatch {
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
		RequestBody: sanitizeAccessLogBody(c.Path(), reqBody),
	}
	if userID := c.Locals("userId"); userID != nil {
		accessLog.Actor = fmt.Sprint(userID)
	} else if tokenID := c.Locals("writeAccessTokenId"); tokenID != nil {
		accessLog.Actor = fmt.Sprintf("write-token:%v", tokenID)
	}

	log.Printf("[%s] %s %s %d %fms\n", accessLog.IPAddr, accessLog.RequestType, accessLog.Endpoint, accessLog.StatusCode, accessLog.ProcessTime*1000)

	// Save to DB asynchronously
	con.ReturnLibController().SaveAccessLogAsync(accessLog)

	return err
}

func sanitizeAccessLogBody(path string, body interface{}) interface{} {
	if strings.HasPrefix(path, "/api/v1/data/") {
		return map[string]interface{}{"redacted": true, "reason": "key-value payload"}
	}
	values, ok := body.(map[string]interface{})
	if !ok {
		return body
	}
	for _, key := range []string{"code", "token", "password", "app_s_key", "nwk_s_key"} {
		if _, exists := values[key]; exists {
			values[key] = "[REDACTED]"
		}
	}
	return values
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
	tokenHash := lib.HashToken(tokenString)
	err := con.DB.Where("token_hash = ? OR token = ?", tokenHash, tokenString).Where("expires_at > ?", time.Now()).Where("deleted_at IS NULL").First(&tokenRecord).Error
	if err == nil {
		updates := map[string]interface{}{"expires_at": time.Now().Add(time.Hour * 24)}
		if tokenRecord.TokenHash == "" {
			// Transparently migrate legacy plaintext credentials after a valid use.
			updates["token_hash"] = tokenHash
			updates["token"] = ""
		}
		err = con.DB.Model(&lib.UserBearerToken{}).Where("id = ?", tokenRecord.ID).Updates(updates).Error
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
	writeQuery := con.DB.Where("token_hash = ?", tokenHash).Where("expires_at > ?", time.Now()).Where("deleted_at IS NULL")
	writeErr := writeQuery.First(&writeTokenRecord).Error
	if writeErr != nil && con.DB.Migrator().HasColumn("write_access_tokens", "token") {
		writeErr = con.DB.Where("CAST(token AS VARCHAR) = ?", tokenString).Where("expires_at > ?", time.Now()).Where("deleted_at IS NULL").First(&writeTokenRecord).Error
	}
	if writeErr == nil {
		if writeTokenRecord.TokenHash == "" {
			_ = con.DB.Model(&writeTokenRecord).Updates(map[string]interface{}{"token_hash": tokenHash, "token": uuid.Nil}).Error
		}
		c.Locals("writeAccessTokenNamespaceId", writeTokenRecord.NameSpaceID)
		c.Locals("writeAccessTokenId", writeTokenRecord.ID)
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
	return c.Status(fiber.StatusNotFound).JSON(lib.NewRFCErrorResponse(lib.ErrorCommonNotFound, "Not Found", fiber.StatusNotFound, "The requested endpoint does not exist", c.Path()))
}

func GlobalErrorHandler(c fiber.Ctx, err error) error {
	status := fiber.StatusInternalServerError
	detail := "Internal Server Error"
	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		status = fiberError.Code
		detail = fiberError.Message
	}
	title := "Request Failed"
	if status == fiber.StatusRequestEntityTooLarge {
		title = "Payload Too Large"
		detail = "Request body may contain at most 1048576 bytes"
	}
	return c.Status(status).JSON(lib.NewRFCErrorResponse(lib.ErrorInvalidRequest, title, status, detail, c.Path()))
}

func NotImplementedMiddlewareHandler(c fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).SendString("Not Implemented")
}
