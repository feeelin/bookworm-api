// Package repository implements data access for the API.
package repository

import (
	"gorm.io/gorm"

	"github.com/feeelin/bookworm-api/models"
)

// BookRepository stores books in a database.
type BookRepository struct {
	DB *gorm.DB
}

// NewBookRepository returns a BookRepository backed by db.
func NewBookRepository(db *gorm.DB) *BookRepository {
	return &BookRepository{DB: db}
}

// Create inserts book.
func (r *BookRepository) Create(book *models.Book) error {
	result := r.DB.Create(book)
	return result.Error
}

// FindAll returns all books.
func (r *BookRepository) FindAll() ([]models.Book, error) {
	var books []models.Book
	result := r.DB.Find(&books)
	return books, result.Error
}

// FindByID returns the book with the given id.
func (r *BookRepository) FindByID(id uint) (*models.Book, error) {
	var book models.Book
	result := r.DB.First(&book, id)
	return &book, result.Error
}

// Update saves changes to book.
func (r *BookRepository) Update(book *models.Book) error {
	result := r.DB.Save(book)
	return result.Error
}

// Delete removes the book with the given id, returning gorm.ErrRecordNotFound if none exists.
func (r *BookRepository) Delete(id uint) error {
	result := r.DB.Delete(&models.Book{}, id)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
