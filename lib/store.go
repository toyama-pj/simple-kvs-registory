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
	Before    time.Time
	After     time.Time
	Namespace uuid.UUID
	Key       string
	Limit     int
	Offset    int
	TimeOrder string
}

type Database struct {
	db *gorm.DB
}

func (con Controller) Write(data Data) error {
	if data.ID == uuid.Nil {
		newUUID, err := uuid.NewV7()
		if err != nil {
			return err
		}
		data.ID = newUUID
	}

	return con.DB.Create(&data).Error
}

func (con Controller) Read(namespace uuid.UUID) ([]Data, error) {
	var results []Data

	err := con.DB.Order("time DESC").Where("namespace = ?", namespace).Limit(50).Find(&results).Error
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (con Controller) ReadWithFilter(filter Filter) ([]Data, error) {
	var results []Data

	query := con.DB.Model(&Data{})

	if !filter.Before.IsZero() {
		query = query.Where("time <= ?", filter.Before)
	}
	if !filter.After.IsZero() {
		query = query.Where("time >= ?", filter.After)
	}
	if filter.Namespace != uuid.Nil {
		query = query.Where("namespace = ?", filter.Namespace)
	}
	if filter.Key != "" {
		query = query.Where("key = ?", filter.Key)
	}

	if filter.TimeOrder == "ASC" {
		query = query.Order("time ASC")
	} else {
		query = query.Order("time DESC")
	}

	limit := filter.Limit
	if limit <= 0 || limit > 50 {
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
