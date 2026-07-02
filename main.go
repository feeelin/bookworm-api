package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"

	"github.com/feeelin/bookworm-api/config"
	"github.com/feeelin/bookworm-api/docs"
	"github.com/feeelin/bookworm-api/routes"
	"github.com/feeelin/bookworm-api/validators"
)

// @title bookworm-api
// @version 1.0
// @description Little API on Go with Gin
// @BasePath /
func main() {
	godotenv.Load()
	docs.SwaggerInfo.Host = os.Getenv("API_HOST")

	validate := validator.New()
	validators.RegisterStatusValidator(validate)

	db := config.ConnectDatabase()
	config.RunMigrations(db)

	r := gin.Default()

	routes.RegisterRoutes(r, db, validate)

	if err := r.Run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
