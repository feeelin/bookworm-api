package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"

	"github.com/feeelin/bookworm-api/config"
	"github.com/feeelin/bookworm-api/docs"
	"github.com/feeelin/bookworm-api/routes"
	"github.com/feeelin/bookworm-api/validators"
)

// Build information, overridden at link time with
// -ldflags "-X main.version=... -X main.commit=... -X main.buildTime=...".
var (
	version   = "dev"
	commit    = "none"
	buildTime = "unknown"
)

// printVersion writes the build information the binary was stamped with.
func printVersion(w io.Writer) {
	_, _ = fmt.Fprintf(w, "version: %s\n", version)
	_, _ = fmt.Fprintf(w, "commit: %s\n", commit)
	_, _ = fmt.Fprintf(w, "built: %s\n", buildTime)
}

// getVersion reports the build information of the running binary.
func getVersion(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"version":   version,
		"commit":    commit,
		"buildTime": buildTime,
	})
}

// @title bookworm-api
// @version 1.0
// @description Little API on Go with Gin
// @BasePath /
func main() {
	showVersion := flag.Bool("version", false, "print build information and exit")
	flag.Parse()

	if *showVersion {
		printVersion(os.Stdout)
		return
	}

	// A missing .env file is fine: configuration may come from the environment.
	_ = godotenv.Load()
	docs.SwaggerInfo.Host = os.Getenv("API_HOST")

	validate := validator.New()
	if err := validators.RegisterStatusValidator(validate); err != nil {
		log.Fatalf("failed to register validators: %v", err)
	}

	db := config.ConnectDatabase()
	config.RunMigrations(db)

	r := gin.Default()

	r.GET("/version", getVersion)
	routes.RegisterRoutes(r, db, validate)

	if err := r.Run(); err != nil {
		log.Fatalf("failed to run server: %v", err)
	}
}
