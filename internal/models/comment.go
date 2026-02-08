package models

import (
	"time"
)

// Comment represents a comment on an article
type Comment struct {
	ID        string    `json:"id" db:"id"`
	ArticleID string    `json:"article_id" db:"article_id"`
	UserID    string    `json:"user_id" db:"user_id"`
	Body      string    `json:"body" db:"body"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CommentNDJSON represents a comment record from NDJSON import
type CommentNDJSON struct {
	ID        string `json:"id"`
	ArticleID string `json:"article_id"`
	UserID    string `json:"user_id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

// MaxCommentWords is the maximum allowed words in a comment body
const MaxCommentWords = 500
