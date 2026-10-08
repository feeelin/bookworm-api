//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/feeelin/bookworm-api/models"
)

// TestBookLifecycle walks one book through every endpoint in the order a
// client would, asserting the API and the database agree at each step.
func TestBookLifecycle(t *testing.T) {
	a := newApp(t)

	// An empty library lists nothing.
	a.get("/books").assertStatus(http.StatusOK)
	if books := a.get("/books").books(); len(books) != 0 {
		t.Fatalf("new library contains %d books, want 0", len(books))
	}

	// Create.
	created := a.createBook(bookJSON("Dune", "Frank Herbert", "Spice and politics.", 9, models.WantToRead))
	if created.ID == 0 {
		t.Fatal("created book has no ID")
	}

	// It now shows up in the collection.
	listed := a.get("/books").assertStatus(http.StatusOK).books()
	if len(listed) != 1 {
		t.Fatalf("library contains %d books, want 1", len(listed))
	}
	if listed[0].ID != created.ID {
		t.Errorf("listed ID = %d, want %d", listed[0].ID, created.ID)
	}

	// And is readable on its own URL.
	fetched := a.get(bookPath(created.ID)).assertStatus(http.StatusOK).book()
	if fetched.Name != "Dune" || fetched.Status != models.WantToRead {
		t.Errorf("fetched = %+v, want Dune/want_to_read", fetched)
	}
	if fetched.Rating == nil || *fetched.Rating != 9 {
		t.Errorf("fetched Rating = %v, want 9", fetched.Rating)
	}

	// Update: move it to "reading" and drop the rating.
	updateBody := bookJSON("Dune", "Frank Herbert", "Spice and politics.", 0, models.Reading)
	updated := a.put(bookPath(created.ID), updateBody).assertStatus(http.StatusOK).book()
	if updated.ID != created.ID {
		t.Errorf("updated ID = %d, want %d", updated.ID, created.ID)
	}
	if updated.Status != models.Reading {
		t.Errorf("updated Status = %q, want %q", updated.Status, models.Reading)
	}

	// The update is visible on a fresh read, not just in the response.
	reread := a.get(bookPath(created.ID)).assertStatus(http.StatusOK).book()
	if reread.Status != models.Reading {
		t.Errorf("re-read Status = %q, want %q", reread.Status, models.Reading)
	}
	if reread.Rating != nil {
		t.Errorf("re-read Rating = %d, want nil after the update cleared it", *reread.Rating)
	}

	// Delete.
	a.delete(bookPath(created.ID)).assertStatus(http.StatusNoContent)

	// It is gone from both the collection and its own URL.
	a.get(bookPath(created.ID)).assertStatus(http.StatusNotFound).assertErrorPayload()
	if books := a.get("/books").books(); len(books) != 0 {
		t.Errorf("library contains %d books after the delete, want 0", len(books))
	}

	// Deleting again is a 404, not a second success.
	a.delete(bookPath(created.ID)).assertStatus(http.StatusNotFound).assertErrorPayload()
}

