package lib

import (
	"time"
)

type AccessLog struct {
	ID          int         `gorm:"primaryKey;column:id;autoIncrement" json:"id"`
	Time        time.Time   `json:"time"`
	Endpoint    string      `gorm:"type:varchar" json:"endpoint"`
	IPAddr      string      `gorm:"type:varchar" json:"ip_addr"`
	RequestType string      `gorm:"type:varchar" json:"request_type"`
	StatusCode  int         `json:"status_code"`
	ProcessTime float32     `json:"process_time"`
	RequestBody interface{} `gorm:"serializer:json;type:varchar" json:"request_body"`
}

func (AccessLog) TableName() string {
	return "access_log"
}

func (c *Controller) SaveAccessLogAsync(log AccessLog) {
	go func() {
		_ = c.DB.Create(&log).Error
	}()
}
