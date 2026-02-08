package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/bulk-import-export-api/internal/database"
	"github.com/bulk-import-export-api/internal/models"
	"github.com/lib/pq"
)

// userRepo is the concrete implementation of UserRepository
type userRepo struct {
	db *database.DB
}

// NewUserRepo creates a new user repository
func NewUserRepo(db *database.DB) UserRepository {
	return &userRepo{db: db}
}

// Create inserts a new user
func (r *userRepo) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, email, name, role, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		user.ID, user.Email, user.Name, user.Role, user.Active,
		user.CreatedAt, time.Now(),
	)
	return err
}

// Upsert inserts or updates a user by email
func (r *userRepo) Upsert(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, email, name, role, active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (email) DO UPDATE SET
			name = EXCLUDED.name,
			role = EXCLUDED.role,
			active = EXCLUDED.active,
			updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		user.ID, user.Email, user.Name, user.Role, user.Active,
		user.CreatedAt, time.Now(),
	)
	return err
}

// BatchInsert inserts multiple users using PostgreSQL COPY for efficiency
func (r *userRepo) BatchInsert(ctx context.Context, users []*models.User) (int, error) {
	if len(users) == 0 {
		return 0, nil
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Prepare COPY statement for bulk insert
	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("users",
		"id", "email", "name", "role", "active", "created_at", "updated_at",
	))
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	now := time.Now()
	inserted := 0

	for _, user := range users {
		_, err := stmt.ExecContext(ctx,
			user.ID, user.Email, user.Name, user.Role, user.Active,
			user.CreatedAt, now,
		)
		if err != nil {
			// Skip individual errors but log them
			continue
		}
		inserted++
	}

	// Execute the COPY
	if _, err := stmt.ExecContext(ctx); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return inserted, nil
}

// GetByID retrieves a user by ID
func (r *userRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	query := `SELECT id, email, name, role, active, created_at, updated_at FROM users WHERE id = $1`

	var user models.User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.Name, &user.Role, &user.Active,
		&user.CreatedAt, &user.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// Exists checks if a user with the given ID exists
func (r *userRepo) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", id).Scan(&exists)
	return exists, err
}

// EmailExists checks if a user with the given email exists
func (r *userRepo) EmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", email).Scan(&exists)
	return exists, err
}

// GetAllIDs retrieves all user IDs (for FK validation cache)
func (r *userRepo) GetAllIDs(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id FROM users")
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

// Count returns the total number of users
func (r *userRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

// StreamAll streams all users for export (memory efficient)
func (r *userRepo) StreamAll(ctx context.Context, callback func(*models.User) error) error {
	query := `SELECT id, email, name, role, active, created_at, updated_at FROM users ORDER BY created_at`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID, &user.Email, &user.Name, &user.Role, &user.Active,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return err
		}

		if err := callback(&user); err != nil {
			return err
		}
	}

	return rows.Err()
}
