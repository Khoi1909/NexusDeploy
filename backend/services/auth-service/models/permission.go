package models

// Permission theo SRS
type Permission struct {
	ID       string `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID   string `gorm:"type:uuid;not null;index"`
	Resource string `gorm:"size:100;not null"`
	Action   string `gorm:"size:50;not null"`
}
