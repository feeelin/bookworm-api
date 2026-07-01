package repository

import (
	"gorm.io/gorm"

	"github.com/feeelin/bookworm-api/models"
)

type BookRepository struct {
	DB *gorm.DB
}

func NewBookRepository(db *gorm.DB) *BookRepository {
	return &BookRepository{DB: db}
}

func (r *BookRepository) Create(book *models.Book) error {
	result := r.DB.Create(book)
	return result.Error
}

func (r *BookRepository) FindAll() ([]models.Book, error) {
	var books []models.Book
	result := r.DB.Find(&books)
	return books, result.Error
}

func (r *BookRepository) FindByID(id uint) (*models.Book, error) {
	var book models.Book
	result := r.DB.First(&book, id)
	return &book, result.Error
}

func (r *BookRepository) Update(book *models.Book) error {
	result := r.DB.Save(book)
	return result.Error
}

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