// TestLibraryWorkflow builds a small library and mutates parts of it, checking
// that operations on one book never disturb the others.
func TestLibraryWorkflow(t *testing.T) {
	a := newApp(t)

	dune := a.createBook(bookJSON("Dune", "Frank Herbert", "Spice.", 9, models.Finished))
	goBook := a.createBook(bookJSON("The Go Programming Language", "Donovan", "A tour of Go.", 10, models.Reading))
	sicp := a.createBook(bookJSON("SICP", "Abelson", "Lisp and abstraction.", 0, models.WantToRead))

	if books := a.get("/books").books(); len(books) != 3 {
		t.Fatalf("library contains %d books, want 3", len(books))
	}

	// Finish the book that was being read.
	body := bookJSON("The Go Programming Language", "Donovan", "A tour of Go.", 8, models.Finished)
	a.put(bookPath(goBook.ID), body).assertStatus(http.StatusOK)

	// Abandon one.
	a.delete(bookPath(sicp.ID)).assertStatus(http.StatusNoContent)

	remaining := a.get("/books").assertStatus(http.StatusOK).books()
	if len(remaining) != 2 {
		t.Fatalf("library contains %d books, want 2", len(remaining))
	}

	byID := make(map[uint]models.Book, len(remaining))
	for _, book := range remaining {
		byID[book.ID] = book
	}

	if _, ok := byID[sicp.ID]; ok {
		t.Error("the deleted book is still listed")
	}

	if got := byID[goBook.ID]; got.Status != models.Finished {
		t.Errorf("updated book Status = %q, want %q", got.Status, models.Finished)
	} else if got.Rating == nil || *got.Rating != 8 {
		t.Errorf("updated book Rating = %v, want 8", got.Rating)
	}

	// The untouched book is byte-for-byte what it was created as.
	if got := byID[dune.ID]; got.Name != dune.Name || got.Status != dune.Status {
		t.Errorf("untouched book = %+v, want %+v", got, dune)
	}
}

// A delete is soft: the API stops serving the book, but the row survives with
// deleted_at set. Pinning this down matters because it is the difference
// between the HTTP contract and what is actually on disk.
func TestDeleteIsSoftInTheDatabase(t *testing.T) {
	a := newApp(t)

	book := a.createBook(bookJSON("Dune", "Frank Herbert", "Spice.", 9, models.Finished))
	a.delete(bookPath(book.ID)).assertStatus(http.StatusNoContent)

	if live := a.countRows(false); live != 0 {
		t.Errorf("%d live rows remain, want 0", live)
	}
	if all := a.countRows(true); all != 1 {
		t.Errorf("%d rows in total, want the soft-deleted row to survive", all)
	}

	var stored models.Book
	if err := a.db.Unscoped().First(&stored, book.ID).Error; err != nil {
		t.Fatalf("read the soft-deleted row: %v", err)
	}
	if !stored.DeletedAt.Valid {
		t.Error("deleted_at was not set on the row")
	}
}

func TestMissingBookReturnsNotFoundOnEveryEndpoint(t *testing.T) {
	a := newApp(t)

	body := bookJSON("n", "a", "d", 5, models.Reading)

	cases := []struct {
		method string
		body   string
	}{
		{method: http.MethodGet},
		{method: http.MethodPut, body: body},
		{method: http.MethodDelete},
	}

	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			resp := a.do(tc.method, "/books/4242", tc.body)

			resp.assertStatus(http.StatusNotFound)
			resp.assertErrorPayload()
		})
	}
}

func TestInvalidIDReturnsBadRequestOnEveryEndpoint(t *testing.T) {
	a := newApp(t)

	body := bookJSON("n", "a", "d", 5, models.Reading)

	for _, id := range []string{"abc", "-1", "1.5", "0x10"} {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			t.Run(method+"_"+id, func(t *testing.T) {
				requestBody := ""
				if method == http.MethodPut {
					requestBody = body
				}

				resp := a.do(method, "/books/"+id, requestBody)

				resp.assertStatus(http.StatusBadRequest)
				resp.assertErrorPayload()
			})
		}
	}
}

