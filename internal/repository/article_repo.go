package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/bulk-import-export-api/internal/database"
	"github.com/bulk-import-export-api/internal/models"
	"github.com/lib/pq"
)

// articleRepo is the concrete implementation of ArticleRepository
type articleRepo struct {
	db *database.DB
}

// NewArticleRepo creates a new article repository
func NewArticleRepo(db *database.DB) ArticleRepository {
	return &articleRepo{db: db}
}

// Create inserts a new article
func (r *articleRepo) Create(ctx context.Context, article *models.Article) error {
	tagsJSON, _ := json.Marshal(article.Tags)

	query := `
		INSERT INTO articles (id, slug, title, body, author_id, tags, status, published_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err := r.db.ExecContext(ctx, query,
		article.ID, article.Slug, article.Title, article.Body, article.AuthorID,
		tagsJSON, article.Status, article.PublishedAt,
		article.CreatedAt, time.Now(),
	)
	return err
}

// BatchInsert inserts multiple articles using PostgreSQL COPY
func (r *articleRepo) BatchInsert(ctx context.Context, articles []*models.Article) (int, error) {
	if len(articles) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("articles",
		"id", "slug", "title", "body", "author_id", "tags", "status", "published_at", "created_at", "updated_at",
	))
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	now := time.Now()
	inserted := 0

	for _, article := range articles {
		tagsJSON, _ := json.Marshal(article.Tags)
		if article.Tags == nil {
			tagsJSON = []byte("[]")
		}

		_, err := stmt.ExecContext(ctx,
			article.ID, article.Slug, article.Title, article.Body, article.AuthorID,
			string(tagsJSON), article.Status, article.PublishedAt,
			article.CreatedAt, now,
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

// GetByID retrieves an article by ID
func (r *articleRepo) GetByID(ctx context.Context, id string) (*models.Article, error) {
	query := `
		SELECT id, slug, title, body, author_id, tags, status, published_at, created_at, updated_at 
		FROM articles WHERE id = $1
	`

	var article models.Article
	var tagsJSON []byte
	var publishedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&article.ID, &article.Slug, &article.Title, &article.Body, &article.AuthorID,
		&tagsJSON, &article.Status, &publishedAt, &article.CreatedAt, &article.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(tagsJSON, &article.Tags)
	if publishedAt.Valid {
		article.PublishedAt = &publishedAt.Time
	}

	return &article, nil
}

// Exists checks if an article with the given ID exists
func (r *articleRepo) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM articles WHERE id = $1)", id).Scan(&exists)
	return exists, err
}

// SlugExists checks if an article with the given slug exists
func (r *articleRepo) SlugExists(ctx context.Context, slug string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM articles WHERE slug = $1)", slug).Scan(&exists)
	return exists, err
}

// GetAllIDs retrieves all article IDs (for FK validation cache)
func (r *articleRepo) GetAllIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id FROM articles")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Count returns the total number of articles
func (r *articleRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM articles").Scan(&count)
	return count, err
}

// StreamAll streams all articles for export
func (r *articleRepo) StreamAll(ctx context.Context, callback func(*models.Article) error) error {
	query := `
		SELECT id, slug, title, body, author_id, tags, status, published_at, created_at, updated_at 
		FROM articles ORDER BY created_at
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var article models.Article
		var tagsJSON []byte
		var publishedAt sql.NullTime

		err := rows.Scan(
			&article.ID, &article.Slug, &article.Title, &article.Body, &article.AuthorID,
			&tagsJSON, &article.Status, &publishedAt, &article.CreatedAt, &article.UpdatedAt,
		)
		if err != nil {
			return err
		}

		json.Unmarshal(tagsJSON, &article.Tags)
		if publishedAt.Valid {
			article.PublishedAt = &publishedAt.Time
		}

		if err := callback(&article); err != nil {
			return err
		}
	}

	return rows.Err()
}
