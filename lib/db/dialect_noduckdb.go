//go:build noduckdb

package db

import (
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func GetDatabaseDialector(provider string, dsn string) gorm.Dialector {
	if provider == "postgres" {
		return postgres.Open(dsn)
	}
	panic("unsupported database provider (DuckDB is disabled in this build)")
}
