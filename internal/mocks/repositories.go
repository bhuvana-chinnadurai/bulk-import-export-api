package mocks

import (
	"context"

	"github.com/bulk-import-export-api/internal/models"
)

// MockUserRepository is a mock implementation of UserRepository
type MockUserRepository struct {
	Users            map[string]*models.User
	EmailToUser      map[string]*models.User
	InsertError      error
	InsertedCount    int
	StreamCallback   func(*models.User) error
	BatchInsertFunc  func(ctx context.Context, users []*models.User) (int, error)
	BatchInsertCalls int
}

func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		Users:       make(map[string]*models.User),
		EmailToUser: make(map[string]*models.User),
	}
}

func (m *MockUserRepository) Create(ctx context.Context, user *models.User) error {
	if m.InsertError != nil {
		return m.InsertError
	}
	m.Users[user.ID] = user
	m.EmailToUser[user.Email] = user
	return nil
}

func (m *MockUserRepository) Upsert(ctx context.Context, user *models.User) error {
	return m.Create(ctx, user)
}

func (m *MockUserRepository) BatchInsert(ctx context.Context, users []*models.User) (int, error) {
	m.BatchInsertCalls++
	if m.BatchInsertFunc != nil {
		return m.BatchInsertFunc(ctx, users)
	}
	if m.InsertError != nil {
		return 0, m.InsertError
	}
	for _, u := range users {
		m.Users[u.ID] = u
		m.EmailToUser[u.Email] = u
	}
	m.InsertedCount += len(users)
	return len(users), nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	return m.Users[id], nil
}

func (m *MockUserRepository) Exists(ctx context.Context, id string) (bool, error) {
	_, exists := m.Users[id]
	return exists, nil
}

func (m *MockUserRepository) EmailExists(ctx context.Context, email string) (bool, error) {
	_, exists := m.EmailToUser[email]
	return exists, nil
}

func (m *MockUserRepository) GetAllIDs(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(m.Users))
	for id := range m.Users {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *MockUserRepository) Count(ctx context.Context) (int, error) {
	return len(m.Users), nil
}

func (m *MockUserRepository) StreamAll(ctx context.Context, callback func(*models.User) error) error {
	for _, user := range m.Users {
		if err := callback(user); err != nil {
			return err
		}
	}
	return nil
}

// MockArticleRepository is a mock implementation of ArticleRepository
type MockArticleRepository struct {
	Articles         map[string]*models.Article
	SlugToArticle    map[string]*models.Article
	InsertError      error
	InsertedCount    int
	BatchInsertFunc  func(ctx context.Context, articles []*models.Article) (int, error)
	BatchInsertCalls int
}

func NewMockArticleRepository() *MockArticleRepository {
	return &MockArticleRepository{
		Articles:      make(map[string]*models.Article),
		SlugToArticle: make(map[string]*models.Article),
	}
}

func (m *MockArticleRepository) Create(ctx context.Context, article *models.Article) error {
	if m.InsertError != nil {
		return m.InsertError
	}
	m.Articles[article.ID] = article
	m.SlugToArticle[article.Slug] = article
	return nil
}

func (m *MockArticleRepository) BatchInsert(ctx context.Context, articles []*models.Article) (int, error) {
	m.BatchInsertCalls++
	if m.BatchInsertFunc != nil {
		return m.BatchInsertFunc(ctx, articles)
	}
	if m.InsertError != nil {
		return 0, m.InsertError
	}
	for _, a := range articles {
		m.Articles[a.ID] = a
		m.SlugToArticle[a.Slug] = a
	}
	m.InsertedCount += len(articles)
	return len(articles), nil
}

func (m *MockArticleRepository) GetByID(ctx context.Context, id string) (*models.Article, error) {
	return m.Articles[id], nil
}

func (m *MockArticleRepository) Exists(ctx context.Context, id string) (bool, error) {
	_, exists := m.Articles[id]
	return exists, nil
}

func (m *MockArticleRepository) SlugExists(ctx context.Context, slug string) (bool, error) {
	_, exists := m.SlugToArticle[slug]
	return exists, nil
}

