package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// newContext builds a gin context whose "id" path parameter is set to raw.
func newContext(raw string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/books/:id", nil)
	c.Params = gin.Params{{Key: "id", Value: raw}}

	return c, recorder
}

func TestGetIDFromURL(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want uint
	}{
		{name: "single digit", raw: "1", want: 1},
		{name: "multiple digits", raw: "12345", want: 12345},
		{name: "zero", raw: "0", want: 0},
		{name: "leading zeros", raw: "007", want: 7},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newContext(tc.raw)

			id, ok := getIDFromURL(c)

			if !ok {
				t.Fatalf("ok = false, want true (body: %s)", recorder.Body.String())
			}
			if id != tc.want {
				t.Errorf("id = %d, want %d", id, tc.want)
			}
			if recorder.Body.Len() != 0 {
				t.Errorf("expected no response to be written, got %s", recorder.Body.String())
			}
		})
	}
}

func TestGetIDFromURLRejectsInvalidValues(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{name: "empty", raw: ""},
		{name: "not a number", raw: "abc"},
		{name: "negative", raw: "-1"},
		{name: "float", raw: "1.5"},
		{name: "trailing space", raw: "1 "},
		{name: "hex", raw: "0x1"},
		{name: "overflow", raw: "99999999999999999999999"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, recorder := newContext(tc.raw)

			id, ok := getIDFromURL(c)

			if ok {
				t.Fatalf("ok = true for %q, want false", tc.raw)
			}
			if id != 0 {
				t.Errorf("id = %d, want 0 on failure", id)
			}
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
			}
			if !hasErrorField(t, recorder.Body.Bytes()) {
				t.Errorf("body %s has no \"error\" field", recorder.Body.String())
			}
		})
	}
}

func TestProcessDatabaseError(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "record not found maps to 404",
			err:        gorm.ErrRecordNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "wrapped record not found maps to 404",
			err:        errors.Join(errors.New("query books"), gorm.ErrRecordNotFound),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "any other error maps to 500",
			err:        errors.New("connection reset"),
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "gorm invalid transaction maps to 500",
			err:        gorm.ErrInvalidTransaction,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)

			processDatabaseError(c, tc.err)

			if recorder.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
			if body := recorder.Body.String(); !hasErrorField(t, recorder.Body.Bytes()) {
				t.Errorf("body %s has no \"error\" field", body)
			}
		})
	}
}

func hasErrorField(t *testing.T, body []byte) bool {
	t.Helper()

	var payload struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}

	return payload.Error != ""
}
