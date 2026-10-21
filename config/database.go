// Package config sets up the database connection and schema.
package config

import (
	"log"

	"github.com/feeelin/bookworm-api/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ConnectDatabase opens the SQLite database, exiting on failure.
func ConnectDatabase() *gorm.DB {
	databaseDriver := sqlite.Open("database.db")

	db, err := gorm.Open(databaseDriver, &gorm.Config{})

	if err != nil {
		log.Fatal("Database connection failed.")
	}

	return db
}

// RunMigrations migrates the database schema, exiting on failure.
func RunMigrations(db *gorm.DB) {
	if err := db.AutoMigrate(&models.Book{}); err != nil {
		log.Fatal("Migration failed.")
	}
}
