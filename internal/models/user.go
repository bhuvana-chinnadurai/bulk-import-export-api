package models

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID        string    `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Name      string    `json:"name" db:"name"`
	Role      string    `json:"role" db:"role"`
	Active    bool      `json:"active" db:"active"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// ValidRoles defines allowed user roles
var ValidRoles = map[string]bool{
	"admin":  true,
	"editor": true,
	"viewer": true,
}

// UserCSV represents a user record from CSV import
type UserCSV struct {
	ID        string `csv:"id"`
	Email     string `csv:"email"`
	Name      string `csv:"name"`
	Role      string `csv:"role"`
	Active    string `csv:"active"` // CSV uses string "true"/"false"
	CreatedAt string `csv:"created_at"`
	UpdatedAt string `csv:"updated_at"`
}
