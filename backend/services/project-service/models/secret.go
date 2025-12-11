package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Secret represents an encrypted environment variable for a project
type Secret struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ProjectID      uuid.UUID `gorm:"type:uuid;not null;index"`
	Name           string    `gorm:"type:varchar(255);not null"`
	EncryptedValue []byte    `gorm:"type:bytea;not null"` // AES-256-GCM encrypted
	CreatedAt      time.Time `gorm:"not null;default:now()"`
	UpdatedAt      time.Time `gorm:"not null;default:now()"`
}

// BeforeCreate generates UUID if not set
func (s *Secret) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name
func (Secret) TableName() string {
	return "secrets"
}

