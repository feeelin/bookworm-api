//go:build integration

// Package integration exercises the assembled application: the real router
// built by routes.RegisterRoutes, the real validator, and a real on-disk
// SQLite database, driven over TCP by an ordinary HTTP client.
//
// Run with: go test -tags=integration ./integration/...
package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/feeelin/bookworm-api/config"
	"github.com/feeelin/bookworm-api/models"
	"github.com/feeelin/bookworm-api/routes"
	"github.com/feeelin/bookworm-api/validators"

	// Imported for its init, which registers the swagger spec. main.go does
	// the same; without it /swagger/doc.json answers 500.
	_ "github.com/feeelin/bookworm-api/docs"
)

func TestMain(m *testing.M) {
	// Match how the binary runs in production, minus the request logging.
	gin.SetMode(gin.ReleaseMode)
	gin.DefaultWriter = io.Discard
	gin.DefaultErrorWriter = io.Discard

	os.Exit(m.Run())
}

// app is a running instance of the API: a real HTTP server in front of a real
// database file.
type app struct {
	t       *testing.T
	baseURL string
	client  *http.Client
	db      *gorm.DB
	dbPath  string
	server  *httptest.Server
}

// newApp starts the API over a fresh database file in the test's temp dir.
func newApp(t *testing.T) *app {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "database.db")

	return startApp(t, openDB(t, dbPath), dbPath)
}

// newAppFromConfig starts the API exactly the way main.go does, going through
// config.ConnectDatabase. That function resolves "database.db" against the
// working directory, so the test moves there first.
func newAppFromConfig(t *testing.T) *app {
	t.Helper()

	dir := t.TempDir()
	t.Chdir(dir)

	db := config.ConnectDatabase()

	return startApp(t, db, filepath.Join(dir, "database.db"))
}

func startApp(t *testing.T, db *gorm.DB, dbPath string) *app {
	t.Helper()

	config.RunMigrations(db)

	validate := validator.New()
	validators.RegisterStatusValidator(validate)

	router := gin.Default()
	routes.RegisterRoutes(router, db, validate)

	server := httptest.NewServer(router)

	a := &app{
		t:       t,
		baseURL: server.URL,
		client:  server.Client(),
		db:      db,
		dbPath:  dbPath,
		server:  server,
	}

	t.Cleanup(a.stop)

	return a
}

// stop shuts the instance down, releasing the port and the database file.
func (a *app) stop() {
	a.server.Close()

	if sqlDB, err := a.db.DB(); err == nil {
		sqlDB.Close()
	}
}

func openDB(t *testing.T, path string) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open database at %s: %v", path, err)
	}

	return db
}

// response is a fully-read HTTP response, so the body can be asserted on more
// than once without worrying about the stream.
type response struct {
	t      *testing.T
	status int
	body   []byte
	header http.Header
}

func (a *app) do(method, path, body string) *response {
	a.t.Helper()

	request, err := http.NewRequest(method, a.baseURL+path, strings.NewReader(body))
	if err != nil {
		a.t.Fatalf("build %s %s: %v", method, path, err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	return a.send(request)
}

func (a *app) send(request *http.Request) *response {
	a.t.Helper()

	httpResponse, err := a.client.Do(request)
	if err != nil {
		a.t.Fatalf("%s %s: %v", request.Method, request.URL.Path, err)
	}
	defer httpResponse.Body.Close()

	payload, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		a.t.Fatalf("read body of %s %s: %v", request.Method, request.URL.Path, err)
	}

	return &response{
		t:      a.t,
		status: httpResponse.StatusCode,
		body:   payload,
		header: httpResponse.Header,
	}
}

func (a *app) get(path string) *response    { return a.do(http.MethodGet, path, "") }
func (a *app) delete(path string) *response { return a.do(http.MethodDelete, path, "") }

func (a *app) post(path, body string) *response {
	return a.do(http.MethodPost, path, body)
}

func (a *app) put(path, body string) *response {
	return a.do(http.MethodPut, path, body)
}

// bookPath is the canonical URL of a single book.
func bookPath(id uint) string {
	return "/books/" + strconv.FormatUint(uint64(id), 10)
}

// createBook posts a book and returns it, failing the test if it is rejected.
func (a *app) createBook(body string) models.Book {
	a.t.Helper()

	resp := a.post("/books", body)
	resp.assertStatus(http.StatusCreated)

	return resp.book()
}

// countRows reports how many book rows exist, optionally including the ones
// GORM has soft-deleted.
func (a *app) countRows(includeDeleted bool) int64 {
	a.t.Helper()

	query := a.db.Model(&models.Book{})
	if includeDeleted {
		query = query.Unscoped()
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		a.t.Fatalf("count rows: %v", err)
	}

	return count
}

func (r *response) assertStatus(want int) *response {
	r.t.Helper()

	if r.status != want {
		r.t.Fatalf("status = %d, want %d (body: %s)", r.status, want, r.body)
	}

	return r
}

func (r *response) book() models.Book {
	r.t.Helper()

	var book models.Book
	if err := json.Unmarshal(r.body, &book); err != nil {
		r.t.Fatalf("decode book from %s: %v", r.body, err)
	}

	return book
}

func (r *response) books() []models.Book {
	r.t.Helper()

	var books []models.Book
	if err := json.Unmarshal(r.body, &books); err != nil {
		r.t.Fatalf("decode books from %s: %v", r.body, err)
	}

	return books
}

// assertErrorPayload checks the response carries a non-empty error message.
func (r *response) assertErrorPayload() {
	r.t.Helper()

	var payload map[string]string
	if err := json.Unmarshal(r.body, &payload); err != nil {
		r.t.Fatalf("decode error payload from %s: %v", r.body, err)
	}
	if payload["error"] == "" {
		r.t.Errorf("expected a non-empty \"error\" field, got %s", r.body)
	}
}

// bookJSON builds a request body. A rating below 1 is omitted from the JSON.
func bookJSON(name, author, description string, rating int, status models.Status) string {
	fields := []string{
		`"name":` + quote(name),
		`"author":` + quote(author),
		`"description":` + quote(description),
	}
	if rating >= 1 {
		fields = append(fields, `"rating":`+strconv.Itoa(rating))
	}
	fields = append(fields, `"status":`+quote(string(status)))

	return "{" + strings.Join(fields, ",") + "}"
}

func quote(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}

	return string(encoded)
}
