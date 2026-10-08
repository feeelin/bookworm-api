//go:build integration

package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/feeelin/bookworm-api/models"
)

// TestApplicationWiringFromConfig boots the stack through config.ConnectDatabase,
// the way main.go does, and serves a request end to end. It is the one test
// that would fail if the startup sequence in main.go stopped working.
func TestApplicationWiringFromConfig(t *testing.T) {
	a := newAppFromConfig(t)

	if _, err := os.Stat(a.dbPath); err != nil {
		t.Fatalf("expected ConnectDatabase to create %s: %v", a.dbPath, err)
	}

	created := a.createBook(bookJSON("Dune", "Frank Herbert", "Spice.", 9, models.Finished))

	fetched := a.get(bookPath(created.ID)).assertStatus(http.StatusOK).book()
	if fetched.Name != "Dune" {
		t.Errorf("Name = %q, want %q", fetched.Name, "Dune")
	}

	// The custom validator has to be registered, or a bad status would be
	// accepted instead of rejected.
	bad := a.post("/books", `{"name":"n","author":"a","description":"d","status":"paused"}`)
	bad.assertStatus(http.StatusBadRequest)
	if !strings.Contains(string(bad.body), "validstatus") {
		t.Errorf("error = %s, want it to name the validstatus rule", bad.body)
	}
}

// Data written through the API must still be there after the process that
// wrote it is gone. The server and connection are torn down and a second
// instance is pointed at the same file.
func TestDataSurvivesARestart(t *testing.T) {
	first := newApp(t)

	created := first.createBook(bookJSON("Dune", "Frank Herbert", "Spice.", 9, models.Finished))
	deleted := first.createBook(bookJSON("Abandoned", "author", "description", 0, models.WantToRead))
	first.delete(bookPath(deleted.ID)).assertStatus(http.StatusNoContent)

	dbPath := first.dbPath
	first.stop()

	second := startApp(t, openDB(t, dbPath), dbPath)

	survivor := second.get(bookPath(created.ID)).assertStatus(http.StatusOK).book()
	if survivor.Name != "Dune" {
		t.Errorf("Name = %q, want %q", survivor.Name, "Dune")
	}
	if survivor.Rating == nil || *survivor.Rating != 9 {
		t.Errorf("Rating = %v, want 9", survivor.Rating)
	}

	// The delete has to survive the restart too.
	second.get(bookPath(deleted.ID)).assertStatus(http.StatusNotFound)

	if books := second.get("/books").assertStatus(http.StatusOK).books(); len(books) != 1 {
		t.Errorf("library contains %d books after the restart, want 1", len(books))
	}
}

// Running the migration against a database that already holds data must not
// disturb it, which is what every restart of the real binary does.
func TestMigrationOnAPopulatedDatabaseKeepsData(t *testing.T) {
	first := newApp(t)

	created := first.createBook(bookJSON("Dune", "Frank Herbert", "Spice.", 9, models.Finished))

	dbPath := first.dbPath
	first.stop()

	// startApp runs config.RunMigrations again on the existing file.
	second := startApp(t, openDB(t, dbPath), dbPath)

	survivor := second.get(bookPath(created.ID)).assertStatus(http.StatusOK).book()
	if survivor.ID != created.ID || survivor.Name != created.Name {
		t.Errorf("book after re-migration = %+v, want %+v", survivor, created)
	}
}

func TestSwaggerServesTheSpec(t *testing.T) {
	a := newApp(t)

	t.Run("ui", func(t *testing.T) {
		resp := a.get("/swagger/index.html").assertStatus(http.StatusOK)

		if contentType := resp.header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", contentType)
		}
		if !strings.Contains(string(resp.body), "swagger") {
			t.Error("the swagger UI page does not mention swagger")
		}
	})

	t.Run("spec", func(t *testing.T) {
		resp := a.get("/swagger/doc.json").assertStatus(http.StatusOK)

		var spec struct {
			Info struct {
				Title string `json:"title"`
			} `json:"info"`
			Paths map[string]map[string]any `json:"paths"`
		}
		if err := json.Unmarshal(resp.body, &spec); err != nil {
			t.Fatalf("decode the swagger spec: %v", err)
		}

		if spec.Info.Title != "bookworm-api" {
			t.Errorf("spec title = %q, want %q", spec.Info.Title, "bookworm-api")
		}

		// Every documented operation must correspond to a route that answers.
		want := map[string][]string{
			"/books":      {"get", "post"},
			"/books/{id}": {"get", "put", "delete"},
		}
		for path, methods := range want {
			operations, ok := spec.Paths[path]
			if !ok {
				t.Errorf("the spec documents no operations for %s", path)
				continue
			}
			for _, method := range methods {
				if _, ok := operations[method]; !ok {
					t.Errorf("the spec is missing %s %s", strings.ToUpper(method), path)
				}
			}
		}
	})
}

// The HTTP surface the clients actually see: headers, and the fact that a 204
// carries no body even though the handler passes gin.H{} to c.JSON.
func TestResponseContract(t *testing.T) {
	a := newApp(t)

	t.Run("json content type on success", func(t *testing.T) {
		for _, path := range []string{"/books"} {
			resp := a.get(path).assertStatus(http.StatusOK)

			if contentType := resp.header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
				t.Errorf("GET %s Content-Type = %q, want application/json", path, contentType)
			}
		}
	})

	t.Run("json content type on error", func(t *testing.T) {
		resp := a.get("/books/9999").assertStatus(http.StatusNotFound)

		if contentType := resp.header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
			t.Errorf("Content-Type = %q, want application/json", contentType)
		}
	})

	t.Run("empty collection serialises as an array", func(t *testing.T) {
		resp := a.get("/books").assertStatus(http.StatusOK)

		if body := strings.TrimSpace(string(resp.body)); body != "[]" {
			t.Errorf("body = %s, want [] rather than null", body)
		}
	})

	t.Run("no content carries no body", func(t *testing.T) {
		book := a.createBook(bookJSON("Temporary", "author", "description", 0, models.Reading))

		resp := a.delete(bookPath(book.ID)).assertStatus(http.StatusNoContent)

		if len(resp.body) != 0 {
			t.Errorf("204 response body = %q, want it empty", resp.body)
		}
	})
}

// ShouldBindJSON parses the body regardless of the declared Content-Type, so
// clients that send the wrong header still succeed. Documented here so a
// future switch to ShouldBind (which does negotiate) is a visible change.
func TestContentTypeIsNotNegotiated(t *testing.T) {
	body := bookJSON("Dune", "Frank Herbert", "Spice.", 9, models.Finished)

	for _, contentType := range []string{"", "text/plain", "application/x-www-form-urlencoded"} {
		name := contentType
		if name == "" {
			name = "absent"
		}

		t.Run(name, func(t *testing.T) {
			a := newApp(t)

			request, err := http.NewRequest(http.MethodPost, a.baseURL+"/books", strings.NewReader(body))
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if contentType != "" {
				request.Header.Set("Content-Type", contentType)
			}

			a.send(request).assertStatus(http.StatusCreated)
		})
	}
}

func TestUnroutedRequests(t *testing.T) {
	a := newApp(t)

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "unknown collection", method: http.MethodGet, path: "/authors"},
		{name: "root", method: http.MethodGet, path: "/"},
		{name: "unsupported method on a book", method: http.MethodPatch, path: "/books/1"},
		{name: "unsupported method on the collection", method: http.MethodDelete, path: "/books"},
		{name: "trailing segment", method: http.MethodGet, path: "/books/1/pages"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a.do(tc.method, tc.path, "").assertStatus(http.StatusNotFound)
		})
	}
}
