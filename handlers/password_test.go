package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/toyama-pj/simple-kvs-registory/lib"
)

func TestPasswordSetChangeAndLogin(t *testing.T) {
	db := resourceTestDB(t)
	user := lib.User{ID: uuid.New(), Name: "Password User", Email: "password@example.com"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	libController := lib.NewController(db, lib.Config{})
	currentToken, err := libController.CreateUserBearerToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	otherToken, err := libController.CreateUserBearerToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	controller := NewController(db, lib.Config{SESSION_COOKIE_SECURE: false})
	app := fiber.New()
	app.Route("/api/v1/auth/", controller.AuthHandlersSetup)

	status := passwordRequest(t, app, http.MethodGet, "/api/v1/auth/password", currentToken.Token, "")
	if status.StatusCode != fiber.StatusOK {
		t.Fatalf("initial password status = %d", status.StatusCode)
	}
	var initial PasswordStatusResponse
	if err := json.NewDecoder(status.Body).Decode(&initial); err != nil {
		t.Fatal(err)
	}
	if initial.Configured {
		t.Fatal("new user unexpectedly has a password")
	}

	firstPassword := "first correct horse battery"
	set := passwordRequest(t, app, http.MethodPut, "/api/v1/auth/password", currentToken.Token, `{"new_password":"`+firstPassword+`"}`)
	if set.StatusCode != fiber.StatusNoContent {
		body, _ := io.ReadAll(set.Body)
		t.Fatalf("set password = %d: %s", set.StatusCode, body)
	}
	if err := db.First(&user, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if user.PasswordHash == "" || strings.Contains(user.PasswordHash, firstPassword) {
		t.Fatal("password was not safely hashed")
	}
	var otherActive int64
	if err := db.Model(&lib.UserBearerToken{}).Where("id = ? AND deleted_at IS NULL", otherToken.ID).Count(&otherActive).Error; err != nil {
		t.Fatal(err)
	}
	if otherActive != 0 {
		t.Fatal("password set did not revoke another session")
	}

	wrong := passwordRequest(t, app, http.MethodPost, "/api/v1/auth/password/login", "", `{"email":"password@example.com","password":"wrong password value"}`)
	if wrong.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("wrong password login = %d", wrong.StatusCode)
	}
	login := passwordRequest(t, app, http.MethodPost, "/api/v1/auth/password/login", "", `{"email":"password@example.com","password":"`+firstPassword+`"}`)
	if login.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(login.Body)
		t.Fatalf("password login = %d: %s", login.StatusCode, body)
	}
	if cookie := login.Header.Get("Set-Cookie"); !strings.Contains(cookie, "HttpOnly") || !strings.Contains(cookie, "SameSite=Strict") {
		t.Fatalf("password login cookie = %q", cookie)
	}

	wrongChange := passwordRequest(t, app, http.MethodPut, "/api/v1/auth/password", currentToken.Token, `{"current_password":"not the current password","new_password":"second correct horse battery"}`)
	if wrongChange.StatusCode != fiber.StatusUnauthorized {
		t.Fatalf("wrong current password change = %d", wrongChange.StatusCode)
	}
	change := passwordRequest(t, app, http.MethodPut, "/api/v1/auth/password", currentToken.Token, `{"current_password":"`+firstPassword+`","new_password":"second correct horse battery"}`)
	if change.StatusCode != fiber.StatusNoContent {
		body, _ := io.ReadAll(change.Body)
		t.Fatalf("change password = %d: %s", change.StatusCode, body)
	}
}

func passwordRequest(t *testing.T, app *fiber.App, method, path, token, body string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}
