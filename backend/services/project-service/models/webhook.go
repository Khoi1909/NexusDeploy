package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Webhook represents a GitHub webhook for a project
type Webhook struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ProjectID       uuid.UUID `gorm:"type:uuid;not null;index"`
	GithubWebhookID int64     `gorm:"not null"`
	Secret          string    `gorm:"type:varchar(255);not null"` // Webhook secret for signature validation
	CreatedAt       time.Time `gorm:"not null;default:now()"`
}

// BeforeCreate generates UUID if not set
func (w *Webhook) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name
func (Webhook) TableName() string {
	return "webhooks"
}

