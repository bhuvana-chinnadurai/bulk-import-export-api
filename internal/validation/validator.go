package validation

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bulk-import-export-api/internal/models"
	"github.com/google/uuid"
)

var (
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	slugRegex  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
)

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string      `json:"field"`
	Message string      `json:"message"`
	Value   interface{} `json:"value,omitempty"`
}

// Validator provides validation methods
type Validator struct {
	userEmailCache   map[string]bool
	articleSlugCache map[string]bool
	userIDCache      map[string]bool
	articleIDCache   map[string]bool
}

// NewValidator creates a new validator instance
func NewValidator() *Validator {
	return &Validator{
		userEmailCache:   make(map[string]bool),
		articleSlugCache: make(map[string]bool),
		userIDCache:      make(map[string]bool),
		articleIDCache:   make(map[string]bool),
	}
}

// SetUserIDCache sets the cache of existing user IDs for FK validation
func (v *Validator) SetUserIDCache(ids []string) {
	for _, id := range ids {
		v.userIDCache[id] = true
	}
}

// SetArticleIDCache sets the cache of existing article IDs for FK validation
func (v *Validator) SetArticleIDCache(ids []string) {
	for _, id := range ids {
		v.articleIDCache[id] = true
	}
}

// AddUserEmail adds an email to the uniqueness cache
func (v *Validator) AddUserEmail(email string) {
	v.userEmailCache[strings.ToLower(email)] = true
}

// AddArticleSlug adds a slug to the uniqueness cache
func (v *Validator) AddArticleSlug(slug string) {
	v.articleSlugCache[slug] = true
}

// AddUserID adds a user ID to the cache for FK validation
func (v *Validator) AddUserID(id string) {
	v.userIDCache[id] = true
}

// AddArticleID adds an article ID to the cache for FK validation
func (v *Validator) AddArticleID(id string) {
	v.articleIDCache[id] = true
}

// ValidateUser validates a user record
func (v *Validator) ValidateUser(user *models.UserCSV, lineNum int) []ValidationError {
	var errors []ValidationError

	// Validate ID
	if user.ID == "" {
		errors = append(errors, ValidationError{Field: "id", Message: "id is required"})
	} else if !isValidUUID(user.ID) {
		errors = append(errors, ValidationError{Field: "id", Message: "invalid UUID format", Value: user.ID})
	}

	// Validate email
	if user.Email == "" {
		errors = append(errors, ValidationError{Field: "email", Message: "email is required"})
	} else if !emailRegex.MatchString(user.Email) {
		errors = append(errors, ValidationError{Field: "email", Message: "invalid email format", Value: user.Email})
	} else {
		// Check for duplicate email in current batch
		emailLower := strings.ToLower(user.Email)
		if v.userEmailCache[emailLower] {
			errors = append(errors, ValidationError{Field: "email", Message: "duplicate email", Value: user.Email})
		}
	}

	// Validate name
	if user.Name == "" {
		errors = append(errors, ValidationError{Field: "name", Message: "name is required"})
	}

	// Validate role
	if user.Role == "" {
		errors = append(errors, ValidationError{Field: "role", Message: "role is required"})
	} else if !models.ValidRoles[user.Role] {
		errors = append(errors, ValidationError{
			Field:   "role",
			Message: fmt.Sprintf("invalid role, must be one of: admin, editor, viewer"),
			Value:   user.Role,
		})
	}

	// Validate active (boolean)
	if user.Active != "" && user.Active != "true" && user.Active != "false" {
		errors = append(errors, ValidationError{Field: "active", Message: "active must be 'true' or 'false'", Value: user.Active})
	}

	// Validate created_at
	if user.CreatedAt == "" {
		errors = append(errors, ValidationError{Field: "created_at", Message: "created_at is required"})
	} else if _, err := time.Parse(time.RFC3339, user.CreatedAt); err != nil {
		errors = append(errors, ValidationError{Field: "created_at", Message: "invalid ISO 8601 date format", Value: user.CreatedAt})
	}

	return errors
}

