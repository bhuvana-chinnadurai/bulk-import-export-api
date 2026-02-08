package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/bulk-import-export-api/internal/database"
	"github.com/bulk-import-export-api/internal/models"
	"github.com/lib/pq"
)

// commentRepo is the concrete implementation of CommentRepository
type commentRepo struct {
	db *database.DB
}

// NewCommentRepo creates a new comment repository
func NewCommentRepo(db *database.DB) CommentRepository {
	return &commentRepo{db: db}
}

// Create inserts a new comment
func (r *commentRepo) Create(ctx context.Context, comment *models.Comment) error {
	query := `
		INSERT INTO comments (id, article_id, user_id, body, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query,
		comment.ID, comment.ArticleID, comment.UserID, comment.Body,
		comment.CreatedAt, time.Now(),
	)
	return err
}

// BatchInsert inserts multiple comments using PostgreSQL COPY
func (r *commentRepo) BatchInsert(ctx context.Context, comments []*models.Comment) (int, error) {
	if len(comments) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("comments",
		"id", "article_id", "user_id", "body", "created_at", "updated_at",
	))
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	now := time.Now()
	inserted := 0

	for _, comment := range comments {
		_, err := stmt.ExecContext(ctx,
			comment.ID, comment.ArticleID, comment.UserID, comment.Body,
			comment.CreatedAt, now,
		)
		if err != nil {
			continue
		}
		inserted++
	}

	if _, err := stmt.ExecContext(ctx); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return inserted, nil
}

// GetByID retrieves a comment by ID
func (r *commentRepo) GetByID(ctx context.Context, id string) (*models.Comment, error) {
	query := `SELECT id, article_id, user_id, body, created_at, updated_at FROM comments WHERE id = $1`

	var comment models.Comment
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&comment.ID, &comment.ArticleID, &comment.UserID, &comment.Body,
		&comment.CreatedAt, &comment.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &comment, nil
}

// Exists checks if a comment with the given ID exists
func (r *commentRepo) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM comments WHERE id = $1)", id).Scan(&exists)
	return exists, err
}

// Count returns the total number of comments
func (r *commentRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM comments").Scan(&count)
	return count, err
}

// StreamAll streams all comments for export
func (r *commentRepo) StreamAll(ctx context.Context, callback func(*models.Comment) error) error {
	query := `SELECT id, article_id, user_id, body, created_at, updated_at FROM comments ORDER BY created_at`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var comment models.Comment
		err := rows.Scan(
			&comment.ID, &comment.ArticleID, &comment.UserID, &comment.Body,
			&comment.CreatedAt, &comment.UpdatedAt,
		)
		if err != nil {
			return err
		}

		if err := callback(&comment); err != nil {
			return err
		}
	}

	return rows.Err()
}
