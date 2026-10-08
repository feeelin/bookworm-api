package models_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/go-playground/validator/v10"

	"github.com/feeelin/bookworm-api/models"
	"github.com/feeelin/bookworm-api/validators"
)

func TestStatusConstants(t *testing.T) {
	cases := map[models.Status]string{
		models.WantToRead: "want_to_read",
		models.Reading:    "reading",
		models.Finished:   "finished",
	}

	for status, want := range cases {
		if string(status) != want {
			t.Errorf("status = %q, want %q", status, want)
		}
	}
}

func TestBookJSONTags(t *testing.T) {
	rating := int8(7)
	book := models.Book{
		Name:        "Dune",
		Author:      "Frank Herbert",
		Description: "Sand, spice and politics.",
		Rating:      &rating,
		Status:      models.Reading,
	}
	book.ID = 42

	data, err := json.Marshal(book)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal into map: %v", err)
	}

	want := map[string]any{
		"ID":          float64(42),
		"name":        "Dune",
		"author":      "Frank Herbert",
		"description": "Sand, spice and politics.",
		"rating":      float64(7),
		"status":      "reading",
	}

	for key, value := range want {
		got, ok := decoded[key]
		if !ok {
			t.Errorf("missing key %q in %s", key, data)
			continue
		}
		if got != value {
			t.Errorf("%q = %v, want %v", key, got, value)
		}
	}
}

func TestBookUnmarshalNullRating(t *testing.T) {
	var book models.Book

	body := `{"name":"n","author":"a","description":"d","rating":null,"status":"reading"}`
	if err := json.Unmarshal([]byte(body), &book); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if book.Rating != nil {
		t.Errorf("Rating = %v, want nil", *book.Rating)
	}
}

func TestBookValidation(t *testing.T) {
	validate := validator.New()
	validators.RegisterStatusValidator(validate)

	rating := func(v int8) *int8 { return &v }

	cases := []struct {
		name    string
		book    models.Book
		wantErr bool
		onField string
		onTag   string
	}{
		{
			name:    "fully populated",
			book:    models.Book{Name: "n", Author: "a", Description: "d", Rating: rating(5), Status: models.Reading},
			wantErr: false,
		},
		{
			name:    "rating is optional",
			book:    models.Book{Name: "n", Author: "a", Description: "d", Status: models.WantToRead},
			wantErr: false,
		},
		{
			name:    "rating at lower bound",
			book:    models.Book{Name: "n", Author: "a", Description: "d", Rating: rating(1), Status: models.Finished},
			wantErr: false,
		},
		{
			name:    "rating at upper bound",
			book:    models.Book{Name: "n", Author: "a", Description: "d", Rating: rating(10), Status: models.Finished},
			wantErr: false,
		},
		{
			name:    "missing name",
			book:    models.Book{Author: "a", Description: "d", Status: models.Reading},
			wantErr: true, onField: "Name", onTag: "required",
		},
		{
			name:    "missing author",
			book:    models.Book{Name: "n", Description: "d", Status: models.Reading},
			wantErr: true, onField: "Author", onTag: "required",
		},
		{
			name:    "missing description",
			book:    models.Book{Name: "n", Author: "a", Status: models.Reading},
			wantErr: true, onField: "Description", onTag: "required",
		},
		{
			name:    "missing status",
			book:    models.Book{Name: "n", Author: "a", Description: "d"},
			wantErr: true, onField: "Status", onTag: "required",
		},
		{
			name:    "rating below minimum",
			book:    models.Book{Name: "n", Author: "a", Description: "d", Rating: rating(0), Status: models.Reading},
			wantErr: true, onField: "Rating", onTag: "min",
		},
		{
			name:    "rating above maximum",
			book:    models.Book{Name: "n", Author: "a", Description: "d", Rating: rating(11), Status: models.Reading},
			wantErr: true, onField: "Rating", onTag: "max",
		},
		{
			name:    "unknown status",
			book:    models.Book{Name: "n", Author: "a", Description: "d", Status: "paused"},
			wantErr: true, onField: "Status", onTag: "validstatus",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validate.Struct(tc.book)

			if !tc.wantErr {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected a validation error")
			}

			var validationErrs validator.ValidationErrors
			if !errors.As(err, &validationErrs) {
				t.Fatalf("expected validator.ValidationErrors, got %T", err)
			}

			found := false
			for _, fieldErr := range validationErrs {
				if fieldErr.Field() == tc.onField && fieldErr.Tag() == tc.onTag {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected failure on %s/%s, got: %v", tc.onField, tc.onTag, validationErrs)
			}
		})
	}
}

// Rating(0) is the interesting case for omitempty: a pointer to the zero value
// is still "present", so the min=1 rule must fire rather than be skipped.
func TestBookZeroRatingIsNotTreatedAsEmpty(t *testing.T) {
	validate := validator.New()
	validators.RegisterStatusValidator(validate)

	zero := int8(0)
	book := models.Book{Name: "n", Author: "a", Description: "d", Rating: &zero, Status: models.Reading}

	if err := validate.Struct(book); err == nil {
		t.Fatal("expected rating 0 to be rejected by min=1")
	}
}