func (m *MockArticleRepository) GetAllIDs(ctx context.Context) ([]string, error) {
	ids := make([]string, 0, len(m.Articles))
	for id := range m.Articles {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *MockArticleRepository) Count(ctx context.Context) (int, error) {
	return len(m.Articles), nil
}

func (m *MockArticleRepository) StreamAll(ctx context.Context, callback func(*models.Article) error) error {
	for _, article := range m.Articles {
		if err := callback(article); err != nil {
			return err
		}
	}
	return nil
}

// MockCommentRepository is a mock implementation of CommentRepository
type MockCommentRepository struct {
	Comments         map[string]*models.Comment
	InsertError      error
	InsertedCount    int
	BatchInsertFunc  func(ctx context.Context, comments []*models.Comment) (int, error)
	BatchInsertCalls int
}

func NewMockCommentRepository() *MockCommentRepository {
	return &MockCommentRepository{
		Comments: make(map[string]*models.Comment),
	}
}

func (m *MockCommentRepository) Create(ctx context.Context, comment *models.Comment) error {
	if m.InsertError != nil {
		return m.InsertError
	}
	m.Comments[comment.ID] = comment
	return nil
}

func (m *MockCommentRepository) BatchInsert(ctx context.Context, comments []*models.Comment) (int, error) {
	m.BatchInsertCalls++
	if m.BatchInsertFunc != nil {
		return m.BatchInsertFunc(ctx, comments)
	}
	if m.InsertError != nil {
		return 0, m.InsertError
	}
	for _, c := range comments {
		m.Comments[c.ID] = c
	}
	m.InsertedCount += len(comments)
	return len(comments), nil
}

func (m *MockCommentRepository) GetByID(ctx context.Context, id string) (*models.Comment, error) {
	return m.Comments[id], nil
}

func (m *MockCommentRepository) Exists(ctx context.Context, id string) (bool, error) {
	_, exists := m.Comments[id]
	return exists, nil
}

func (m *MockCommentRepository) Count(ctx context.Context) (int, error) {
	return len(m.Comments), nil
}

func (m *MockCommentRepository) StreamAll(ctx context.Context, callback func(*models.Comment) error) error {
	for _, comment := range m.Comments {
		if err := callback(comment); err != nil {
			return err
		}
	}
	return nil
}

// MockJobRepository is a mock implementation of JobRepository
type MockJobRepository struct {
	Jobs            map[string]*models.Job
	IdempotencyJobs map[string]*models.Job
	Errors          map[string][]models.ValidationError
	CreateError     error
	UpdateError     error
}

func NewMockJobRepository() *MockJobRepository {
	return &MockJobRepository{
		Jobs:            make(map[string]*models.Job),
		IdempotencyJobs: make(map[string]*models.Job),
		Errors:          make(map[string][]models.ValidationError),
	}
}

func (m *MockJobRepository) Create(ctx context.Context, job *models.Job) error {
	if m.CreateError != nil {
		return m.CreateError
	}
	m.Jobs[job.ID] = job
	if job.IdempotencyKey != "" {
		m.IdempotencyJobs[job.IdempotencyKey] = job
	}
	return nil
}

func (m *MockJobRepository) Update(ctx context.Context, job *models.Job) error {
	if m.UpdateError != nil {
		return m.UpdateError
	}
	m.Jobs[job.ID] = job
	return nil
}

func (m *MockJobRepository) GetByID(ctx context.Context, id string) (*models.Job, error) {
	return m.Jobs[id], nil
}

func (m *MockJobRepository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Job, error) {
	return m.IdempotencyJobs[key], nil
}

func (m *MockJobRepository) GetPendingJobs(ctx context.Context) ([]*models.Job, error) {
	var pending []*models.Job
	for _, job := range m.Jobs {
		if job.Status == models.JobStatusPending {
			pending = append(pending, job)
		}
	}
	return pending, nil
}

func (m *MockJobRepository) MarkJobAsProcessing(ctx context.Context, jobID string) (bool, error) {
	job, exists := m.Jobs[jobID]
	if !exists || job.Status != models.JobStatusPending {
		return false, nil
	}
	job.Status = models.JobStatusProcessing
	return true, nil
}

func (m *MockJobRepository) AddError(ctx context.Context, jobID string, err *models.ValidationError) error {
	m.Errors[jobID] = append(m.Errors[jobID], *err)
	return nil
}

func (m *MockJobRepository) AddErrors(ctx context.Context, jobID string, errors []models.ValidationError) error {
	m.Errors[jobID] = append(m.Errors[jobID], errors...)
	return nil
}

func (m *MockJobRepository) GetErrors(ctx context.Context, jobID string, limit int) ([]models.ValidationError, error) {
	errors := m.Errors[jobID]
	if limit > 0 && len(errors) > limit {
		return errors[:limit], nil
	}
	return errors, nil
}
