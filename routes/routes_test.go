package routes_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/feeelin/bookworm-api/internal/testutil"
	"github.com/feeelin/bookworm-api/models"
	"github.com/feeelin/bookworm-api/routes"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newRouter(t *testing.T) *gin.Engine {
	t.Helper()

	router := gin.New()
	routes.RegisterRoutes(router, testutil.NewDB(t), testutil.NewValidator(t))

	return router
}

func TestRegisterRoutesRegistersBookEndpoints(t *testing.T) {
	router := newRouter(t)

	want := map[string]string{
		http.MethodPost:   "/books",
		http.MethodGet:    "/books",
		http.MethodPut:    "/books/:id",
		http.MethodDelete: "/books/:id",
	}

	registered := make(map[string]map[string]bool)
	for _, route := range router.Routes() {
		if registered[route.Method] == nil {
			registered[route.Method] = make(map[string]bool)
		}
		registered[route.Method][route.Path] = true
	}

	for method, path := range want {
		if !registered[method][path] {
			t.Errorf("route %s %s is not registered", method, path)
		}
	}

	if !registered[http.MethodGet]["/books/:id"] {
		t.Error("route GET /books/:id is not registered")
	}
	if !registered[http.MethodGet]["/swagger/*any"] {
		t.Error("route GET /swagger/*any is not registered")
	}
}

func TestRegisterRoutesHasNoUnexpectedRoutes(t *testing.T) {
	router := newRouter(t)

	expected := map[string]bool{
		"POST /books":       true,
		"GET /books":        true,
		"GET /books/:id":    true,
		"PUT /books/:id":    true,
		"DELETE /books/:id": true,
		"GET /swagger/*any": true,
	}

	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if !expected[key] {
			t.Errorf("unexpected route registered: %s", key)
		}
	}
}

// A request through the fully wired router proves the handler, repository and
// validator are connected to each other, not just individually correct.
func TestRegisteredRoutesServeRequestsEndToEnd(t *testing.T) {
	router := newRouter(t)

	body := `{"name":"Dune","author":"Frank Herbert","description":"Spice.","rating":9,"status":"finished"}`
	created := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/books", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(created, request)

	if created.Code != http.StatusCreated {
		t.Fatalf("POST /books status = %d, want %d (body: %s)", created.Code, http.StatusCreated, created.Body)
	}

	var book models.Book
	if err := json.Unmarshal(created.Body.Bytes(), &book); err != nil {
		t.Fatalf("decode created book: %v", err)
	}
	if book.ID == 0 {
		t.Fatal("created book has no ID")
	}

	listed := httptest.NewRecorder()
	router.ServeHTTP(listed, httptest.NewRequest(http.MethodGet, "/books", nil))

	if listed.Code != http.StatusOK {
		t.Fatalf("GET /books status = %d, want %d", listed.Code, http.StatusOK)
	}

	var books []models.Book
	if err := json.Unmarshal(listed.Body.Bytes(), &books); err != nil {
		t.Fatalf("decode listed books: %v", err)
	}
	if len(books) != 1 || books[0].ID != book.ID {
		t.Errorf("GET /books = %+v, want the single created book", books)
	}
}

func TestSwaggerRouteIsReachable(t *testing.T) {
	router := newRouter(t)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))

	if recorder.Code == http.StatusNotFound {
		t.Error("GET /swagger/index.html returned 404; the swagger handler is not wired up")
	}
}

func TestUnknownRouteReturnsNotFound(t *testing.T) {
	router := newRouter(t)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/authors", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
