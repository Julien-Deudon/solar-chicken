package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID                uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	Email             string     `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash      string     `gorm:"not null" json:"-"`
	FirstName         string     `json:"firstName"`
	LastName          string     `json:"lastName"`
	EmailVerified     bool       `gorm:"default:false" json:"emailVerified"`
	VerificationToken *string    `gorm:"index" json:"-"`
	VerifiedAt        *time.Time `json:"verifiedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`

	// Relations
	Coops                []Coop                `gorm:"foreignKey:UserID" json:"coops,omitempty"`
	NotificationSettings *NotificationSettings `gorm:"foreignKey:UserID" json:"notificationSettings,omitempty"`
}

// BeforeCreate hook to generate UUID
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
