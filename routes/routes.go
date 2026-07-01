package routes

import (
	"github.com/feeelin/bookworm-api/handlers"
	"github.com/feeelin/bookworm-api/repository"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

func RegisterRoutes(r *gin.Engine, db *gorm.DB, validate *validator.Validate) {
	registerBookRoutes(r, db, validate)
}

func registerBookRoutes(r *gin.Engine, db *gorm.DB, validate *validator.Validate) {
	repo := repository.NewBookRepository(db)
	handler := handlers.NewBookHandler(repo, validate)
	group := r.Group("/books")

	group.POST("", handler.CreateBook)
	group.GET("", handler.GetBooks)
	group.GET("/:id", handler.GetBook)
	group.PUT("/:id", handler.UpdateBook)
	group.DELETE("/:id", handler.DeleteBook)
}
