package main

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/toyama-pj/simple-kvs-registory/lib"
	databaseDialect "github.com/toyama-pj/simple-kvs-registory/lib/db"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestBuildAppServesWebUIAndAPI(t *testing.T) {
	database, err := gorm.Open(databaseDialect.GetDatabaseDialector("duckdb", ":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	if err := lib.MigrateSchema(database); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	app := buildApp(database, lib.Config{})

	for _, test := range []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/", contentType: "text/html", contains: `<main id="app"`},
		{path: "/assets/styles.css", contentType: "text/css", contains: ".metric-card"},
		{path: "/assets/app.js", contentType: "text/javascript", contains: "navigator.credentials"},
	} {
		response, err := app.Test(httptest.NewRequest("GET", test.path, nil))
		if err != nil {
			t.Fatalf("GET %s: %v", test.path, err)
		}
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatalf("read GET %s: %v", test.path, err)
		}
		if response.StatusCode != 200 {
			t.Fatalf("GET %s status = %d, body = %s", test.path, response.StatusCode, body)
		}
		if !strings.Contains(response.Header.Get("Content-Type"), test.contentType) {
			t.Errorf("GET %s Content-Type = %q", test.path, response.Header.Get("Content-Type"))
		}
		if !strings.Contains(string(body), test.contains) {
			t.Errorf("GET %s body does not contain %q", test.path, test.contains)
		}
		if response.Header.Get("Content-Security-Policy") == "" {
			t.Errorf("GET %s has no Content-Security-Policy", test.path)
		}
	}

	response, err := app.Test(httptest.NewRequest("GET", "/api/v1/", nil))
	if err != nil {
		t.Fatalf("GET /api/v1/: %v", err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read GET /api/v1/: %v", err)
	}
	if response.StatusCode != 200 || string(body) != "ok!" {
		t.Fatalf("GET /api/v1/ = %d %q", response.StatusCode, body)
	}
}
