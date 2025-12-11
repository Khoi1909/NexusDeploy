package models

import "time"

// User theo SRS: users bảng trong auth_db
type User struct {
	ID                   string    `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	GithubID             int64     `gorm:"uniqueIndex;not null"`
	Username             string    `gorm:"size:255;not null"`
	Email                string    `gorm:"size:255;uniqueIndex;not null"`
	AvatarURL            string    `gorm:"type:text"`
	Plan                 string    `gorm:"size:50;not null;default:standard"`
	GithubTokenEncrypted string    `gorm:"type:text"`
	CreatedAt            time.Time `gorm:"not null;default:now()"`
	UpdatedAt            time.Time `gorm:"not null;default:now()"`
}
