// Package seed creates a default admin user on first boot when the users
// table is empty.
package seed

import (
"context"
"crypto/rand"
"encoding/hex"
"fmt"
"log/slog"

"github.com/d9705996/autopsy/internal/model"
"golang.org/x/crypto/bcrypt"
"gorm.io/gorm"
)

// AdminOptions configures the seed admin user.
type AdminOptions struct {
Email    string
Password string // if empty, a random password is generated
}

// EnsureAdmin creates a seed admin user if no users exist.
// It prints the generated password to stdout and returns it.
// If a password was supplied in opts it is used directly.
// The function is idempotent — it is safe to call on every startup.
func EnsureAdmin(_ context.Context, db *gorm.DB, opts AdminOptions, log *slog.Logger) error {
var count int64
if err := db.Model(&model.User{}).Count(&count).Error; err != nil {
return fmt.Errorf("count users: %w", err)
}
if count > 0 {
log.Info("seed admin already exists")
return nil
}

password := opts.Password
if password == "" {
var err error
password, err = generatePassword()
if err != nil {
return fmt.Errorf("generate seed password: %w", err)
}
// Print the generated password to stdout exactly once.
fmt.Printf("[autopsy] seed admin password: %s\n", password)
}

hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
if err != nil {
return fmt.Errorf("hash seed password: %w", err)
}

u := &model.User{
Email:        opts.Email,
Name:         "Seed Admin",
PasswordHash: string(hash),
Roles:        model.StringSlice{"Admin"},
}
if err := db.Create(u).Error; err != nil {
return fmt.Errorf("insert seed admin: %w", err)
}

log.Info("seed admin created", "email", opts.Email)
return nil
}

func generatePassword() (string, error) {
b := make([]byte, 16)
if _, err := rand.Read(b); err != nil {
return "", err
}
return hex.EncodeToString(b), nil
}
