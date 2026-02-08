package validation

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/bulk-import-export-api/internal/models"
)

func testdataPath(t *testing.T, filename string) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	projectRoot := filepath.Dir(filepath.Dir(filepath.Dir(currentFile)))
	path := filepath.Join(projectRoot, "testdata", filename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("testdata file not found: %s", path)
	}
	return path
}

func TestValidateUser_RealCSVData(t *testing.T) {
	filePath := testdataPath(t, "users_huge.csv")

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}

	headerMap := make(map[string]int)
	for i, h := range header {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	validator := NewValidator()
	totalRecords := 0
	totalFailed := 0
	lineNum := 1

	// Process first 100 records to validate error detection
	for i := 0; i < 100; i++ {
		record, err := reader.Read()
		if err != nil {
			break
		}
		lineNum++
		totalRecords++

		userCSV := &models.UserCSV{
			ID:        getCSVField(record, headerMap, "id"),
			Email:     getCSVField(record, headerMap, "email"),
			Name:      getCSVField(record, headerMap, "name"),
			Role:      getCSVField(record, headerMap, "role"),
			Active:    getCSVField(record, headerMap, "active"),
			CreatedAt: getCSVField(record, headerMap, "created_at"),
		}

		errors := validator.ValidateUser(userCSV, lineNum)
		if len(errors) > 0 {
			totalFailed++
		} else {
			validator.AddUserEmail(userCSV.Email)
		}
	}

	if totalRecords != 100 {
		t.Errorf("Expected to read 100 records, got %d", totalRecords)
	}

	// First record (line 2) should fail: missing ID, invalid email "foo@bar", invalid role "manager"
	file.Seek(0, 0)
	reader = csv.NewReader(file)
	reader.Read() // skip header
	firstRecord, _ := reader.Read()

	firstUser := &models.UserCSV{
		ID:        getCSVField(firstRecord, headerMap, "id"),
		Email:     getCSVField(firstRecord, headerMap, "email"),
		Name:      getCSVField(firstRecord, headerMap, "name"),
		Role:      getCSVField(firstRecord, headerMap, "role"),
		Active:    getCSVField(firstRecord, headerMap, "active"),
		CreatedAt: getCSVField(firstRecord, headerMap, "created_at"),
	}

	v2 := NewValidator()
	errors := v2.ValidateUser(firstUser, 2)
	if len(errors) == 0 {
		t.Error("First data row should have validation errors (missing ID, invalid email, invalid role)")
	}

	// Check specific errors on line 2
	errorFields := make(map[string]bool)
	for _, e := range errors {
		errorFields[e.Field] = true
	}
	if !errorFields["id"] {
		t.Error("Line 2: expected error for missing ID")
	}
	if !errorFields["role"] {
		t.Error("Line 2: expected error for invalid role 'manager'")
	}

	t.Logf("Validated %d records from real CSV: %d failed", totalRecords, totalFailed)
}

func TestValidateArticle_RealNDJSONData(t *testing.T) {
	filePath := testdataPath(t, "articles_huge.ndjson")

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	validator := NewValidator()
	totalRecords := 0
	totalFailed := 0

	// Process first 100 records
	for i := 0; i < 100 && scanner.Scan(); i++ {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		totalRecords++

		var article models.ArticleNDJSON
		if err := json.Unmarshal([]byte(line), &article); err != nil {
			totalFailed++
			continue
		}

		errors := validator.ValidateArticle(&article, i+1)
		if len(errors) > 0 {
			totalFailed++
		} else {
			validator.AddArticleSlug(article.Slug)
		}
	}

	if totalRecords == 0 {
		t.Error("Expected to read some records from articles NDJSON")
	}

	// First article has missing ID - should fail
	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)
	scanner.Scan()
	firstLine := scanner.Text()

	var firstArticle models.ArticleNDJSON
	if err := json.Unmarshal([]byte(firstLine), &firstArticle); err != nil {
		t.Fatalf("Failed to parse first article: %v", err)
	}

	v2 := NewValidator()
	errors := v2.ValidateArticle(&firstArticle, 1)
	if len(errors) == 0 {
		t.Error("First article should have validation errors (missing ID)")
	}
	hasIDError := false
	for _, e := range errors {
		if e.Field == "id" {
			hasIDError = true
		}
	}
	if !hasIDError {
		t.Error("First article should have an 'id' validation error")
	}

	t.Logf("Validated %d records from real NDJSON: %d failed", totalRecords, totalFailed)
}

