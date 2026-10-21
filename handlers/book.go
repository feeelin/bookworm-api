package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/feeelin/bookworm-api/models"
	"github.com/feeelin/bookworm-api/repository"
)

// BookHandler serves the book endpoints.
type BookHandler struct {
	Repo     *repository.BookRepository
	Validate *validator.Validate
}

// NewBookHandler returns a BookHandler using repo and validate.
func NewBookHandler(repo *repository.BookRepository, validate *validator.Validate) *BookHandler {
	return &BookHandler{Repo: repo, Validate: validate}
}

// CreateBook creates a book.
//
// @Summary Create book
// @Tags Book
// @Accept json
// @Produce json
// @Param book body models.Book true "Book data"
// @Success 201 {object} models.Book
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books [post]
func (h *BookHandler) CreateBook(c *gin.Context) {
	var book models.Book
	if err := c.ShouldBindJSON(&book); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Validate.Struct(book); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Repo.Create(&book); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, book)
}

// GetBooks lists all books.
//
// @Summary Get all books
// @Tags Book
// @Produce json
// @Success 200 {array} models.Book
// @Failure 500 {object} map[string]string
// @Router /books [get]
func (h *BookHandler) GetBooks(c *gin.Context) {
	result, err := h.Repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetBook returns a book by id.
//
// @Summary Get book by id
// @Tags Book
// @Produce json
// @Param id path int true "Book id"
// @Success 200 {object} models.Book
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id} [get]
func (h *BookHandler) GetBook(c *gin.Context) {
	id, ok := getIDFromURL(c)
	if !ok {
		return
	}

	result, err := h.Repo.FindByID(id)
	if err != nil {
		processDatabaseError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// UpdateBook updates a book by id.
//
// @Summary Update book by id
// @Tags Book
// @Accept json
// @Produce json
// @Param id path int true "Book id"
// @Param book body models.Book true "Book data"
// @Success 200 {object} models.Book
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id} [put]
func (h *BookHandler) UpdateBook(c *gin.Context) {
	id, ok := getIDFromURL(c)
	if !ok {
		return
	}

	_, err := h.Repo.FindByID(id)
	if err != nil {
		processDatabaseError(c, err)
		return
	}

	var book models.Book
	if err := c.ShouldBindJSON(&book); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	book.ID = id

	if err := h.Validate.Struct(book); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Repo.Update(&book); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, book)
}

// DeleteBook deletes a book by id.
//
// @Summary Delete book by id
// @Tags Book
// @Param id path int true "Book id"
// @Success 204 {object} nil
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id} [delete]
func (h *BookHandler) DeleteBook(c *gin.Context) {
	id, ok := getIDFromURL(c)
	if !ok {
		return
	}

	if err := h.Repo.Delete(id); err != nil {
		processDatabaseError(c, err)
		return
	}

	c.JSON(http.StatusNoContent, gin.H{})
}
