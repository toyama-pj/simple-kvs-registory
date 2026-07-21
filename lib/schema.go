package lib

import (
	"fmt"

	"gorm.io/gorm"
)

// MigrateSchema avoids running a full AutoMigrate on every DuckDB startup,
// which cannot alter a table while dependent indexes exist. Existing databases
// receive only the security columns and indexes introduced after the initial
// schema; missing tables are still created normally.
func MigrateSchema(db *gorm.DB) error {
	models := []interface{}{
		&Data{},
		&User{},
		&UserOneTimeLogin{},
		&UserBearerToken{},
		&NamespaceAccessPermission{},
		&WriteAccessToken{},
		&AccessLog{},
		&UserRegistration{},
	}

	if !db.Migrator().HasTable(&User{}) {
		return db.AutoMigrate(models...)
	}
	for _, model := range models {
		if !db.Migrator().HasTable(model) {
			if err := db.AutoMigrate(model); err != nil {
				return fmt.Errorf("create missing table: %w", err)
			}
		}
	}

	columns := []struct {
		model interface{}
		field string
	}{
		{&UserBearerToken{}, "TokenHash"},
		{&WriteAccessToken{}, "TokenHash"},
		{&AccessLog{}, "Actor"},
	}
	for _, column := range columns {
		if !db.Migrator().HasColumn(column.model, column.field) {
			if err := db.Migrator().AddColumn(column.model, column.field); err != nil {
				return fmt.Errorf("add column %s: %w", column.field, err)
			}
		}
	}

	indexes := []struct {
		model interface{}
		name  string
	}{
		{&UserBearerToken{}, "idx_user_bearer_token_hash"},
		{&WriteAccessToken{}, "idx_write_access_token_hash"},
		{&NamespaceAccessPermission{}, "idx_namespace_user"},
	}
	for _, index := range indexes {
		if !db.Migrator().HasIndex(index.model, index.name) {
			if err := db.Migrator().CreateIndex(index.model, index.name); err != nil {
				return fmt.Errorf("create index %s: %w", index.name, err)
			}
		}
	}
	return nil
}
