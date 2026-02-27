// Package model contains GORM model definitions shared across packages.
// All models are driver-agnostic: they work with both PostgreSQL and SQLite.
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Organization represents a tenant in the multi-tenancy schema.
type Organization struct {
	ID        string    `gorm:"type:text;primaryKey"`
	Name      string    `gorm:"type:text;not null"`
	Slug      string    `gorm:"type:text;not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"not null"`
}

// BeforeCreate generates a UUID primary key if not set.
func (o *Organization) BeforeCreate(_ *gorm.DB) error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}

// StringSlice is a []string that GORM serialises as JSON for both SQLite
// (TEXT column) and PostgreSQL (TEXT column after migration 0004).
type StringSlice []string

// User is the GORM model for the users table.
type User struct {
	ID                   string      `gorm:"type:text;primaryKey"`
	OrganizationID       *string     `gorm:"type:text"`
	Email                string      `gorm:"type:text;not null;uniqueIndex"`
	Name                 string      `gorm:"type:text;not null;default:''"`
	PasswordHash         string      `gorm:"type:text;not null;default:''"`
	Roles                StringSlice `gorm:"type:text;not null;default:'[]';serializer:json"`
	NotificationChannels string      `gorm:"type:text;not null;default:'[]'"`
	OIDCSub              *string     `gorm:"type:text"`
	DeactivatedAt        *time.Time
	CreatedAt            time.Time `gorm:"not null"`
	UpdatedAt            time.Time `gorm:"not null"`
}

// BeforeCreate generates a UUID primary key if not set.
func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// RefreshToken is the GORM model for the refresh_tokens table.
type RefreshToken struct {
	ID        string    `gorm:"type:text;primaryKey"`
	UserID    string    `gorm:"type:text;not null;index"`
	TokenHash string    `gorm:"type:text;not null;uniqueIndex"`
	ExpiresAt time.Time `gorm:"not null"`
	RevokedAt *time.Time
	CreatedAt time.Time `gorm:"not null"`
}

// BeforeCreate generates a UUID primary key if not set.
func (rt *RefreshToken) BeforeCreate(_ *gorm.DB) error {
	if rt.ID == "" {
		rt.ID = uuid.New().String()
	}
	return nil
}
