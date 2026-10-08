package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/feeelin/bookworm-api/config"
	"github.com/feeelin/bookworm-api/models"
)

// ConnectDatabase opens "database.db" relative to the working directory, so
// the test runs from a temporary directory to avoid touching the repository.
func TestConnectDatabase(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	db := config.ConnectDatabase()
	if db == nil {
		t.Fatal("ConnectDatabase returned nil")
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "database.db")); err != nil {
		t.Errorf("expected database.db to be created in the working directory: %v", err)
	}
}

func TestRunMigrationsCreatesBooksTable(t *testing.T) {
	db := newMemoryDB(t)

	config.RunMigrations(db)

	if !db.Migrator().HasTable(&models.Book{}) {
		t.Fatal("books table was not created")
	}

	for _, column := range []string{"id", "created_at", "updated_at", "deleted_at", "name", "author", "description", "rating", "status"} {
		if !db.Migrator().HasColumn(&models.Book{}, column) {
			t.Errorf("column %q is missing from the books table", column)
		}
	}
}

func TestRunMigrationsIsIdempotent(t *testing.T) {
	db := newMemoryDB(t)

	config.RunMigrations(db)
	config.RunMigrations(db)

	if !db.Migrator().HasTable(&models.Book{}) {
		t.Fatal("books table is missing after a second migration")
	}
}

// After migrating, a book must round-trip through the schema that was created.
func TestRunMigrationsProducesAUsableSchema(t *testing.T) {
	db := newMemoryDB(t)
	config.RunMigrations(db)

	rating := int8(8)
	book := models.Book{
		Name:        "Dune",
		Author:      "Frank Herbert",
		Description: "Spice.",
		Rating:      &rating,
		Status:      models.Finished,
	}

	if err := db.Create(&book).Error; err != nil {
		t.Fatalf("insert into the migrated schema: %v", err)
	}

	var stored models.Book
	if err := db.First(&stored, book.ID).Error; err != nil {
		t.Fatalf("read back: %v", err)
	}
	if stored.Name != book.Name || stored.Status != book.Status {
		t.Errorf("stored book = %+v, want %+v", stored, book)
	}
	if stored.Rating == nil || *stored.Rating != rating {
		t.Errorf("stored Rating = %v, want %d", stored.Rating, rating)
	}
}

func newMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	})

	return db
}
