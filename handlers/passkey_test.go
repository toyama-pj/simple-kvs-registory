package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
)

func TestPasskeyBeginFlowsPersistOneTimeCeremonies(t *testing.T) {
	db := resourceTestDB(t)
	user := lib.User{ID: uuid.New(), Name: "Passkey User", Email: "passkey@example.com"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	token, err := lib.NewController(db, lib.Config{}).CreateUserBearerToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	config := lib.Config{
		DEVELOPMENT:             true,
		PASSKEY_ENABLED:         true,
		PASSKEY_RP_ID:           "localhost",
		PASSKEY_RP_DISPLAY_NAME: "Simple Chirp Test",
		PASSKEY_RP_ORIGINS:      []string{"http://localhost:3000"},
		SESSION_COOKIE_SECURE:   false,
	}
	controller := NewController(db, config)
	app := fiber.New()
	app.Route("/api/v1/auth/", controller.AuthHandlersSetup)

	request := httptest.NewRequest("POST", "/api/v1/auth/passkeys/register/begin", strings.NewReader(`{"name":"Laptop"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token.Token)
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("registration begin = %d: %s", response.StatusCode, body)
	}
	var registration struct {
		CeremonyID uuid.UUID      `json:"ceremony_id"`
		Options    map[string]any `json:"options"`
	}
	if err := json.NewDecoder(response.Body).Decode(&registration); err != nil {
		t.Fatal(err)
	}
	publicKey := registration.Options["publicKey"].(map[string]any)
	selection := publicKey["authenticatorSelection"].(map[string]any)
	if selection["residentKey"] != "required" || selection["userVerification"] != "required" {
		t.Fatalf("authenticator selection = %#v", selection)
	}
	var ceremony lib.PasskeyCeremony
	if err := db.First(&ceremony, "id = ?", registration.CeremonyID).Error; err != nil {
		t.Fatal(err)
	}
	if ceremony.UserID == nil || *ceremony.UserID != user.ID || ceremony.CredentialName != "Laptop" || ceremony.ExpiresAt.Before(time.Now()) {
		t.Fatalf("registration ceremony = %#v", ceremony)
	}

	loginResponse, err := app.Test(httptest.NewRequest("POST", "/api/v1/auth/passkeys/login/begin", nil))
	if err != nil {
		t.Fatal(err)
	}
	if loginResponse.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(loginResponse.Body)
		t.Fatalf("login begin = %d: %s", loginResponse.StatusCode, body)
	}
	var login struct {
		CeremonyID uuid.UUID      `json:"ceremony_id"`
		Options    map[string]any `json:"options"`
	}
	if err := json.NewDecoder(loginResponse.Body).Decode(&login); err != nil {
		t.Fatal(err)
	}
	loginPublicKey := login.Options["publicKey"].(map[string]any)
	if loginPublicKey["userVerification"] != "required" {
		t.Fatalf("login options = %#v", loginPublicKey)
	}
	if allowed, exists := loginPublicKey["allowCredentials"]; exists && allowed != nil {
		if values, ok := allowed.([]any); ok && len(values) != 0 {
			t.Fatalf("discoverable login contains allowCredentials: %#v", allowed)
		}
	}
}

func TestSessionCookieAuthenticatesAndLogoutRevokesToken(t *testing.T) {
	db := resourceTestDB(t)
	user := lib.User{ID: uuid.New(), Name: "Cookie User", Email: "cookie@example.com"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	token, err := lib.NewController(db, lib.Config{}).CreateUserBearerToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	controller := NewController(db, lib.Config{SESSION_COOKIE_SECURE: false})
	app := fiber.New()
	app.Route("/api/v1/auth/", controller.AuthHandlersSetup)

	request := httptest.NewRequest("GET", "/api/v1/auth/passkeys", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token.Token})
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != fiber.StatusOK {
		t.Fatalf("cookie auth status = %d", response.StatusCode)
	}
	if setCookie := response.Header.Get("Set-Cookie"); !strings.Contains(setCookie, "HttpOnly") || !strings.Contains(setCookie, "SameSite=Strict") {
		t.Fatalf("refreshed cookie = %q", setCookie)
	}

	logout := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	logout.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token.Token})
	logoutResponse, err := app.Test(logout)
	if err != nil {
		t.Fatal(err)
	}
	if logoutResponse.StatusCode != fiber.StatusNoContent {
		t.Fatalf("logout status = %d", logoutResponse.StatusCode)
	}
	var active int64
	if err := db.Model(&lib.UserBearerToken{}).Where("id = ? AND deleted_at IS NULL", token.ID).Count(&active).Error; err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatal("logout did not revoke bearer token")
	}
}
