package lib

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

var ErrForbidden = errors.New("forbidden")

// JSONValue keeps JSON as JSON in API responses and uses PostgreSQL jsonb in
// production while remaining compatible with the lightweight DuckDB test DB.
type JSONValue []byte

func NewJSONValue(value any) (JSONValue, error) {
	encoded, err := json.Marshal(value)
	return JSONValue(encoded), err
}

func (j JSONValue) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "null", nil
	}
	if !json.Valid(j) {
		return nil, errors.New("invalid JSON value")
	}
	return string(j), nil
}

func (j *JSONValue) Scan(value any) error {
	switch typed := value.(type) {
	case nil:
		*j = JSONValue("null")
	case []byte:
		*j = append((*j)[:0], typed...)
	case string:
		*j = append((*j)[:0], typed...)
	default:
		return fmt.Errorf("cannot scan JSON from %T", value)
	}
	return nil
}

func (JSONValue) GormDataType() string { return "json" }

func (JSONValue) GormDBDataType(db *gorm.DB, _ *schema.Field) string {
	if db.Dialector.Name() == "postgres" {
		return "JSONB"
	}
	return "VARCHAR"
}

func (j JSONValue) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	if !json.Valid(j) {
		return nil, errors.New("invalid JSON value")
	}
	return j, nil
}

func (j *JSONValue) UnmarshalJSON(value []byte) error {
	if !json.Valid(value) {
		return errors.New("invalid JSON value")
	}
	*j = append((*j)[:0], value...)
	return nil
}

type Organization struct {
	ID        uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	Name      string         `gorm:"type:varchar(100);column:name" json:"name"`
	CreatedAt time.Time      `gorm:"type:timestamptz;column:created_at" json:"created_at"`
	UpdatedAt time.Time      `gorm:"type:timestamptz;column:updated_at" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-" swaggerignore:"true"`
}

func (Organization) TableName() string { return "organization" }

