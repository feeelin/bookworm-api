package models

import (
	"gorm.io/gorm"
)

type Status string

const (
	WantToRead Status = "want_to_read"
	Reading    Status = "reading"
	Finished   Status = "finished"
)

type Book struct {
	gorm.Model

	Name        string `json:"name"        validate:"required"`
	Author      string `json:"author"      validate:"required"`
	Description string `json:"description" validate:"required"`
	Rating      *int8  `json:"rating"      validate:"omitempty,min=1,max=10"`
	Status      Status `json:"status"      validate:"required,validstatus"`
}
