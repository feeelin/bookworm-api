package validators_test

import (
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/feeelin/bookworm-api/models"
	"github.com/feeelin/bookworm-api/validators"
)

type statusHolder struct {
	Status models.Status `validate:"validstatus"`
}

func newValidator(t *testing.T) *validator.Validate {
	t.Helper()

	v := validator.New()
	if err := validators.RegisterStatusValidator(v); err != nil {
		t.Fatalf("register status validator: %v", err)
	}

	return v
}

func TestValidStatusAcceptsKnownStatuses(t *testing.T) {
	v := newValidator(t)

	statuses := []models.Status{models.WantToRead, models.Reading, models.Finished}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			if err := v.Struct(statusHolder{Status: status}); err != nil {
				t.Fatalf("expected %q to be valid, got: %v", status, err)
			}
		})
	}
}

func TestValidStatusRejectsUnknownStatuses(t *testing.T) {
	v := newValidator(t)

	cases := map[string]models.Status{
		"empty":        "",
		"unknown word": "abandoned",
		"wrong case":   "Reading",
		"whitespace":   " reading",
		"numeric":      "1",
	}

	for name, status := range cases {
		t.Run(name, func(t *testing.T) {
			err := v.Struct(statusHolder{Status: status})
			if err == nil {
				t.Fatalf("expected %q to be rejected", status)
			}

			var validationErrs validator.ValidationErrors
			if !errors.As(err, &validationErrs) {
				t.Fatalf("expected validation errors, got %T: %v", err, err)
			}

			if tag := validationErrs[0].Tag(); tag != "validstatus" {
				t.Errorf("failed on tag %q, want %q", tag, "validstatus")
			}
		})
	}
}

func TestRegisterStatusValidatorIsRequiredForTheTag(t *testing.T) {
	// Without registration the tag is unknown and validator panics, which is
	// what makes RegisterStatusValidator a hard requirement at startup.
	v := validator.New()

	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for the unregistered validstatus tag")
		}
	}()

	_ = v.Struct(statusHolder{Status: models.Reading})
}

func TestRegisterStatusValidatorIsIdempotent(t *testing.T) {
	v := validator.New()
	if err := validators.RegisterStatusValidator(v); err != nil {
		t.Fatalf("register status validator: %v", err)
	}
	if err := validators.RegisterStatusValidator(v); err != nil {
		t.Fatalf("register status validator: %v", err)
	}

	if err := v.Struct(statusHolder{Status: models.Reading}); err != nil {
		t.Fatalf("expected valid status after double registration, got: %v", err)
	}
}
