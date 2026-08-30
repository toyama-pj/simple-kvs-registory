package handlers

import (
	"bytes"
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
	database "github.com/toyama-pj/simple-kvs-registory/lib/db"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const resourceTestMasterKey = "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f"

func TestResourceAPIOrganizationDeviceAndMeasurements(t *testing.T) {
	db := resourceTestDB(t)
	user := lib.User{ID: uuid.New(), Name: "owner", Email: "owner@example.com"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	token, err := lib.NewController(db, lib.Config{}).CreateUserBearerToken(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	config := lib.Config{DEVELOPMENT: true, DEVICE_SESSION_KEY_ENCRYPTION_KEY: resourceTestMasterKey}
	controller := NewController(db, config)
	app := fiber.New()
	app.Route("/api/v1/organizations", controller.OrganizationHandlersSetup)
	app.Route("/api/v1/namespaces", controller.NamespaceHandlersSetup)
	app.Route("/api/v1/devices", controller.DeviceHandlersSetup)

	organizationResponse := doResourceRequest(t, app, "POST", "/api/v1/organizations", token.Token, `{"name":"Farm"}`)
	if organizationResponse.StatusCode != fiber.StatusCreated {
		body, _ := io.ReadAll(organizationResponse.Body)
		t.Fatalf("create organization status = %d: %s", organizationResponse.StatusCode, body)
	}
	var organization lib.Organization
	if err := json.NewDecoder(organizationResponse.Body).Decode(&organization); err != nil {
		t.Fatal(err)
	}

	namespaceResponse := doResourceRequest(t, app, "POST", "/api/v1/organizations/"+organization.ID.String()+"/namespaces", token.Token, `{"name":"Greenhouse"}`)
	if namespaceResponse.StatusCode != fiber.StatusCreated {
		t.Fatalf("create namespace status = %d", namespaceResponse.StatusCode)
	}
	var namespace lib.Namespace
	if err := json.NewDecoder(namespaceResponse.Body).Decode(&namespace); err != nil {
		t.Fatal(err)
	}

	deviceBody := `{"name":"Sensor 1","dev_eui":"0102030405060708","dev_addr":"26011bda","app_s_key":"00112233445566778899aabbccddeeff","nwk_s_key":"ffeeddccbbaa99887766554433221100"}`
	deviceResponse := doResourceRequest(t, app, "POST", "/api/v1/namespaces/"+namespace.ID.String()+"/devices", token.Token, deviceBody)
	if deviceResponse.StatusCode != fiber.StatusCreated {
		t.Fatalf("create device status = %d", deviceResponse.StatusCode)
	}
	var rawResponse map[string]any
	if err := json.NewDecoder(deviceResponse.Body).Decode(&rawResponse); err != nil {
		t.Fatal(err)
	}
	if _, exposed := rawResponse["app_s_key"]; exposed {
		t.Fatal("AppSKey was exposed in the API response")
	}
	if rawResponse["dev_addr"] != "26011BDA" {
		t.Fatalf("normalized DevAddr = %#v", rawResponse["dev_addr"])
	}
	deviceID, err := uuid.Parse(rawResponse["id"].(string))
	if err != nil {
		t.Fatal(err)
	}
	patchResponse := doResourceRequest(t, app, "PATCH", "/api/v1/devices/"+deviceID.String(), token.Token, `{"enabled":false}`)
	if patchResponse.StatusCode != fiber.StatusOK {
		body, _ := io.ReadAll(patchResponse.Body)
		t.Fatalf("patch device status = %d: %s", patchResponse.StatusCode, body)
	}
	var patchedDevice lib.Device
	if err := json.NewDecoder(patchResponse.Body).Decode(&patchedDevice); err != nil {
		t.Fatal(err)
	}
	if patchedDevice.Enabled {
		t.Fatal("device was not disabled")
	}

	value, _ := lib.NewJSONValue(23.5)
	measurement := lib.Measurement{DeviceID: deviceID, NamespaceID: namespace.ID, GatewayEUI: "0102030405060708", ReceivedAt: time.Unix(100, 0), FrameCounter: 1, Channel: 1, Type: 103, Name: "temperature", Value: value}
	if err := db.Create(&measurement).Error; err != nil {
		t.Fatal(err)
	}
	measurementResponse := doResourceRequest(t, app, "GET", "/api/v1/devices/"+deviceID.String()+"/measurements?name=temperature", token.Token, "")
	if measurementResponse.StatusCode != fiber.StatusOK {
		t.Fatalf("get measurements status = %d", measurementResponse.StatusCode)
	}
	responseBytes := new(bytes.Buffer)
	if _, err := responseBytes.ReadFrom(measurementResponse.Body); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(responseBytes.String(), `"value":23.5`) {
		t.Fatalf("measurement JSON = %s", responseBytes.String())
	}
}

func doResourceRequest(t *testing.T, app *fiber.App, method, path, token, body string) *http.Response {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func resourceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(database.GetDatabaseDialector("duckdb", ":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := lib.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}
