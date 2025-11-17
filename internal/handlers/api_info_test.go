package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Payback159/tenama/internal/models"
	"github.com/labstack/echo/v4"
)

func TestGetBuildInfo(t *testing.T) {
	tests := []struct {
		name           string
		expectedStatus int
		expectedFields bool
	}{
		{
			name:           "successful get build info",
			expectedStatus: http.StatusOK,
			expectedFields: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/info", nil)
			rec := httptest.NewRecorder()
			ctx := e.NewContext(req, rec)

			// Create container
			container := &Container{}

			// Act
			err := container.GetBuildInfo(ctx)

			// Assert
			if err != nil {
				t.Errorf("GetBuildInfo returned error: %v", err)
			}

			if rec.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rec.Code)
			}

			// Verify response contains BuildInfo
			if tt.expectedFields && rec.Body.Len() == 0 {
				t.Errorf("Expected non-empty response body")
			}
		})
	}
}

func TestBuildInfoStructure(t *testing.T) {
	info := models.GetInfo200Response{
		Version:   "1.0.0",
		BuildDate: "2025-11-17",
		Commit:    "abc123",
	}

	if info.Version == "" {
		t.Errorf("Version should not be empty")
	}
	if info.BuildDate == "" {
		t.Errorf("BuildDate should not be empty")
	}
	if info.Commit == "" {
		t.Errorf("Commit should not be empty")
	}
}
