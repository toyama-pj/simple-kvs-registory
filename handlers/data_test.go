package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
)

func int64Pointer(value int64) *int64    { return &value }
func stringPointer(value string) *string { return &value }

func TestValidateKeyValuePayload(t *testing.T) {
	tests := []struct {
		name    string
		payload KeyValueRequestPayload
		valid   bool
	}{
		{name: "valid including empty value", payload: KeyValueRequestPayload{KeyValueWithTime: []KeyValuesAtTime{{Time: int64Pointer(0), KeyValues: []KeyValue{{Key: "temperature", Value: stringPointer("")}}}}}, valid: true},
		{name: "empty batch", payload: KeyValueRequestPayload{}, valid: false},
		{name: "missing time", payload: KeyValueRequestPayload{KeyValueWithTime: []KeyValuesAtTime{{KeyValues: []KeyValue{{Key: "key", Value: stringPointer("value")}}}}}, valid: false},
		{name: "empty key", payload: KeyValueRequestPayload{KeyValueWithTime: []KeyValuesAtTime{{Time: int64Pointer(1), KeyValues: []KeyValue{{Value: stringPointer("value")}}}}}, valid: false},
		{name: "missing value", payload: KeyValueRequestPayload{KeyValueWithTime: []KeyValuesAtTime{{Time: int64Pointer(1), KeyValues: []KeyValue{{Key: "key"}}}}}, valid: false},
		{name: "value too large", payload: KeyValueRequestPayload{KeyValueWithTime: []KeyValuesAtTime{{Time: int64Pointer(1), KeyValues: []KeyValue{{Key: "key", Value: stringPointer(strings.Repeat("x", maxValueBytes+1))}}}}}, valid: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validateKeyValuePayload(&test.payload) == ""; got != test.valid {
				t.Fatalf("valid = %v, want %v", got, test.valid)
			}
		})
	}
}

func TestDataRouteMatchesSwaggerPath(t *testing.T) {
	app := fiber.New()
	controller := &Controller{}
	app.Route("/api/v1/data/", controller.DataHandlersSetup)
	response, err := app.Test(httptest.NewRequest("GET", "/api/v1/data/00000000-0000-0000-0000-000000000000", nil))
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; Swagger route may not match implementation", response.StatusCode, fiber.StatusUnauthorized)
	}
}

func TestDataCursorRoundTrip(t *testing.T) {
	wantTime := time.Date(2026, 7, 21, 1, 2, 3, 456000000, time.UTC)
	wantID := uuid.New()
	encoded := encodeDataCursor(lib.Data{Time: wantTime, ID: wantID})
	gotTime, gotID, err := decodeDataCursor(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !gotTime.Equal(wantTime) || gotID != wantID {
		t.Fatalf("cursor = (%v, %v), want (%v, %v)", gotTime, gotID, wantTime, wantID)
	}
	if _, _, err := decodeDataCursor("invalid"); err == nil {
		t.Fatal("malformed cursor was accepted")
	}
}

func TestSanitizeAccessLogBody(t *testing.T) {
	body := map[string]interface{}{"email": "user@example.com", "code": "123456"}
	sanitized := sanitizeAccessLogBody("/api/v1/auth/login/callback", body).(map[string]interface{})
	if sanitized["code"] != "[REDACTED]" {
		t.Fatalf("code was not redacted: %#v", sanitized)
	}

	data := sanitizeAccessLogBody("/api/v1/data/00000000-0000-0000-0000-000000000000", map[string]interface{}{"value": "secret"}).(map[string]interface{})
	if data["redacted"] != true {
		t.Fatalf("data payload was not redacted: %#v", data)
	}
}
