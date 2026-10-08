package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/feeelin/bookworm-api/handlers"
	"github.com/feeelin/bookworm-api/internal/testutil"
	"github.com/feeelin/bookworm-api/models"
	"github.com/feeelin/bookworm-api/repository"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// suite wires a handler onto a router the same way routes.RegisterRoutes does,
// so the tests exercise the handlers through real HTTP requests.
type suite struct {
	router *gin.Engine
	repo   *repository.BookRepository
}

func newSuite(t *testing.T, db *gorm.DB) *suite {
	t.Helper()

	repo := repository.NewBookRepository(db)
	handler := handlers.NewBookHandler(repo, testutil.NewValidator(t))

	router := gin.New()
	router.POST("/books", handler.CreateBook)
	router.GET("/books", handler.GetBooks)
	router.GET("/books/:id", handler.GetBook)
	router.PUT("/books/:id", handler.UpdateBook)
	router.DELETE("/books/:id", handler.DeleteBook)

	return &suite{router: router, repo: repo}
}

func (s *suite) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	s.router.ServeHTTP(recorder, request)

	return recorder
}

func (s *suite) seed(t *testing.T, mutators ...func(*models.Book)) *models.Book {
	t.Helper()

	book := testutil.NewBook(mutators...)
	if err := s.repo.Create(book); err != nil {
		t.Fatalf("seed book: %v", err)
	}

	return book
}

func decodeBook(t *testing.T, recorder *httptest.ResponseRecorder) models.Book {
	t.Helper()

	var book models.Book
	if err := json.Unmarshal(recorder.Body.Bytes(), &book); err != nil {
		t.Fatalf("decode book from %s: %v", recorder.Body.String(), err)
	}

	return book
}

func assertErrorPayload(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	var payload map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error payload from %s: %v", recorder.Body.String(), err)
	}
	if payload["error"] == "" {
		t.Errorf("expected a non-empty \"error\" field, got %s", recorder.Body.String())
	}
}

func assertStatus(t *testing.T, recorder *httptest.ResponseRecorder, want int) {
	t.Helper()

	if recorder.Code != want {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, want, recorder.Body.String())
	}
}

func TestNewBookHandler(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))
	validate := testutil.NewValidator(t)

	handler := handlers.NewBookHandler(repo, validate)

	if handler.Repo != repo {
		t.Error("handler does not hold the repository it was built with")
	}
	if handler.Validate != validate {
		t.Error("handler does not hold the validator it was built with")
	}
}

func TestCreateBook(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))

	body := `{"name":"Dune","author":"Frank Herbert","description":"Spice.","rating":9,"status":"finished"}`
	recorder := s.do(t, http.MethodPost, "/books", body)

	assertStatus(t, recorder, http.StatusCreated)

	created := decodeBook(t, recorder)
	if created.ID == 0 {
		t.Error("response has no generated ID")
	}
	if created.Name != "Dune" || created.Author != "Frank Herbert" {
		t.Errorf("response = %+v, want the submitted fields echoed back", created)
	}
	if created.Rating == nil || *created.Rating != 9 {
		t.Errorf("Rating = %v, want 9", created.Rating)
	}

	stored, err := s.repo.FindByID(created.ID)
	if err != nil {
		t.Fatalf("book was not persisted: %v", err)
	}
	if stored.Name != "Dune" {
		t.Errorf("stored name = %q, want %q", stored.Name, "Dune")
	}
}

func TestCreateBookWithoutRating(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))

	body := `{"name":"Dune","author":"Frank Herbert","description":"Spice.","status":"want_to_read"}`
	recorder := s.do(t, http.MethodPost, "/books", body)

	assertStatus(t, recorder, http.StatusCreated)

	if created := decodeBook(t, recorder); created.Rating != nil {
		t.Errorf("Rating = %d, want nil", *created.Rating)
	}
}