func TestValidateComment_RealNDJSONData(t *testing.T) {
	filePath := testdataPath(t, "comments_huge.ndjson")

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	validator := NewValidator()
	totalRecords := 0
	totalFailed := 0

	// Process first 100 records
	for i := 0; i < 100 && scanner.Scan(); i++ {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		totalRecords++

		var comment models.CommentNDJSON
		if err := json.Unmarshal([]byte(line), &comment); err != nil {
			totalFailed++
			continue
		}

		errors := validator.ValidateComment(&comment, i+1)
		if len(errors) > 0 {
			totalFailed++
		}
	}

	if totalRecords == 0 {
		t.Error("Expected to read some records from comments NDJSON")
	}

	// First comment has missing ID and body - should fail
	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)
	scanner.Scan()
	firstLine := scanner.Text()

	var firstComment models.CommentNDJSON
	if err := json.Unmarshal([]byte(firstLine), &firstComment); err != nil {
		t.Fatalf("Failed to parse first comment: %v", err)
	}

	v2 := NewValidator()
	errors := v2.ValidateComment(&firstComment, 1)
	if len(errors) == 0 {
		t.Error("First comment should have validation errors (missing ID and body)")
	}
	errorFields := make(map[string]bool)
	for _, e := range errors {
		errorFields[e.Field] = true
	}
	if !errorFields["id"] {
		t.Error("First comment should have an 'id' validation error")
	}
	if !errorFields["body"] {
		t.Error("First comment should have a 'body' validation error")
	}

	// Line 4 (index 3): article_id "a-missing-12510" is not a valid UUID
	file.Seek(0, 0)
	scanner = bufio.NewScanner(file)
	for i := 0; i < 4; i++ {
		scanner.Scan()
	}
	line4 := scanner.Text()
	var comment4 models.CommentNDJSON
	json.Unmarshal([]byte(line4), &comment4)

	v3 := NewValidator()
	errors4 := v3.ValidateComment(&comment4, 4)
	hasArticleIDError := false
	for _, e := range errors4 {
		if e.Field == "article_id" {
			hasArticleIDError = true
		}
	}
	if !hasArticleIDError {
		t.Error("Line 4 comment should have an article_id validation error for 'a-missing-12510'")
	}

	t.Logf("Validated %d records from real NDJSON: %d failed", totalRecords, totalFailed)
}

func TestValidateDuplicateEmails_AcrossRealData(t *testing.T) {
	filePath := testdataPath(t, "users_huge.csv")

	file, err := os.Open(filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}

	headerMap := make(map[string]int)
	for i, h := range header {
		headerMap[strings.ToLower(strings.TrimSpace(h))] = i
	}

	validator := NewValidator()
	duplicateCount := 0
	lineNum := 1

	// Process ALL records to test duplicate detection across the full file
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}
		lineNum++

		userCSV := &models.UserCSV{
			ID:        getCSVField(record, headerMap, "id"),
			Email:     getCSVField(record, headerMap, "email"),
			Name:      getCSVField(record, headerMap, "name"),
			Role:      getCSVField(record, headerMap, "role"),
			Active:    getCSVField(record, headerMap, "active"),
			CreatedAt: getCSVField(record, headerMap, "created_at"),
		}

		errors := validator.ValidateUser(userCSV, lineNum)
		hasDuplicate := false
		for _, e := range errors {
			if e.Field == "email" && e.Message == "duplicate email" {
				hasDuplicate = true
				duplicateCount++
			}
		}

		if len(errors) == 0 || !hasDuplicate {
			// Only add to cache if not a duplicate (simulating the import flow)
			if userCSV.Email != "" {
				validator.AddUserEmail(userCSV.Email)
			}
		}
	}

	if duplicateCount == 0 {
		t.Error("Expected to detect duplicate emails across the full dataset")
	}
	t.Logf("Detected %d duplicate emails across %d records", duplicateCount, lineNum-1)
}

func getCSVField(record []string, headerMap map[string]int, field string) string {
	if idx, ok := headerMap[field]; ok && idx < len(record) {
		return strings.TrimSpace(record[idx])
	}
	return ""
}
