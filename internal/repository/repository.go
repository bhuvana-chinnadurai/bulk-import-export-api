package repository

import (
	"context"

	"github.com/bulk-import-export-api/internal/database"
	"github.com/bulk-import-export-api/internal/models"
)

// UserRepository defines the interface for user data operations
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	Upsert(ctx context.Context, user *models.User) error
	BatchInsert(ctx context.Context, users []*models.User) (int, error)
	GetByID(ctx context.Context, id string) (*models.User, error)
	Exists(ctx context.Context, id string) (bool, error)
	EmailExists(ctx context.Context, email string) (bool, error)
	GetAllIDs(ctx context.Context) ([]string, error)
	Count(ctx context.Context) (int, error)
	StreamAll(ctx context.Context, callback func(*models.User) error) error
}

// ArticleRepository defines the interface for article data operations
type ArticleRepository interface {
	Create(ctx context.Context, article *models.Article) error
	BatchInsert(ctx context.Context, articles []*models.Article) (int, error)
	GetByID(ctx context.Context, id string) (*models.Article, error)
	Exists(ctx context.Context, id string) (bool, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
	GetAllIDs(ctx context.Context) ([]string, error)
	Count(ctx context.Context) (int, error)
	StreamAll(ctx context.Context, callback func(*models.Article) error) error
}

// CommentRepository defines the interface for comment data operations
type CommentRepository interface {
	Create(ctx context.Context, comment *models.Comment) error
	BatchInsert(ctx context.Context, comments []*models.Comment) (int, error)
	GetByID(ctx context.Context, id string) (*models.Comment, error)
	Exists(ctx context.Context, id string) (bool, error)
	Count(ctx context.Context) (int, error)
	StreamAll(ctx context.Context, callback func(*models.Comment) error) error
}

// JobRepository defines the interface for job data operations
type JobRepository interface {
	Create(ctx context.Context, job *models.Job) error
	Update(ctx context.Context, job *models.Job) error
	GetByID(ctx context.Context, id string) (*models.Job, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*models.Job, error)
	GetPendingJobs(ctx context.Context) ([]*models.Job, error)
	MarkJobAsProcessing(ctx context.Context, jobID string) (bool, error)
	AddError(ctx context.Context, jobID string, err *models.ValidationError) error
	AddErrors(ctx context.Context, jobID string, errors []models.ValidationError) error
	GetErrors(ctx context.Context, jobID string, limit int) ([]models.ValidationError, error)
}

// Repositories holds all repository interfaces
type Repositories struct {
	User    UserRepository
	Article ArticleRepository
	Comment CommentRepository
	Job     JobRepository
}

// New creates all repositories with the given database connection
func New(db *database.DB) *Repositories {
	return &Repositories{
		User:    NewUserRepo(db),
		Article: NewArticleRepo(db),
		Comment: NewCommentRepo(db),
		Job:     NewJobRepo(db),
	}
}
