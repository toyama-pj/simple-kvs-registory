package lib

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
)

type AccessLog struct {
	Time        time.Time         `json:"time"`
	Endpoint    string            `json:"endpoint"`
	IPAddr      string            `json:"ip_addr"`
	RequestType string            `json:"request_type"`
	StatusCode  int               `json:"status_code"`
	ProcessTime float32           `json:"process_time"`
	RequestBody map[string]string `gorm:"serializer:json" json:"request_body"`
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

func AuthenticationMiddlewareHandler(c fiber.Ctx) error {
	return c.Next()
}
