package repository_test

import (
	"errors"
	"testing"

	"gorm.io/gorm"

	"github.com/feeelin/bookworm-api/internal/testutil"
	"github.com/feeelin/bookworm-api/models"
	"github.com/feeelin/bookworm-api/repository"
)

func TestNewBookRepository(t *testing.T) {
	db := testutil.NewDB(t)

	repo := repository.NewBookRepository(db)

	if repo == nil {
		t.Fatal("NewBookRepository returned nil")
	}
	if repo.DB != db {
		t.Error("repository does not hold the database it was built with")
	}
}

func TestCreate(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	book := testutil.NewBook()
	if err := repo.Create(book); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if book.ID == 0 {
		t.Error("Create did not populate the generated ID")
	}
	if book.CreatedAt.IsZero() {
		t.Error("Create did not populate CreatedAt")
	}

	stored, err := repo.FindByID(book.ID)
	if err != nil {
		t.Fatalf("FindByID after Create: %v", err)
	}
	if stored.Name != book.Name || stored.Status != book.Status {
		t.Errorf("stored book = %+v, want name %q and status %q", stored, book.Name, book.Status)
	}
}

func TestCreateWithoutRating(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	book := testutil.NewBook(func(b *models.Book) { b.Rating = nil })
	if err := repo.Create(book); err != nil {
		t.Fatalf("Create: %v", err)
	}

	stored, err := repo.FindByID(book.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if stored.Rating != nil {
		t.Errorf("Rating = %d, want nil", *stored.Rating)
	}
}

func TestCreateError(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewBrokenDB(t))

	if err := repo.Create(testutil.NewBook()); err == nil {
		t.Fatal("expected an error from a closed database")
	}
}

func TestFindAll(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	names := []string{"First", "Second", "Third"}
	for _, name := range names {
		book := testutil.NewBook(func(b *models.Book) { b.Name = name })
		if err := repo.Create(book); err != nil {
			t.Fatalf("Create %q: %v", name, err)
		}
	}

	books, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(books) != len(names) {
		t.Fatalf("got %d books, want %d", len(books), len(names))
	}

	for i, name := range names {
		if books[i].Name != name {
			t.Errorf("books[%d].Name = %q, want %q", i, books[i].Name, name)
		}
	}
}

func TestFindAllOnEmptyTable(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	books, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(books) != 0 {
		t.Errorf("got %d books, want 0", len(books))
	}
}

func TestFindAllSkipsSoftDeletedRows(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	kept := testutil.NewBook(func(b *models.Book) { b.Name = "Kept" })
	removed := testutil.NewBook(func(b *models.Book) { b.Name = "Removed" })

	for _, book := range []*models.Book{kept, removed} {
		if err := repo.Create(book); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	if err := repo.Delete(removed.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	books, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(books) != 1 || books[0].Name != "Kept" {
		t.Errorf("FindAll = %+v, want only the kept book", books)
	}
}

func TestFindAllError(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewBrokenDB(t))

	books, err := repo.FindAll()
	if err == nil {
		t.Fatal("expected an error from a closed database")
	}
	if books != nil {
		t.Errorf("books = %+v, want nil on error", books)
	}
}

func TestFindByIDNotFound(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	_, err := repo.FindByID(404)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("err = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestFindByIDOfSoftDeletedRow(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	book := testutil.NewBook()
	if err := repo.Create(book); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(book.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := repo.FindByID(book.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("err = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestUpdate(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	book := testutil.NewBook()
	if err := repo.Create(book); err != nil {
		t.Fatalf("Create: %v", err)
	}

	book.Name = "Renamed"
	book.Status = models.WantToRead
	book.Rating = nil

	if err := repo.Update(book); err != nil {
		t.Fatalf("Update: %v", err)
	}

	stored, err := repo.FindByID(book.ID)
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if stored.Name != "Renamed" {
		t.Errorf("Name = %q, want %q", stored.Name, "Renamed")
	}
	if stored.Status != models.WantToRead {
		t.Errorf("Status = %q, want %q", stored.Status, models.WantToRead)
	}
	if stored.Rating != nil {
		t.Errorf("Rating = %d, want nil", *stored.Rating)
	}
}

// Save inserts when the primary key is unset, which is worth pinning down
// because the handler relies on Update only ever touching existing rows.
func TestUpdateWithoutIDInserts(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	book := testutil.NewBook()
	if err := repo.Update(book); err != nil {
		t.Fatalf("Update: %v", err)
	}

	if book.ID == 0 {
		t.Fatal("expected Save to assign an ID")
	}

	books, err := repo.FindAll()
	if err != nil {
		t.Fatalf("FindAll: %v", err)
	}
	if len(books) != 1 {
		t.Errorf("got %d books, want 1", len(books))
	}
}

func TestUpdateError(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewBrokenDB(t))

	book := testutil.NewBook()
	book.ID = 1

	if err := repo.Update(book); err == nil {
		t.Fatal("expected an error from a closed database")
	}
}

func TestDelete(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	book := testutil.NewBook()
	if err := repo.Create(book); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(book.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := repo.FindByID(book.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("book still readable after Delete: %v", err)
	}
}

func TestDeleteMissingRowReportsNotFound(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	if err := repo.Delete(999); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("err = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestDeleteTwiceReportsNotFound(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewDB(t))

	book := testutil.NewBook()
	if err := repo.Create(book); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(book.ID); err != nil {
		t.Fatalf("first Delete: %v", err)
	}

	if err := repo.Delete(book.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("second Delete err = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestDeleteError(t *testing.T) {
	repo := repository.NewBookRepository(testutil.NewBrokenDB(t))

	err := repo.Delete(1)
	if err == nil {
		t.Fatal("expected an error from a closed database")
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		t.Error("a connection failure must not be reported as a missing record")
	}
}
