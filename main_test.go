package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestGetVersionReturnsTheBuildInformation(t *testing.T) {
	router := gin.New()
	router.GET("/version", getVersion)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/version", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode the response: %v", err)
	}

	want := map[string]string{
		"version":   version,
		"commit":    commit,
		"buildTime": buildTime,
	}
	for field, value := range want {
		got, ok := body[field]
		if !ok {
			t.Errorf("the response has no %q field: %s", field, recorder.Body)
			continue
		}
		if got != value {
			t.Errorf("%s = %q, want %q", field, got, value)
		}
	}

	if len(body) != len(want) {
		t.Errorf("the response has %d fields, want %d: %s", len(body), len(want), recorder.Body)
	}
}

// The default values are what an un-stamped build reports; -ldflags replaces
// them, so a change here is a change to the binary's contract.
func TestBuildInformationDefaults(t *testing.T) {
	if version != "dev" || commit != "none" || buildTime != "unknown" {
		t.Errorf("defaults = %q/%q/%q, want dev/none/unknown", version, commit, buildTime)
	}
}

func TestPrintVersionWritesThreeLines(t *testing.T) {
	var out bytes.Buffer
	printVersion(&out)

	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("printVersion wrote %d lines, want 3: %q", len(lines), out.String())
	}

	for i, want := range []string{"version: " + version, "commit: " + commit, "built: " + buildTime} {
		if lines[i] != want {
			t.Errorf("line %d = %q, want %q", i+1, lines[i], want)
		}
	}
}
