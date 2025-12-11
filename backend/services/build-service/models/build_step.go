package models

import (
	"time"

	"github.com/google/uuid"
)

// StepStatus represents the state of a build step
type StepStatus string

const (
	StepStatusPending  StepStatus = "pending"
	StepStatusRunning  StepStatus = "running"
	StepStatusSuccess  StepStatus = "success"
	StepStatusFailed   StepStatus = "failed"
	StepStatusSkipped  StepStatus = "skipped"
)

// BuildStep represents a step within a build (SRS B.3)
type BuildStep struct {
	ID         uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	BuildID    uuid.UUID  `gorm:"type:uuid;not null;index"`
	StepName   string     `gorm:"type:varchar(100);not null"`
	Status     StepStatus `gorm:"type:varchar(50);not null;default:pending"`
	DurationMs *int       `gorm:"type:integer"`
	CreatedAt  time.Time  `gorm:"not null;default:now()"`
	UpdatedAt  time.Time  `gorm:"not null;default:now()"`
}

// TableName specifies the table name for BuildStep
func (BuildStep) TableName() string {
	return "build_steps"
}

// Common build step names
const (
	StepClone    = "clone"
	StepInstall  = "install"
	StepBuild    = "build"
	StepTest     = "test"
	StepDockerBuild = "docker_build"
	StepDockerPush  = "docker_push"
	StepDeploy   = "deploy"
)

