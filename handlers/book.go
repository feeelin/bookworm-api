package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/feeelin/bookworm-api/models"
	"github.com/feeelin/bookworm-api/repository"
)

type BookHandler struct {
	Repo     *repository.BookRepository
	Validate *validator.Validate
}

func NewBookHandler(repo *repository.BookRepository, validate *validator.Validate) *BookHandler {
	return &BookHandler{Repo: repo, Validate: validate}
}

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

func (h *BookHandler) GetBooks(c *gin.Context) {
	result, err := h.Repo.FindAll()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

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
