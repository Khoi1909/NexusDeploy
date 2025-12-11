package models

import (
	"time"

	"github.com/google/uuid"
)

// BuildLog represents a single log line from a build (SRS B.3)
type BuildLog struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	BuildID   uuid.UUID `gorm:"type:uuid;not null;index"`
	Timestamp time.Time `gorm:"type:timestamptz;not null"`
	LogLine   string    `gorm:"type:text;not null"`
}

// TableName specifies the table name for BuildLog
func (BuildLog) TableName() string {
	return "build_logs"
}

