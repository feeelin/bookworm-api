package config

import (
	"log"

	"github.com/feeelin/bookworm-api/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func ConnectDatabase() *gorm.DB {
	databaseDriver := sqlite.Open("database.db")

	db, err := gorm.Open(databaseDriver, &gorm.Config{})

	if err != nil {
		log.Fatal("Database connection failed.")
	}

	return db
}

func RunMigrations(db *gorm.DB) {
	if err := db.AutoMigrate(&models.Book{}); err != nil {
		log.Fatal("Migration failed.")
	}
}
