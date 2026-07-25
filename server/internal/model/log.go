package model

import (
	"time"
)

type Log struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	OperationID string    `json:"-" gorm:"index"`
	Message     string    `json:"message"`
	Tag         string    `json:"tag" gorm:"column:tag;index"`
	Detail      string    `json:"detail"`
	CreatedAt   time.Time `json:"created_at" gorm:"index"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"index"`
}

func (Log) TableName() string {
	return "logs"
}

type LogFilter struct {
	Level     string
	Tag       string
	StartTime *time.Time
	EndTime   *time.Time
	Keyword   string
	Limit     int
	Cursor    string
}
