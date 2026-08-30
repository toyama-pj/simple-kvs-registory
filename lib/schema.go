package lib

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MigrateSchema avoids running a full AutoMigrate on every DuckDB startup,
// which cannot alter a table while dependent indexes exist. Existing databases
// receive only the security columns and indexes introduced after the initial
// schema; missing tables are still created normally.
func MigrateSchema(db *gorm.DB) error {
	models := []interface{}{
		&User{},
		&Organization{},
		&OrganizationMembership{},
		&Namespace{},
		&Device{},
		&Measurement{},
		&SemtechUDPLog{},
		&Data{},
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
			if err := db.Migrator().CreateTable(model); err != nil {
				return fmt.Errorf("create missing table %T: %w", model, err)
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
	return backfillLegacyNamespaces(db)
}

// Older releases represented a namespace only by IDs in permission rows.
// Preserve those IDs while giving each legacy namespace a real parent
// organization and an owner.
func backfillLegacyNamespaces(db *gorm.DB) error {
	var namespaceIDs []uuid.UUID
	if err := db.Model(&NamespaceAccessPermission{}).Distinct("namespace_id").Pluck("namespace_id", &namespaceIDs).Error; err != nil {
		return fmt.Errorf("list legacy namespaces: %w", err)
	}
	for _, namespaceID := range namespaceIDs {
		var count int64
		if err := db.Model(&Namespace{}).Where("id = ?", namespaceID).Count(&count).Error; err != nil {
			return fmt.Errorf("check legacy namespace %s: %w", namespaceID, err)
		}
		if count != 0 {
			continue
		}
		if err := db.Transaction(func(tx *gorm.DB) error {
			var ownerGrant NamespaceAccessPermission
			err := tx.Where("namespace_id = ? AND grant_type = ? AND deleted_at IS NULL", namespaceID, "admin").Order("id").First(&ownerGrant).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				err = tx.Where("namespace_id = ? AND deleted_at IS NULL", namespaceID).Order("id").First(&ownerGrant).Error
			}
			if err != nil {
				return err
			}
			organization := Organization{Name: "Migrated organization"}
			if err := tx.Create(&organization).Error; err != nil {
				return err
			}
			membership := OrganizationMembership{OrganizationID: organization.ID, UserID: ownerGrant.UserID, Role: "owner"}
			if err := tx.Create(&membership).Error; err != nil {
				return err
			}
			return tx.Create(&Namespace{ID: namespaceID, OrganizationID: organization.ID, Name: "Migrated namespace"}).Error
		}); err != nil {
			return fmt.Errorf("backfill legacy namespace %s: %w", namespaceID, err)
		}
	}
	return nil
}
