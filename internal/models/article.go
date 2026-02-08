package models

import (
	"time"
)

// Article represents an article in the system
type Article struct {
	ID          string     `json:"id" db:"id"`
	Slug        string     `json:"slug" db:"slug"`
	Title       string     `json:"title" db:"title"`
	Body        string     `json:"body" db:"body"`
	AuthorID    string     `json:"author_id" db:"author_id"`
	Tags        []string   `json:"tags" db:"-"` // Stored as JSON string in DB
	TagsJSON    string     `json:"-" db:"tags"` // For DB storage
	Status      string     `json:"status" db:"status"`
	PublishedAt *time.Time `json:"published_at,omitempty" db:"published_at"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// ValidStatuses defines allowed article statuses
var ValidStatuses = map[string]bool{
	"draft":     true,
	"published": true,
}

// ArticleNDJSON represents an article record from NDJSON import
type ArticleNDJSON struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	AuthorID    string   `json:"author_id"`
	Tags        []string `json:"tags"`
	Status      string   `json:"status"`
	PublishedAt string   `json:"published_at,omitempty"`
}
