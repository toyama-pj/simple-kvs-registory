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
type StatusType string

const (
	ErrorUnexpected ErrorType = "error/unexpected"

	ErrorCommonUnauthorized   ErrorType = "error/common/unauthorized"
	ErrorCommonNotFound       ErrorType = "error/common/not_found"
	ErrorCommonNotImplemented ErrorType = "error/common/not_implemented"

	ErrorInvalidRequest      ErrorType = "error/common/invalid_request"
	ErrorInternalServerError ErrorType = "error/common/internal_server_error"
	ErrorDatabaseError       ErrorType = "error/common/database_error"

	ErrorAuthTokenError ErrorType = "error/auth/invalid_token"
)

func NewRFCErrorResponse(ErrorType ErrorType, status int, title, detail, instance string) RFCErrorResponse {
	return RFCErrorResponse{
		Type:     string(ErrorType),
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}
}

func NewRFCUnauthorizedErrorResponse(detail string, instance string) RFCErrorResponse {
	return NewRFCErrorResponse(
		ErrorCommonUnauthorized,
		fiber.StatusUnauthorized,
		"Unauthorized Error",
		detail,
		instance,
	)
}

func NewRFCNotImplementErrorResponse(instance string) RFCErrorResponse {
	return NewRFCErrorResponse(
		ErrorCommonNotImplemented,
		fiber.StatusNotImplemented,
		"Not Implemented Error",
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
