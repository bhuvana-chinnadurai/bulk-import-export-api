package validation

import (
	"strings"
	"testing"

	"github.com/bulk-import-export-api/internal/models"
)

func TestValidateUser(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name       string
		user       *models.UserCSV
		wantErrors int
		wantFields []string
	}{
		{
			name: "valid user with all fields",
			user: &models.UserCSV{
				ID:        "550e8400-e29b-41d4-a716-446655440000",
				Email:     "test@example.com",
				Name:      "Test User",
				Role:      "admin",
				Active:    "true",
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 0,
		},
		{
			name: "missing id - required field",
			user: &models.UserCSV{
				Email:     "test@example.com",
				Name:      "Test User",
				Role:      "admin",
				Active:    "true",
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 1,
			wantFields: []string{"id"},
		},
		{
			name: "invalid email format",
			user: &models.UserCSV{
				ID:        "550e8400-e29b-41d4-a716-446655440000",
				Email:     "not-an-email",
				Name:      "Test User",
				Role:      "admin",
				Active:    "true",
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 1,
			wantFields: []string{"email"},
		},
		{
			name: "invalid role - not in allowed values",
			user: &models.UserCSV{
				ID:        "550e8400-e29b-41d4-a716-446655440000",
				Email:     "test@example.com",
				Name:      "Test User",
				Role:      "superadmin",
				Active:    "true",
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 1,
			wantFields: []string{"role"},
		},
		{
			name: "invalid active boolean",
			user: &models.UserCSV{
				ID:        "550e8400-e29b-41d4-a716-446655440000",
				Email:     "test@example.com",
				Name:      "Test User",
				Role:      "admin",
				Active:    "yes",
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 1,
			wantFields: []string{"active"},
		},
		{
			name: "invalid date format",
			user: &models.UserCSV{
				ID:        "550e8400-e29b-41d4-a716-446655440000",
				Email:     "test@example.com",
				Name:      "Test User",
				Role:      "admin",
				Active:    "true",
				CreatedAt: "01/01/2024",
			},
			wantErrors: 1,
			wantFields: []string{"created_at"},
		},
		{
			name: "multiple validation errors",
			user: &models.UserCSV{
				ID:        "", // missing
				Email:     "invalid",
				Name:      "", // missing
				Role:      "unknown",
				Active:    "maybe",
				CreatedAt: "invalid-date",
			},
			wantErrors: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateUser(tt.user, 1)
			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateUser() got %d errors, want %d. Errors: %v", len(errors), tt.wantErrors, errors)
			}

			// Check specific fields if provided
			if tt.wantFields != nil {
				for _, wantField := range tt.wantFields {
					found := false
					for _, err := range errors {
						if err.Field == wantField {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error for field '%s' but not found", wantField)
					}
				}
			}
		})
	}
}

func TestValidateArticle(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name       string
		article    *models.ArticleNDJSON
		wantErrors int
		wantFields []string
	}{
		{
			name: "valid published article",
			article: &models.ArticleNDJSON{
				ID:          "550e8400-e29b-41d4-a716-446655440000",
				Slug:        "my-first-article",
				Title:       "My First Article",
				Body:        "This is the body content",
				AuthorID:    "550e8400-e29b-41d4-a716-446655440001",
				Tags:        []string{"tech", "go"},
				Status:      "published",
				PublishedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 0,
		},
		{
			name: "valid draft article",
			article: &models.ArticleNDJSON{
				ID:       "550e8400-e29b-41d4-a716-446655440000",
				Slug:     "my-draft-article",
				Title:    "My Draft Article",
				Body:     "This is a draft",
				AuthorID: "550e8400-e29b-41d4-a716-446655440001",
				Status:   "draft",
			},
			wantErrors: 0,
		},
		{
			name: "invalid slug - not kebab-case",
			article: &models.ArticleNDJSON{
				ID:       "550e8400-e29b-41d4-a716-446655440000",
				Slug:     "My_First_Article",
				Title:    "My First Article",
				Body:     "This is the body",
				AuthorID: "550e8400-e29b-41d4-a716-446655440001",
				Status:   "draft",
			},
			wantErrors: 1,
			wantFields: []string{"slug"},
		},
		{
			name: "draft with published_at - logical error",
			article: &models.ArticleNDJSON{
				ID:          "550e8400-e29b-41d4-a716-446655440000",
				Slug:        "my-first-article",
				Title:       "My First Article",
				Body:        "This is the body",
				AuthorID:    "550e8400-e29b-41d4-a716-446655440001",
				Status:      "draft",
				PublishedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 1,
			wantFields: []string{"published_at"},
		},
		{
			name: "invalid status",
			article: &models.ArticleNDJSON{
				ID:       "550e8400-e29b-41d4-a716-446655440000",
				Slug:     "my-first-article",
				Title:    "My First Article",
				Body:     "This is the body",
				AuthorID: "550e8400-e29b-41d4-a716-446655440001",
				Status:   "archived",
			},
			wantErrors: 1,
			wantFields: []string{"status"},
		},
		{
			name: "missing required fields",
			article: &models.ArticleNDJSON{
				ID:     "",
				Slug:   "",
				Title:  "",
				Body:   "",
				Status: "draft",
			},
			wantErrors: 5, // id, slug, title, body, author_id
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateArticle(tt.article, 1)
			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateArticle() got %d errors, want %d. Errors: %v", len(errors), tt.wantErrors, errors)
			}

			if tt.wantFields != nil {
				for _, wantField := range tt.wantFields {
					found := false
					for _, err := range errors {
						if err.Field == wantField {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error for field '%s' but not found", wantField)
					}
				}
			}
		})
	}
}

func TestValidateComment(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name       string
		comment    *models.CommentNDJSON
		wantErrors int
		wantFields []string
	}{
		{
			name: "valid comment",
			comment: &models.CommentNDJSON{
				ID:        "cm_550e8400-e29b-41d4-a716-446655440000",
				ArticleID: "550e8400-e29b-41d4-a716-446655440001",
				UserID:    "550e8400-e29b-41d4-a716-446655440002",
				Body:      "This is a valid comment",
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 0,
		},
		{
			name: "missing body",
			comment: &models.CommentNDJSON{
				ID:        "cm_550e8400-e29b-41d4-a716-446655440000",
				ArticleID: "550e8400-e29b-41d4-a716-446655440001",
				UserID:    "550e8400-e29b-41d4-a716-446655440002",
				Body:      "",
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 1,
			wantFields: []string{"body"},
		},
		{
			name: "invalid article_id format",
			comment: &models.CommentNDJSON{
				ID:        "cm_550e8400-e29b-41d4-a716-446655440000",
				ArticleID: "not-a-uuid",
				UserID:    "550e8400-e29b-41d4-a716-446655440002",
				Body:      "This is a comment",
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 1,
			wantFields: []string{"article_id"},
		},
		{
			name: "body exceeds word limit",
			comment: &models.CommentNDJSON{
				ID:        "cm_550e8400-e29b-41d4-a716-446655440000",
				ArticleID: "550e8400-e29b-41d4-a716-446655440001",
				UserID:    "550e8400-e29b-41d4-a716-446655440002",
				Body:      strings.Repeat("word ", 600), // 600 words
				CreatedAt: "2024-01-01T00:00:00Z",
			},
			wantErrors: 1,
			wantFields: []string{"body"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := validator.ValidateComment(tt.comment, 1)
			if len(errors) != tt.wantErrors {
				t.Errorf("ValidateComment() got %d errors, want %d. Errors: %v", len(errors), tt.wantErrors, errors)
			}

			if tt.wantFields != nil {
				for _, wantField := range tt.wantFields {
					found := false
					for _, err := range errors {
						if err.Field == wantField {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Expected error for field '%s' but not found", wantField)
					}
				}
			}
		})
	}
}

func TestDuplicateEmailDetection(t *testing.T) {
	validator := NewValidator()

	user1 := &models.UserCSV{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Email:     "duplicate@example.com",
		Name:      "User One",
		Role:      "admin",
		Active:    "true",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	// First user should be valid
	errors := validator.ValidateUser(user1, 1)
	if len(errors) != 0 {
		t.Errorf("First user should be valid, got %d errors: %v", len(errors), errors)
	}

	// Add email to cache (simulating successful insert)
	validator.AddUserEmail(user1.Email)

	// Second user with same email should fail
	user2 := &models.UserCSV{
		ID:        "550e8400-e29b-41d4-a716-446655440001",
		Email:     "duplicate@example.com",
		Name:      "User Two",
		Role:      "editor",
		Active:    "true",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	errors = validator.ValidateUser(user2, 2)
	if len(errors) != 1 {
		t.Errorf("Second user should have 1 error, got %d: %v", len(errors), errors)
	}
	if len(errors) > 0 && errors[0].Message != "duplicate email" {
		t.Errorf("Expected 'duplicate email' error, got '%s'", errors[0].Message)
	}
}

func TestDuplicateSlugDetection(t *testing.T) {
	validator := NewValidator()

	article1 := &models.ArticleNDJSON{
		ID:       "550e8400-e29b-41d4-a716-446655440000",
		Slug:     "duplicate-slug",
		Title:    "Article One",
		Body:     "Body content",
		AuthorID: "550e8400-e29b-41d4-a716-446655440001",
		Status:   "draft",
	}

	// First article should be valid
	errors := validator.ValidateArticle(article1, 1)
	if len(errors) != 0 {
		t.Errorf("First article should be valid, got %d errors: %v", len(errors), errors)
	}

	// Add slug to cache
	validator.AddArticleSlug(article1.Slug)

	// Second article with same slug should fail
	article2 := &models.ArticleNDJSON{
		ID:       "550e8400-e29b-41d4-a716-446655440002",
		Slug:     "duplicate-slug",
		Title:    "Article Two",
		Body:     "Other body",
		AuthorID: "550e8400-e29b-41d4-a716-446655440001",
		Status:   "draft",
	}

	errors = validator.ValidateArticle(article2, 2)
	if len(errors) != 1 {
		t.Errorf("Second article should have 1 error, got %d: %v", len(errors), errors)
	}
	if len(errors) > 0 && errors[0].Message != "duplicate slug" {
		t.Errorf("Expected 'duplicate slug' error, got '%s'", errors[0].Message)
	}
}

func TestValidationErrorMessages(t *testing.T) {
	validator := NewValidator()

	// Test that error messages are clear and actionable
	user := &models.UserCSV{
		ID:        "",
		Email:     "invalid",
		Name:      "",
		Role:      "superuser",
		Active:    "maybe",
		CreatedAt: "not-a-date",
	}

	errors := validator.ValidateUser(user, 1)

	// Verify each error has a clear message
	for _, err := range errors {
		if err.Field == "" {
			t.Error("Error should have a field name")
		}
		if err.Message == "" {
			t.Error("Error should have a message")
		}
		// ValidationError struct in this package doesn't have Line field
		// Line number is tracked separately in the import service
	}
}

func TestKebabCaseValidation(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		slug  string
		valid bool
	}{
		{"valid-slug", true},
		{"another-valid-slug", true},
		{"a", true},
		{"a-b-c", true},
		{"Invalid-Slug", false},
		{"invalid_slug", false},
		{"invalid slug", false},
		{"123-numbers", true},
		{"slug-123", true},
		{"-starts-with-dash", false},
		{"ends-with-dash-", false},
		{"double--dash", false},
	}

	for _, tt := range tests {
		t.Run(tt.slug, func(t *testing.T) {
			article := &models.ArticleNDJSON{
				ID:       "550e8400-e29b-41d4-a716-446655440000",
				Slug:     tt.slug,
				Title:    "Test",
				Body:     "Test body",
				AuthorID: "550e8400-e29b-41d4-a716-446655440001",
				Status:   "draft",
			}
			errors := validator.ValidateArticle(article, 1)
			hasSlugError := false
			for _, err := range errors {
				if err.Field == "slug" {
					hasSlugError = true
					break
				}
			}
			if tt.valid && hasSlugError {
				t.Errorf("Slug '%s' should be valid", tt.slug)
			}
			if !tt.valid && !hasSlugError {
				t.Errorf("Slug '%s' should be invalid", tt.slug)
			}
		})
	}
}

func TestCommentBodyWordBoundary(t *testing.T) {
	validator := NewValidator()

	// Exactly 500 words - should pass
	words500 := strings.Repeat("word ", 500)
	comment500 := &models.CommentNDJSON{
		ID:        "cm_550e8400-e29b-41d4-a716-446655440000",
		ArticleID: "550e8400-e29b-41d4-a716-446655440001",
		UserID:    "550e8400-e29b-41d4-a716-446655440002",
		Body:      strings.TrimSpace(words500),
		CreatedAt: "2024-01-01T00:00:00Z",
	}
	errors := validator.ValidateComment(comment500, 1)
	for _, e := range errors {
		if e.Field == "body" {
			t.Errorf("500 words should be valid, but got body error: %s", e.Message)
		}
	}

	// 501 words - should fail
	words501 := strings.Repeat("word ", 501)
	comment501 := &models.CommentNDJSON{
		ID:        "cm_550e8400-e29b-41d4-a716-446655440003",
		ArticleID: "550e8400-e29b-41d4-a716-446655440001",
		UserID:    "550e8400-e29b-41d4-a716-446655440002",
		Body:      strings.TrimSpace(words501),
		CreatedAt: "2024-01-01T00:00:00Z",
	}
	errors = validator.ValidateComment(comment501, 2)
	hasBodyError := false
	for _, e := range errors {
		if e.Field == "body" {
			hasBodyError = true
		}
	}
	if !hasBodyError {
		t.Error("501 words should fail validation, but no body error was returned")
	}
}

func TestValidateUser_UnicodeNames(t *testing.T) {
	validator := NewValidator()

	user := &models.UserCSV{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Email:     "unicode@example.com",
		Name:      "Ünïcödé Üser 日本語",
		Role:      "admin",
		Active:    "true",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	errors := validator.ValidateUser(user, 1)
	if len(errors) != 0 {
		t.Errorf("Unicode names should be valid, got errors: %v", errors)
	}
}

func TestValidateUser_EmptyActiveField(t *testing.T) {
	validator := NewValidator()

	// Empty active field should not produce an error (it's optional)
	user := &models.UserCSV{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Email:     "test@example.com",
		Name:      "Test User",
		Role:      "admin",
		Active:    "",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	errors := validator.ValidateUser(user, 1)
	for _, e := range errors {
		if e.Field == "active" {
			t.Errorf("Empty active should be allowed, got error: %s", e.Message)
		}
	}
}

func TestValidateArticle_ForeignKeyValidation(t *testing.T) {
	validator := NewValidator()

	// Set up a user ID cache with known IDs
	validator.SetUserIDCache([]string{
		"550e8400-e29b-41d4-a716-446655440001",
		"550e8400-e29b-41d4-a716-446655440002",
	})

	// Valid author_id that exists in cache
	article := &models.ArticleNDJSON{
		ID:       "550e8400-e29b-41d4-a716-446655440000",
		Slug:     "test-article",
		Title:    "Test",
		Body:     "Test body",
		AuthorID: "550e8400-e29b-41d4-a716-446655440001",
		Status:   "draft",
	}
	errors := validator.ValidateArticle(article, 1)
	if len(errors) != 0 {
		t.Errorf("Article with valid author_id should pass, got: %v", errors)
	}

	// Invalid author_id (valid UUID but not in cache)
	article2 := &models.ArticleNDJSON{
		ID:       "550e8400-e29b-41d4-a716-446655440010",
		Slug:     "test-article-2",
		Title:    "Test 2",
		Body:     "Test body 2",
		AuthorID: "550e8400-e29b-41d4-a716-446655440099",
		Status:   "draft",
	}
	errors = validator.ValidateArticle(article2, 2)
	hasAuthorError := false
	for _, e := range errors {
		if e.Field == "author_id" && e.Message == "referenced user does not exist" {
			hasAuthorError = true
		}
	}
	if !hasAuthorError {
		t.Error("Article with non-existent author_id should fail FK validation")
	}
}

func BenchmarkValidateComment(b *testing.B) {
	validator := NewValidator()
	comment := &models.CommentNDJSON{
		ID:        "cm_550e8400-e29b-41d4-a716-446655440000",
		ArticleID: "550e8400-e29b-41d4-a716-446655440001",
		UserID:    "550e8400-e29b-41d4-a716-446655440002",
		Body:      "This is a benchmark test comment body",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateComment(comment, i)
	}
}

func BenchmarkValidateUser(b *testing.B) {
	validator := NewValidator()
	user := &models.UserCSV{
		ID:        "550e8400-e29b-41d4-a716-446655440000",
		Email:     "test@example.com",
		Name:      "Test User",
		Role:      "admin",
		Active:    "true",
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateUser(user, i)
	}
}

func BenchmarkValidateArticle(b *testing.B) {
	validator := NewValidator()
	article := &models.ArticleNDJSON{
		ID:       "550e8400-e29b-41d4-a716-446655440000",
		Slug:     "test-article",
		Title:    "Test Article",
		Body:     "This is a test body",
		AuthorID: "550e8400-e29b-41d4-a716-446655440001",
		Tags:     []string{"test", "benchmark"},
		Status:   "published",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		validator.ValidateArticle(article, i)
	}
}