func (o *Organization) BeforeCreate(_ *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

type OrganizationMembership struct {
	ID             uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;column:organization_id;uniqueIndex:idx_organization_user" json:"organization_id"`
	UserID         uuid.UUID      `gorm:"type:uuid;column:user_id;uniqueIndex:idx_organization_user" json:"user_id"`
	Role           string         `gorm:"type:varchar(16);column:role" json:"role"`
	CreatedAt      time.Time      `gorm:"type:timestamptz;column:created_at" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"type:timestamptz;column:updated_at" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-" swaggerignore:"true"`
}

func (OrganizationMembership) TableName() string { return "organization_membership" }

func (m *OrganizationMembership) BeforeCreate(_ *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

type Namespace struct {
	ID             uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	OrganizationID uuid.UUID      `gorm:"type:uuid;column:organization_id;index" json:"organization_id"`
	Name           string         `gorm:"type:varchar(100);column:name" json:"name"`
	CreatedAt      time.Time      `gorm:"type:timestamptz;column:created_at" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"type:timestamptz;column:updated_at" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-" swaggerignore:"true"`
}

func (Namespace) TableName() string { return "namespace" }

func (n *Namespace) BeforeCreate(_ *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

// Device represents one LoRaWAN end device. Session keys are encrypted before
// they are stored and are never included in JSON responses.
type Device struct {
	ID                 uuid.UUID      `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	NamespaceID        uuid.UUID      `gorm:"type:uuid;column:namespace_id;index" json:"namespace_id"`
	Name               string         `gorm:"type:varchar(100);column:name" json:"name"`
	DevEUI             string         `gorm:"type:varchar(16);column:dev_eui;uniqueIndex" json:"dev_eui"`
	DevAddr            string         `gorm:"type:varchar(8);column:dev_addr;uniqueIndex" json:"dev_addr"`
	AppSKeyEncrypted   string         `gorm:"type:text;column:app_s_key_encrypted" json:"-" swaggerignore:"true"`
	NwkSKeyEncrypted   string         `gorm:"type:text;column:nwk_s_key_encrypted" json:"-" swaggerignore:"true"`
	UplinkFrameCounter uint32         `gorm:"column:uplink_frame_counter" json:"uplink_frame_counter"`
	HasUplinkFrame     bool           `gorm:"column:has_uplink_frame" json:"-" swaggerignore:"true"`
	Enabled            bool           `gorm:"column:enabled;default:true" json:"enabled"`
	CreatedAt          time.Time      `gorm:"type:timestamptz;column:created_at" json:"created_at"`
	UpdatedAt          time.Time      `gorm:"type:timestamptz;column:updated_at" json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-" swaggerignore:"true"`
}

func (Device) TableName() string { return "device" }

func (d *Device) BeforeCreate(_ *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

type Measurement struct {
	ID           uuid.UUID  `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	DeviceID     uuid.UUID  `gorm:"type:uuid;column:device_id;index:idx_measurement_device_time,priority:1" json:"device_id"`
	NamespaceID  uuid.UUID  `gorm:"type:uuid;column:namespace_id;index:idx_measurement_namespace_time,priority:1" json:"namespace_id"`
	GatewayEUI   string     `gorm:"type:varchar(16);column:gateway_eui" json:"gateway_eui"`
	ReceivedAt   time.Time  `gorm:"type:timestamptz;column:received_at;index:idx_measurement_device_time,priority:2;index:idx_measurement_namespace_time,priority:2" json:"received_at"`
	GatewayTime  *time.Time `gorm:"type:timestamptz;column:gateway_time" json:"gateway_time,omitempty"`
	FrameCounter uint32     `gorm:"column:frame_counter" json:"frame_counter"`
	Channel      uint8      `gorm:"column:channel" json:"channel"`
	Type         uint8      `gorm:"column:type" json:"type"`
	Name         string     `gorm:"type:varchar(32);column:name" json:"name"`
	Value        JSONValue  `gorm:"column:value" json:"value" swaggertype:"object"`
}

func (Measurement) TableName() string { return "measurement" }

func (m *Measurement) BeforeCreate(_ *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// SemtechUDPLog is the durable audit trail for every UDP datagram. The
// DatabaseCommitted flag refers to committing decoded measurements, not merely
// committing this log row.
type SemtechUDPLog struct {
	ID                uuid.UUID `gorm:"primaryKey;type:uuid;column:id" json:"id"`
	ReceivedAt        time.Time `gorm:"type:timestamptz;column:received_at;index" json:"received_at"`
	SourceIP          string    `gorm:"type:varchar(45);column:source_ip;index" json:"source_ip"`
	PacketType        string    `gorm:"type:varchar(32);column:packet_type" json:"packet_type"`
	GatewayEUI        string    `gorm:"type:varchar(16);column:gateway_eui;index" json:"gateway_eui,omitempty"`
	DatabaseCommitted bool      `gorm:"column:database_committed" json:"database_committed"`
	Payload           JSONValue `gorm:"column:payload" json:"payload" swaggertype:"object"`
	Error             string    `gorm:"type:text;column:error" json:"error,omitempty"`
}

func (SemtechUDPLog) TableName() string { return "semtech_udp_log" }

func (l *SemtechUDPLog) BeforeCreate(_ *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

func (c *Controller) CreateOrganization(ownerID uuid.UUID, name string) (Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return Organization{}, errors.New("organization name must contain 1 to 100 characters")
	}
	organization := Organization{Name: name}
	err := c.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&User{}, "id = ?", ownerID).Error; err != nil {
			return err
		}
		if err := tx.Create(&organization).Error; err != nil {
			return err
		}
		return tx.Create(&OrganizationMembership{OrganizationID: organization.ID, UserID: ownerID, Role: "owner"}).Error
	})
	return organization, err
}

func (c *Controller) CreateNamespaceForOrganization(actorID, organizationID uuid.UUID, name string) (Namespace, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 100 {
		return Namespace{}, errors.New("namespace name must contain 1 to 100 characters")
	}
	namespace := Namespace{OrganizationID: organizationID, Name: name}
	err := c.DB.Transaction(func(tx *gorm.DB) error {
		var membership OrganizationMembership
		if err := tx.Where("organization_id = ? AND user_id = ? AND deleted_at IS NULL", organizationID, actorID).First(&membership).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrForbidden
			}
			return err
		}
		if membership.Role != "owner" && membership.Role != "admin" {
			return ErrForbidden
		}
		if err := tx.Create(&namespace).Error; err != nil {
			return err
		}
		return tx.Create(&NamespaceAccessPermission{NamespaceID: namespace.ID, UserID: actorID, GrantType: "admin", CreatedAt: time.Now()}).Error
	})
	return namespace, err
}