func TestCreateBookRejectsBadRequests(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "malformed json", body: `{"name":`},
		{name: "wrong type for rating", body: `{"name":"n","author":"a","description":"d","rating":"nine","status":"reading"}`},
		{name: "missing name", body: `{"author":"a","description":"d","status":"reading"}`},
		{name: "missing author", body: `{"name":"n","description":"d","status":"reading"}`},
		{name: "missing description", body: `{"name":"n","author":"a","status":"reading"}`},
		{name: "missing status", body: `{"name":"n","author":"a","description":"d"}`},
		{name: "unknown status", body: `{"name":"n","author":"a","description":"d","status":"paused"}`},
		{name: "rating too low", body: `{"name":"n","author":"a","description":"d","rating":0,"status":"reading"}`},
		{name: "rating too high", body: `{"name":"n","author":"a","description":"d","rating":11,"status":"reading"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newSuite(t, testutil.NewDB(t))

			recorder := s.do(t, http.MethodPost, "/books", tc.body)

			assertStatus(t, recorder, http.StatusBadRequest)
			assertErrorPayload(t, recorder)

			books, err := s.repo.FindAll()
			if err != nil {
				t.Fatalf("FindAll: %v", err)
			}
			if len(books) != 0 {
				t.Errorf("a rejected request persisted %d books", len(books))
			}
		})
	}
}

func TestCreateBookDatabaseFailure(t *testing.T) {
	s := newSuite(t, testutil.NewBrokenDB(t))

	body := `{"name":"n","author":"a","description":"d","status":"reading"}`
	recorder := s.do(t, http.MethodPost, "/books", body)

	assertStatus(t, recorder, http.StatusInternalServerError)
	assertErrorPayload(t, recorder)
}

func TestGetBooks(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))

	s.seed(t, func(b *models.Book) { b.Name = "First" })
	s.seed(t, func(b *models.Book) { b.Name = "Second" })

	recorder := s.do(t, http.MethodGet, "/books", "")

	assertStatus(t, recorder, http.StatusOK)

	var books []models.Book
	if err := json.Unmarshal(recorder.Body.Bytes(), &books); err != nil {
		t.Fatalf("decode books from %s: %v", recorder.Body.String(), err)
	}
	if len(books) != 2 {
		t.Fatalf("got %d books, want 2", len(books))
	}
	if books[0].Name != "First" || books[1].Name != "Second" {
		t.Errorf("books = %+v, want First then Second", books)
	}
}

func TestGetBooksWhenEmpty(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))

	recorder := s.do(t, http.MethodGet, "/books", "")

	assertStatus(t, recorder, http.StatusOK)

	if body := strings.TrimSpace(recorder.Body.String()); body != "[]" {
		t.Errorf("body = %s, want []", body)
	}
}

func TestGetBooksDatabaseFailure(t *testing.T) {
	s := newSuite(t, testutil.NewBrokenDB(t))

	recorder := s.do(t, http.MethodGet, "/books", "")

	assertStatus(t, recorder, http.StatusInternalServerError)
	assertErrorPayload(t, recorder)
}

func TestGetBook(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))
	book := s.seed(t)

	recorder := s.do(t, http.MethodGet, "/books/"+strconv.FormatUint(uint64(book.ID), 10), "")

	assertStatus(t, recorder, http.StatusOK)

	got := decodeBook(t, recorder)
	if got.ID != book.ID {
		t.Errorf("ID = %d, want %d", got.ID, book.ID)
	}
	if got.Name != book.Name {
		t.Errorf("Name = %q, want %q", got.Name, book.Name)
	}
}

func TestGetBookNotFound(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))

	recorder := s.do(t, http.MethodGet, "/books/999", "")

	assertStatus(t, recorder, http.StatusNotFound)
	assertErrorPayload(t, recorder)
}

func TestGetBookInvalidID(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))

	for _, id := range []string{"abc", "-1", "1.5"} {
		t.Run(id, func(t *testing.T) {
			recorder := s.do(t, http.MethodGet, "/books/"+id, "")

			assertStatus(t, recorder, http.StatusBadRequest)
			assertErrorPayload(t, recorder)
		})
	}
}

func TestGetBookDatabaseFailure(t *testing.T) {
	s := newSuite(t, testutil.NewBrokenDB(t))

	recorder := s.do(t, http.MethodGet, "/books/1", "")

	assertStatus(t, recorder, http.StatusInternalServerError)
	assertErrorPayload(t, recorder)
}

func TestUpdateBook(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))
	book := s.seed(t)

	body := `{"name":"Renamed","author":"New Author","description":"New description.","rating":3,"status":"reading"}`
	recorder := s.do(t, http.MethodPut, "/books/"+strconv.FormatUint(uint64(book.ID), 10), body)

	assertStatus(t, recorder, http.StatusOK)

	updated := decodeBook(t, recorder)
	if updated.ID != book.ID {
		t.Errorf("ID = %d, want the id from the URL (%d)", updated.ID, book.ID)
	}
	if updated.Name != "Renamed" {
		t.Errorf("Name = %q, want %q", updated.Name, "Renamed")
	}

	stored, err := s.repo.FindByID(book.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if stored.Name != "Renamed" || stored.Status != models.Reading {
		t.Errorf("stored book = %+v, want the updated fields", stored)
	}
	if stored.Rating == nil || *stored.Rating != 3 {
		t.Errorf("stored Rating = %v, want 3", stored.Rating)
	}
}

// The id in the URL wins over any id in the payload, otherwise a client could
// overwrite an unrelated row.
func TestUpdateBookIgnoresIDInBody(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))
	target := s.seed(t, func(b *models.Book) { b.Name = "Target" })
	other := s.seed(t, func(b *models.Book) { b.Name = "Other" })

	body := `{"ID":` + strconv.FormatUint(uint64(other.ID), 10) +
		`,"name":"Renamed","author":"a","description":"d","status":"reading"}`
	recorder := s.do(t, http.MethodPut, "/books/"+strconv.FormatUint(uint64(target.ID), 10), body)

	assertStatus(t, recorder, http.StatusOK)

	updatedTarget, err := s.repo.FindByID(target.ID)
	if err != nil {
		t.Fatalf("FindByID target: %v", err)
	}
	if updatedTarget.Name != "Renamed" {
		t.Errorf("target name = %q, want %q", updatedTarget.Name, "Renamed")
	}

	untouched, err := s.repo.FindByID(other.ID)
	if err != nil {
		t.Fatalf("FindByID other: %v", err)
	}
	if untouched.Name != "Other" {
		t.Errorf("other book was modified: name = %q, want %q", untouched.Name, "Other")
	}
}

func TestUpdateBookNotFound(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))

	body := `{"name":"n","author":"a","description":"d","status":"reading"}`
	recorder := s.do(t, http.MethodPut, "/books/999", body)

	assertStatus(t, recorder, http.StatusNotFound)
	assertErrorPayload(t, recorder)
}

func TestUpdateBookInvalidID(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))

	body := `{"name":"n","author":"a","description":"d","status":"reading"}`
	recorder := s.do(t, http.MethodPut, "/books/abc", body)

	assertStatus(t, recorder, http.StatusBadRequest)
	assertErrorPayload(t, recorder)
}

func TestUpdateBookRejectsBadRequests(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "malformed json", body: `{"name":`},
		{name: "missing name", body: `{"author":"a","description":"d","status":"reading"}`},
		{name: "missing status", body: `{"name":"n","author":"a","description":"d"}`},
		{name: "unknown status", body: `{"name":"n","author":"a","description":"d","status":"paused"}`},
		{name: "rating too high", body: `{"name":"n","author":"a","description":"d","rating":11,"status":"reading"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := newSuite(t, testutil.NewDB(t))
			book := s.seed(t, func(b *models.Book) { b.Name = "Original" })

			path := "/books/" + strconv.FormatUint(uint64(book.ID), 10)
			recorder := s.do(t, http.MethodPut, path, tc.body)

			assertStatus(t, recorder, http.StatusBadRequest)
			assertErrorPayload(t, recorder)

			stored, err := s.repo.FindByID(book.ID)
			if err != nil {
				t.Fatalf("FindByID: %v", err)
			}
			if stored.Name != "Original" {
				t.Errorf("a rejected request changed the stored book: name = %q", stored.Name)
			}
		})
	}
}

