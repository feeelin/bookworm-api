// Package testutil provides helpers shared by the test suites of this module.
package testutil

import (
	"fmt"
	"testing"

	"github.com/go-playground/validator/v10"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/feeelin/bookworm-api/models"
	"github.com/feeelin/bookworm-api/validators"
)

// NewDB returns a migrated, isolated in-memory SQLite database. Each call gets
// its own shared-cache name so parallel tests never see each other's rows.
func NewDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", sanitize(t.Name()))

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Discard,
	})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}

	if err := db.AutoMigrate(&models.Book{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

// NewBrokenDB returns a database handle whose connection pool is already
// closed, so every query made through it fails. It is used to exercise the
// error branches that a healthy database never reaches.
func NewBrokenDB(t *testing.T) *gorm.DB {
	t.Helper()

	db := NewDB(t)

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql.DB: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql.DB: %v", err)
	}

	return db
}

// NewValidator returns a validator with the application's custom rules
// registered, matching what main.go wires up.
func NewValidator(t *testing.T) *validator.Validate {
	t.Helper()

	validate := validator.New()
	if err := validators.RegisterStatusValidator(validate); err != nil {
		t.Fatalf("register status validator: %v", err)
	}

	return validate
}

// Rating is a convenience helper for the *int8 field on models.Book.
func Rating(value int8) *int8 {
	return &value
}

// NewBook builds a valid book, applying the given mutators before returning it.
func NewBook(mutators ...func(*models.Book)) *models.Book {
	book := &models.Book{
		Name:        "The Go Programming Language",
		Author:      "Donovan & Kernighan",
		Description: "A thorough tour of Go.",
		Rating:      Rating(9),
		Status:      models.Finished,
	}

	for _, mutate := range mutators {
		mutate(book)
	}

	return book
}

// sanitize turns a test name into something safe for a SQLite DSN.
func sanitize(name string) string {
	out := make([]rune, 0, len(name))
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
