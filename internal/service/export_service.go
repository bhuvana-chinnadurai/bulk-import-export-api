package service

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bulk-import-export-api/internal/models"
	"github.com/bulk-import-export-api/internal/repository"
	"github.com/rs/zerolog"
)

// exportService is the concrete implementation of ExportService
type exportService struct {
	repos *repository.Repositories
	log   zerolog.Logger
}

// newExportService creates a new ExportService
func newExportService(repos *repository.Repositories, log zerolog.Logger) *exportService {
	return &exportService{
		repos: repos,
		log:   log.With().Str("service", "export").Logger(),
	}
}

// StreamUsers streams users in the specified format
func (s *exportService) StreamUsers(ctx context.Context, w http.ResponseWriter, format string) error {
	s.log.Info().Str("format", format).Msg("Starting users export")

	switch format {
	case "ndjson":
		return s.streamUsersNDJSON(ctx, w)
	case "json":
		return s.streamUsersJSON(ctx, w)
	case "csv":
		return s.streamUsersCSV(ctx, w)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// StreamArticles streams articles in the specified format
func (s *exportService) StreamArticles(ctx context.Context, w http.ResponseWriter, format string) error {
	s.log.Info().Str("format", format).Msg("Starting articles export")

	switch format {
	case "ndjson":
		return s.streamArticlesNDJSON(ctx, w)
	case "json":
		return s.streamArticlesJSON(ctx, w)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// StreamComments streams comments in the specified format
func (s *exportService) StreamComments(ctx context.Context, w http.ResponseWriter, format string) error {
	s.log.Info().Str("format", format).Msg("Starting comments export")

	switch format {
	case "ndjson":
		return s.streamCommentsNDJSON(ctx, w)
	case "json":
		return s.streamCommentsJSON(ctx, w)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// Users streaming implementations

func (s *exportService) streamUsersNDJSON(ctx context.Context, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", "attachment; filename=users.ndjson")

	flusher, _ := w.(http.Flusher)
	count := 0

	err := s.repos.User.StreamAll(ctx, func(user *models.User) error {
		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		w.Write(data)
		w.Write([]byte("\n"))
		count++

		// Flush every 100 records for streaming
		if count%100 == 0 && flusher != nil {
			flusher.Flush()
		}
		return nil
	})

	s.log.Info().Int("count", count).Msg("Users export completed")
	return err
}

func (s *exportService) streamUsersJSON(ctx context.Context, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=users.json")

	w.Write([]byte("["))
	first := true

	err := s.repos.User.StreamAll(ctx, func(user *models.User) error {
		if !first {
			w.Write([]byte(","))
		}
		first = false

		data, err := json.Marshal(user)
		if err != nil {
			return err
		}
		w.Write(data)
		return nil
	})

	w.Write([]byte("]"))
	return err
}

func (s *exportService) streamUsersCSV(ctx context.Context, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=users.csv")

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Write header
	writer.Write([]string{"id", "email", "name", "role", "active", "created_at", "updated_at"})

	return s.repos.User.StreamAll(ctx, func(user *models.User) error {
		active := "false"
		if user.Active {
			active = "true"
		}
		return writer.Write([]string{
			user.ID,
			user.Email,
			user.Name,
			user.Role,
			active,
			user.CreatedAt.Format("2006-01-02T15:04:05Z"),
			user.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	})
}

// Articles streaming implementations

func (s *exportService) streamArticlesNDJSON(ctx context.Context, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", "attachment; filename=articles.ndjson")

	flusher, _ := w.(http.Flusher)
	count := 0

	err := s.repos.Article.StreamAll(ctx, func(article *models.Article) error {
		data, err := json.Marshal(article)
		if err != nil {
			return err
		}
		w.Write(data)
		w.Write([]byte("\n"))
		count++

		if count%100 == 0 && flusher != nil {
			flusher.Flush()
		}
		return nil
	})

	s.log.Info().Int("count", count).Msg("Articles export completed")
	return err
}

func (s *exportService) streamArticlesJSON(ctx context.Context, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=articles.json")

	w.Write([]byte("["))
	first := true

	err := s.repos.Article.StreamAll(ctx, func(article *models.Article) error {
		if !first {
			w.Write([]byte(","))
		}
		first = false

		data, err := json.Marshal(article)
		if err != nil {
			return err
		}
		w.Write(data)
		return nil
	})

	w.Write([]byte("]"))
	return err
}

// Comments streaming implementations

func (s *exportService) streamCommentsNDJSON(ctx context.Context, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", "attachment; filename=comments.ndjson")

	flusher, _ := w.(http.Flusher)
	count := 0

	err := s.repos.Comment.StreamAll(ctx, func(comment *models.Comment) error {
		data, err := json.Marshal(comment)
		if err != nil {
			return err
		}
		w.Write(data)
		w.Write([]byte("\n"))
		count++

		if count%100 == 0 && flusher != nil {
			flusher.Flush()
		}
		return nil
	})

	s.log.Info().Int("count", count).Msg("Comments export completed")
	return err
}

func (s *exportService) streamCommentsJSON(ctx context.Context, w http.ResponseWriter) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=comments.json")

	w.Write([]byte("["))
	first := true

	err := s.repos.Comment.StreamAll(ctx, func(comment *models.Comment) error {
		if !first {
			w.Write([]byte(","))
		}
		first = false

		data, err := json.Marshal(comment)
		if err != nil {
			return err
		}
		w.Write(data)
		return nil
	})

	w.Write([]byte("]"))
	return err
}

// GetCount returns count for a resource
func (s *exportService) GetCount(ctx context.Context, resource string) (int, error) {
	switch resource {
	case "users":
		return s.repos.User.Count(ctx)
	case "articles":
		return s.repos.Article.Count(ctx)
	case "comments":
		return s.repos.Comment.Count(ctx)
	default:
		return 0, fmt.Errorf("unknown resource: %s", resource)
	}
}

// StreamResource streams any resource type
func (s *exportService) StreamResource(ctx context.Context, w io.Writer, resource, format string) error {
	hw, ok := w.(http.ResponseWriter)
	if !ok {
		return fmt.Errorf("writer is not http.ResponseWriter")
	}

	switch resource {
	case "users":
		return s.StreamUsers(ctx, hw, format)
	case "articles":
		return s.StreamArticles(ctx, hw, format)
	case "comments":
		return s.StreamComments(ctx, hw, format)
	default:
		return fmt.Errorf("unknown resource: %s", resource)
	}
}
