package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Project represents a deployment project
type Project struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index"`
	Name         string    `gorm:"type:varchar(255);not null"`
	RepoURL      string    `gorm:"type:text;not null"`
	Branch       string    `gorm:"type:varchar(255);not null;default:'main'"`
	Preset       string    `gorm:"type:varchar(50);not null"` // nodejs, go, python, docker, static
	BuildCommand string    `gorm:"type:text"`
	StartCommand string    `gorm:"type:text"`
	Port         int       `gorm:"not null;default:8080"`
	GithubRepoID int64     `gorm:"not null"`
	IsPrivate    bool      `gorm:"default:false"`
	CreatedAt    time.Time `gorm:"not null;default:now()"`
	UpdatedAt    time.Time `gorm:"not null;default:now()"`

	// Relations
	Secrets  []Secret  `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE"`
	Webhooks []Webhook `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE"`
}

// BeforeCreate generates UUID if not set
func (p *Project) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// TableName returns the table name
func (Project) TableName() string {
	return "projects"
}