func TestValidationIsEnforcedOverHTTP(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "empty body", body: ""},
		{name: "empty object", body: `{}`},
		{name: "malformed json", body: `{"name":`},
		{name: "missing name", body: `{"author":"a","description":"d","status":"reading"}`},
		{name: "missing author", body: `{"name":"n","description":"d","status":"reading"}`},
		{name: "missing description", body: `{"name":"n","author":"a","status":"reading"}`},
		{name: "missing status", body: `{"name":"n","author":"a","description":"d"}`},
		{name: "unknown status", body: `{"name":"n","author":"a","description":"d","status":"paused"}`},
		{name: "rating too low", body: `{"name":"n","author":"a","description":"d","rating":0,"status":"reading"}`},
		{name: "rating too high", body: `{"name":"n","author":"a","description":"d","rating":11,"status":"reading"}`},
		{name: "rating wrong type", body: `{"name":"n","author":"a","description":"d","rating":"nine","status":"reading"}`},
	}

	t.Run("create", func(t *testing.T) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				a := newApp(t)

				resp := a.post("/books", tc.body)

				resp.assertStatus(http.StatusBadRequest)
				resp.assertErrorPayload()

				if rows := a.countRows(true); rows != 0 {
					t.Errorf("a rejected create wrote %d rows", rows)
				}
			})
		}
	})

	t.Run("update", func(t *testing.T) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				a := newApp(t)
				book := a.createBook(bookJSON("Original", "a", "d", 5, models.Reading))

				resp := a.put(bookPath(book.ID), tc.body)

				resp.assertStatus(http.StatusBadRequest)
				resp.assertErrorPayload()

				unchanged := a.get(bookPath(book.ID)).assertStatus(http.StatusOK).book()
				if unchanged.Name != "Original" {
					t.Errorf("a rejected update changed the book: name = %q", unchanged.Name)
				}
			})
		}
	})
}

// Concurrent writers must all be persisted, with no lost rows and no reused
// identifiers.
func TestConcurrentCreatesArePersisted(t *testing.T) {
	a := newApp(t)

	const writers = 24

	type outcome struct {
		status int
		id     uint
		body   string
	}

	outcomes := make([]outcome, writers)

	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			body := bookJSON(fmt.Sprintf("Book %02d", i), "author", "description", 5, models.Reading)
			resp := a.post("/books", body)

			outcomes[i] = outcome{status: resp.status, body: string(resp.body)}
			if resp.status == http.StatusCreated {
				outcomes[i].id = resp.book().ID
			}
		}(i)
	}
	wg.Wait()

	seen := make(map[uint]bool, writers)
	for i, got := range outcomes {
		if got.status != http.StatusCreated {
			t.Errorf("writer %d: status = %d, want %d (body: %s)", i, got.status, http.StatusCreated, got.body)
			continue
		}
		if seen[got.id] {
			t.Errorf("writer %d: ID %d was handed out twice", i, got.id)
		}
		seen[got.id] = true
	}

	if rows := a.countRows(false); rows != writers {
		t.Errorf("%d rows persisted, want %d", rows, writers)
	}
	if books := a.get("/books").assertStatus(http.StatusOK).books(); len(books) != writers {
		t.Errorf("GET /books returned %d books, want %d", len(books), writers)
	}
}

// Reads issued while writes are in flight must keep succeeding and must never
// return a partially-written book.
func TestConcurrentReadsAndWrites(t *testing.T) {
	a := newApp(t)

	seeded := a.createBook(bookJSON("Seeded", "author", "description", 7, models.Reading))

	const (
		writers = 8
		readers = 8
	)

	var wg sync.WaitGroup
	errs := make(chan error, writers+readers)

	for i := range writers {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			body := bookJSON(fmt.Sprintf("Concurrent %02d", i), "author", "description", 5, models.WantToRead)
			if resp := a.post("/books", body); resp.status != http.StatusCreated {
				errs <- fmt.Errorf("writer %d: status %d (body: %s)", i, resp.status, resp.body)
			}
		}(i)
	}

	for i := range readers {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			resp := a.get(bookPath(seeded.ID))
			if resp.status != http.StatusOK {
				errs <- fmt.Errorf("reader %d: status %d (body: %s)", i, resp.status, resp.body)
				return
			}
			if book := resp.book(); book.Name != "Seeded" || book.Author != "author" {
				errs <- fmt.Errorf("reader %d: read a torn book: %+v", i, book)
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	if rows := a.countRows(false); rows != writers+1 {
		t.Errorf("%d rows persisted, want %d", rows, writers+1)
	}
}