func TestUpdateBookDatabaseFailure(t *testing.T) {
	s := newSuite(t, testutil.NewBrokenDB(t))

	body := `{"name":"n","author":"a","description":"d","status":"reading"}`
	recorder := s.do(t, http.MethodPut, "/books/1", body)

	assertStatus(t, recorder, http.StatusInternalServerError)
	assertErrorPayload(t, recorder)
}

func TestDeleteBook(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))
	book := s.seed(t)

	recorder := s.do(t, http.MethodDelete, "/books/"+strconv.FormatUint(uint64(book.ID), 10), "")

	assertStatus(t, recorder, http.StatusNoContent)

	if _, err := s.repo.FindByID(book.ID); err == nil {
		t.Error("book is still readable after DELETE")
	}
}

func TestDeleteBookNotFound(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))

	recorder := s.do(t, http.MethodDelete, "/books/999", "")

	assertStatus(t, recorder, http.StatusNotFound)
	assertErrorPayload(t, recorder)
}

func TestDeleteBookTwice(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))
	book := s.seed(t)

	path := "/books/" + strconv.FormatUint(uint64(book.ID), 10)

	assertStatus(t, s.do(t, http.MethodDelete, path, ""), http.StatusNoContent)
	assertStatus(t, s.do(t, http.MethodDelete, path, ""), http.StatusNotFound)
}

func TestDeleteBookInvalidID(t *testing.T) {
	s := newSuite(t, testutil.NewDB(t))

	recorder := s.do(t, http.MethodDelete, "/books/abc", "")

	assertStatus(t, recorder, http.StatusBadRequest)
	assertErrorPayload(t, recorder)
}

func TestDeleteBookDatabaseFailure(t *testing.T) {
	s := newSuite(t, testutil.NewBrokenDB(t))

	recorder := s.do(t, http.MethodDelete, "/books/1", "")

	assertStatus(t, recorder, http.StatusInternalServerError)
	assertErrorPayload(t, recorder)
}

// A write that fails *after* the existence check must surface as a 500. A
// unique index on name makes the Save fail while the lookup still succeeds.
func TestUpdateBookSaveFailure(t *testing.T) {
	db := testutil.NewDB(t)
	s := newSuite(t, db)

	target := s.seed(t, func(b *models.Book) { b.Name = "Target" })
	s.seed(t, func(b *models.Book) { b.Name = "Taken" })

	if err := db.Exec("CREATE UNIQUE INDEX idx_books_name ON books(name)").Error; err != nil {
		t.Fatalf("create unique index: %v", err)
	}

	body := `{"name":"Taken","author":"a","description":"d","status":"reading"}`
	recorder := s.do(t, http.MethodPut, "/books/"+strconv.FormatUint(uint64(target.ID), 10), body)

	assertStatus(t, recorder, http.StatusInternalServerError)
	assertErrorPayload(t, recorder)

	stored, err := s.repo.FindByID(target.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if stored.Name != "Target" {
		t.Errorf("stored name = %q, want it unchanged at %q", stored.Name, "Target")
	}
}
