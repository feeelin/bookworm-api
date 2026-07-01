package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"

	"github.com/feeelin/bookworm-api/config"
	"github.com/feeelin/bookworm-api/routes"
	"github.com/feeelin/bookworm-api/validators"
)

func main() {
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
