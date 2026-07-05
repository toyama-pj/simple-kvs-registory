package lib

import (
	"encoding/json"

	"github.com/gofiber/fiber/v3"
)

type RFCErrorResponse struct {
	// RFC 7807 に準拠した API エラーレスポンスを返却するための構造体
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

type ErrorType string

const (
	ErrUnexpected ErrorType = "UNEXPECTED_ERROR"
)

func NewRFCErrorResponse(status int, title, detail, instance string) RFCErrorResponse {
	return RFCErrorResponse{
		Type:     string(ErrUnexpected),
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}
}

func NewRFCUnauthorizedErrorResponse(detail string, instance string) RFCErrorResponse {
	return NewRFCErrorResponse(
		fiber.StatusUnauthorized,
		"err/login/unauthorized",
		detail,
		instance,
	)
}

func NewRFCNotImplementErrorResponse(instance string) RFCErrorResponse {
	return NewRFCErrorResponse(
		fiber.StatusNotImplemented,
		"err/login/not_implemented",
		"This feature is not implemented",
		instance,
	)
}

// error インターフェースを満たすための実装
func (e RFCErrorResponse) Error() string {
	return e.Detail
}

// ToJSON 構造体全体を JSON データのバイト列（json.RawMessage）として返却する
func (e RFCErrorResponse) ToJSON() json.RawMessage {
	bytes, err := json.Marshal(e)
	if err != nil {
		return json.RawMessage(`{"title":"Internal Error"}`)
	}
	return json.RawMessage(bytes)
}
