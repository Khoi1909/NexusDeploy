package models

import (
	"time"

	"github.com/google/uuid"
)

// BuildStatus represents the state of a build in the CI/CD pipeline
type BuildStatus string

const (
	BuildStatusPending       BuildStatus = "pending"
	BuildStatusRunning       BuildStatus = "running"
	BuildStatusFailed        BuildStatus = "failed"
	BuildStatusBuildingImage BuildStatus = "building_image"
	BuildStatusPushingImage  BuildStatus = "pushing_image"
	BuildStatusDeploying     BuildStatus = "deploying"
	BuildStatusSuccess       BuildStatus = "success"
	BuildStatusDeployFailed  BuildStatus = "deploy_failed"
)

// Build represents a CI/CD build job (SRS B.3)
type Build struct {
	ID         uuid.UUID   `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProjectID  uuid.UUID   `gorm:"type:uuid;not null;index"`
	CommitSHA  string      `gorm:"type:varchar(40)"`
	Status     BuildStatus `gorm:"type:varchar(50);not null;default:pending"`
	ImageTag   string      `gorm:"type:varchar(255)"` // Docker image tag được tạo bởi Runner
	StartedAt  *time.Time  `gorm:"type:timestamptz"`
	FinishedAt *time.Time  `gorm:"type:timestamptz"`
	CreatedAt  time.Time   `gorm:"not null;default:now()"`
	UpdatedAt  time.Time   `gorm:"not null;default:now()"`

	// Associations
	Logs  []BuildLog  `gorm:"foreignKey:BuildID;constraint:OnDelete:CASCADE"`
	Steps []BuildStep `gorm:"foreignKey:BuildID;constraint:OnDelete:CASCADE"`
}

// TableName specifies the table name for Build
func (Build) TableName() string {
	return "builds"
}

// IsTerminal returns true if the build is in a terminal state
func (b *Build) IsTerminal() bool {
	switch b.Status {
	case BuildStatusFailed, BuildStatusSuccess, BuildStatusDeployFailed:
		return true
	default:
		return false
	}
}

// CanTransitionTo checks if a status transition is valid
func (b *Build) CanTransitionTo(newStatus BuildStatus) bool {
	transitions := map[BuildStatus][]BuildStatus{
		BuildStatusPending:       {BuildStatusRunning},
		BuildStatusRunning:       {BuildStatusFailed, BuildStatusBuildingImage},
		BuildStatusBuildingImage: {BuildStatusFailed, BuildStatusPushingImage},
		BuildStatusPushingImage:  {BuildStatusFailed, BuildStatusSuccess, BuildStatusDeploying},
		BuildStatusDeploying:     {BuildStatusSuccess, BuildStatusDeployFailed},
	}

	allowed, ok := transitions[b.Status]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == newStatus {
			return true
		}
	}
	return false
}