// ValidateArticle validates an article record
func (v *Validator) ValidateArticle(article *models.ArticleNDJSON, lineNum int) []ValidationError {
	var errors []ValidationError

	// Validate ID
	if article.ID == "" {
		errors = append(errors, ValidationError{Field: "id", Message: "id is required"})
	} else if !isValidUUID(article.ID) {
		errors = append(errors, ValidationError{Field: "id", Message: "invalid UUID format", Value: article.ID})
	}

	// Validate slug
	if article.Slug == "" {
		errors = append(errors, ValidationError{Field: "slug", Message: "slug is required"})
	} else if !slugRegex.MatchString(article.Slug) {
		errors = append(errors, ValidationError{Field: "slug", Message: "slug must be kebab-case (lowercase letters, numbers, hyphens)", Value: article.Slug})
	} else {
		// Check for duplicate slug in current batch
		if v.articleSlugCache[article.Slug] {
			errors = append(errors, ValidationError{Field: "slug", Message: "duplicate slug", Value: article.Slug})
		}
	}

	// Validate title
	if article.Title == "" {
		errors = append(errors, ValidationError{Field: "title", Message: "title is required"})
	}

	// Validate body
	if article.Body == "" {
		errors = append(errors, ValidationError{Field: "body", Message: "body is required"})
	}

	// Validate author_id (FK)
	if article.AuthorID == "" {
		errors = append(errors, ValidationError{Field: "author_id", Message: "author_id is required"})
	} else if !isValidUUID(article.AuthorID) {
		errors = append(errors, ValidationError{Field: "author_id", Message: "invalid UUID format", Value: article.AuthorID})
	} else if len(v.userIDCache) > 0 && !v.userIDCache[article.AuthorID] {
		errors = append(errors, ValidationError{Field: "author_id", Message: "referenced user does not exist", Value: article.AuthorID})
	}

	// Validate status
	if article.Status != "" && !models.ValidStatuses[article.Status] {
		errors = append(errors, ValidationError{
			Field:   "status",
			Message: "invalid status, must be one of: draft, published",
			Value:   article.Status,
		})
	}

	// Validate draft must not have published_at
	if article.Status == "draft" && article.PublishedAt != "" {
		errors = append(errors, ValidationError{Field: "published_at", Message: "draft articles must not have published_at"})
	}

	// Validate published_at format if present
	if article.PublishedAt != "" {
		if _, err := time.Parse(time.RFC3339, article.PublishedAt); err != nil {
			errors = append(errors, ValidationError{Field: "published_at", Message: "invalid ISO 8601 date format", Value: article.PublishedAt})
		}
	}

	return errors
}

// ValidateComment validates a comment record
func (v *Validator) ValidateComment(comment *models.CommentNDJSON, lineNum int) []ValidationError {
	var errors []ValidationError

	// Validate ID
	if comment.ID == "" {
		errors = append(errors, ValidationError{Field: "id", Message: "id is required"})
	} else if !isValidUUID(comment.ID) && !strings.HasPrefix(comment.ID, "cm_") {
		errors = append(errors, ValidationError{Field: "id", Message: "invalid ID format", Value: comment.ID})
	}

	// Validate article_id (FK)
	if comment.ArticleID == "" {
		errors = append(errors, ValidationError{Field: "article_id", Message: "article_id is required"})
	} else if !isValidUUID(comment.ArticleID) {
		errors = append(errors, ValidationError{Field: "article_id", Message: "invalid UUID format", Value: comment.ArticleID})
	} else if len(v.articleIDCache) > 0 && !v.articleIDCache[comment.ArticleID] {
		errors = append(errors, ValidationError{Field: "article_id", Message: "referenced article does not exist", Value: comment.ArticleID})
	}

	// Validate user_id (FK)
	if comment.UserID == "" {
		errors = append(errors, ValidationError{Field: "user_id", Message: "user_id is required"})
	} else if !isValidUUID(comment.UserID) {
		errors = append(errors, ValidationError{Field: "user_id", Message: "invalid UUID format", Value: comment.UserID})
	} else if len(v.userIDCache) > 0 && !v.userIDCache[comment.UserID] {
		errors = append(errors, ValidationError{Field: "user_id", Message: "referenced user does not exist", Value: comment.UserID})
	}

	// Validate body
	if comment.Body == "" {
		errors = append(errors, ValidationError{Field: "body", Message: "body is required"})
	} else {
		// Check word count (max 500 words)
		wordCount := len(strings.Fields(comment.Body))
		if wordCount > models.MaxCommentWords {
			errors = append(errors, ValidationError{
				Field:   "body",
				Message: fmt.Sprintf("body exceeds maximum of %d words (has %d)", models.MaxCommentWords, wordCount),
			})
		}
	}

	// Validate created_at
	if comment.CreatedAt == "" {
		errors = append(errors, ValidationError{Field: "created_at", Message: "created_at is required"})
	} else if _, err := time.Parse(time.RFC3339, comment.CreatedAt); err != nil {
		errors = append(errors, ValidationError{Field: "created_at", Message: "invalid ISO 8601 date format", Value: comment.CreatedAt})
	}

	return errors
}

// isValidUUID checks if a string is a valid UUID
func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}
