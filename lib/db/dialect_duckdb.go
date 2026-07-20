//go:build !noduckdb

package db

import (
	"github.com/alifiroozi80/duckdb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func GetDatabaseDialector(provider string, dsn string) gorm.Dialector {
	if provider == "duckdb" {
		return duckdb.Open(dsn)
	} else if provider == "postgres" {
		return postgres.Open(dsn)
	}
	panic("unsupported database provider")
}
