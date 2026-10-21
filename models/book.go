// Package models defines the domain types persisted by the API.
package models

import (
	"gorm.io/gorm"
)

// Status is the reading state of a book.
type Status string

// Allowed values of Status.
const (
	WantToRead Status = "want_to_read"
	Reading    Status = "reading"
	Finished   Status = "finished"
)

// Book is a book tracked by the API.
type Book struct {
	gorm.Model

	Name        string `json:"name"        validate:"required"`
	Author      string `json:"author"      validate:"required"`
	Description string `json:"description" validate:"required"`
	Rating      *int8  `json:"rating"      validate:"omitempty,min=1,max=10"`
	Status      Status `json:"status"      validate:"required,validstatus"`
}
