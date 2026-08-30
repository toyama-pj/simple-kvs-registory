package lib

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PasskeyCredential stores the complete WebAuthn credential record as JSON.
// CredentialID is duplicated in a searchable form for authentication lookup.
type PasskeyCredential struct {
	ID           uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID       uuid.UUID      `gorm:"type:uuid;column:user_id;index" json:"user_id"`
	CredentialID string         `gorm:"type:varchar(1024);column:credential_id;uniqueIndex" json:"-" swaggerignore:"true"`
	Name         string         `gorm:"type:varchar(100);column:name" json:"name"`
	Credential   JSONValue      `gorm:"column:credential" json:"-" swaggerignore:"true"`
	CreatedAt    time.Time      `gorm:"type:timestamptz;column:created_at" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"type:timestamptz;column:updated_at" json:"updated_at"`
	LastUsedAt   *time.Time     `gorm:"type:timestamptz;column:last_used_at" json:"last_used_at,omitempty"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-" swaggerignore:"true"`
	User         User           `gorm:"foreignKey:UserID;constraint:OnDelete:RESTRICT" json:"-" swaggerignore:"true"`
}

func (PasskeyCredential) TableName() string { return "passkey_credential" }

func (credential *PasskeyCredential) BeforeCreate(_ *gorm.DB) error {
	if credential.ID == uuid.Nil {
		credential.ID = uuid.New()
	}
	return nil
}

// PasskeyCeremony persists the exact SessionData returned by go-webauthn.
// Rows are consumed once and expire quickly to prevent challenge replay.
type PasskeyCeremony struct {
	ID             uuid.UUID  `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	UserID         *uuid.UUID `gorm:"type:uuid;column:user_id;index" json:"-"`
	Flow           string     `gorm:"type:varchar(16);column:flow;index" json:"-"`
	CredentialName string     `gorm:"type:varchar(100);column:credential_name" json:"-"`
	Session        JSONValue  `gorm:"column:session" json:"-"`
	CreatedAt      time.Time  `gorm:"type:timestamptz;column:created_at" json:"-"`
	ExpiresAt      time.Time  `gorm:"type:timestamptz;column:expires_at;index" json:"-"`
	ConsumedAt     *time.Time `gorm:"type:timestamptz;column:consumed_at" json:"-"`
}

func (PasskeyCeremony) TableName() string { return "passkey_ceremony" }

func (ceremony *PasskeyCeremony) BeforeCreate(_ *gorm.DB) error {
	if ceremony.ID == uuid.Nil {
		ceremony.ID = uuid.New()
	}
	return nil
}
