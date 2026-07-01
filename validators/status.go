package validators

import (
	"slices"

	"github.com/feeelin/bookworm-api/models"
	"github.com/go-playground/validator/v10"
)

func validateStatus(fl validator.FieldLevel) bool {
	value := fl.Field().String()
	status := models.Status(value)

	validStatus := []models.Status{models.WantToRead, models.Reading, models.Finished}

	return slices.Contains(validStatus, status)
}

func RegisterStatusValidator(v *validator.Validate) {
	v.RegisterValidation("validstatus", validateStatus)
}
