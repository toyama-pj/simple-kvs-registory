package lib

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Data struct {
	ID        uuid.UUID `gorm:"primaryKey;type:uuid;column:id"`
	Time      time.Time `gorm:"type:timestamptz;column:time"`
	Namespace string    `gorm:"type:varchar;column:namespace"`
	Key       string    `gorm:"type:varchar;column:key"`
	Value     string    `gorm:"column:value"`
}

func (Data) TableName() string {
	return "data"
}

type Filter struct {
	Before    *time.Time
	After     *time.Time
	Namespace string
	Key       string
	Limit     int
	Offset    int
}

type Database struct {
	db *gorm.DB
}

func NewDatabase(db *gorm.DB) Database {
	return Database{db: db}
}

func (d Database) Init() error {
	return d.db.AutoMigrate(&Data{})
}

func (d Database) Write(data Data) error {
	if data.ID == uuid.Nil {
		newUUID, err := uuid.NewV7()
		if err != nil {
			return err
		}
		data.ID = newUUID
	}

	return d.db.Create(&data).Error
}

func (d Database) Read() ([]Data, error) {
	var results []Data

	err := d.db.Order("time DESC").Limit(50).Find(&results).Error
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (d Database) ReadWithFilter(filter Filter) ([]Data, error) {
	var results []Data

	query := d.db.Model(&Data{})

	if filter.Before != nil {
		query = query.Where("time <= ?", *filter.Before)
	}
	if filter.After != nil {
		query = query.Where("time >= ?", *filter.After)
	}
	if filter.Namespace != "" {
		query = query.Where("namespace = ?", filter.Namespace)
	}
	if filter.Key != "" {
		query = query.Where("key = ?", filter.Key)
	}

	query = query.Order("time DESC")

	limit := filter.Limit
	if limit == 0 || limit > 50 {
		limit = 50
	}
	query = query.Limit(limit)

	if filter.Offset != 0 {
		query = query.Offset(filter.Offset)
	}

	err := query.Find(&results).Error
	if err != nil {
		return nil, err
	}

	return results, nil
}
