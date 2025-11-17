package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Payback159/tenama/internal/models"
	"github.com/labstack/echo/v4"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/fake"
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

func TestGetBuildInfoWithGlobalLimits(t *testing.T) {
	tests := []struct {
		name          string
		withWatcher   bool
		globalLimits  v1.ResourceList
		currentUsage  v1.ResourceList
		expectEnabled bool
	}{
		{
			name:          "no watcher - no global limits info",
			withWatcher:   false,
			expectEnabled: false,
		},
		{
			name:        "watcher with global limits",
			withWatcher: true,
			globalLimits: v1.ResourceList{
				v1.ResourceCPU:     *resource.NewMilliQuantity(5000, resource.DecimalSI),
				v1.ResourceMemory:  *resource.NewQuantity(10*1024*1024*1024, resource.BinarySI),
				v1.ResourceStorage: *resource.NewQuantity(50*1024*1024*1024, resource.BinarySI),
			},
			currentUsage: v1.ResourceList{
				v1.ResourceCPU:     *resource.NewMilliQuantity(1000, resource.DecimalSI),
				v1.ResourceMemory:  *resource.NewQuantity(2*1024*1024*1024, resource.BinarySI),
				v1.ResourceStorage: *resource.NewQuantity(5*1024*1024*1024, resource.BinarySI),
			},
			expectEnabled: true,
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

			// Add watcher with global limits if needed
			if tt.withWatcher {
				fakeCS := fake.NewSimpleClientset()
				watcher := NewNamespaceWatcher(fakeCS.CoreV1(), "test-")
				watcher.SetGlobalLimits(tt.globalLimits)
				// Add some current usage
				for name, quantity := range tt.currentUsage {
					watcher.currentUsage[name] = quantity
				}
				container.watcher = watcher
			}

			// Act
			err := container.GetBuildInfo(ctx)

			// Assert
			if err != nil {
				t.Errorf("GetBuildInfo returned error: %v", err)
			}

			if rec.Code != http.StatusOK {
				t.Errorf("Expected status %d, got %d", http.StatusOK, rec.Code)
			}

			// Parse response
			var response models.GetInfo200Response
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Errorf("Failed to parse response: %v", err)
				return
			}

			// Verify GlobalLimits presence
			if tt.withWatcher {
				if response.GlobalLimits == nil {
					t.Errorf("Expected GlobalLimits to be present")
					return
				}

				if response.GlobalLimits.Enabled != tt.expectEnabled {
					t.Errorf("Expected Enabled=%v, got %v", tt.expectEnabled, response.GlobalLimits.Enabled)
				}

				// Verify usage and limits are populated
				if len(response.GlobalLimits.CurrentUsage) == 0 {
					t.Errorf("Expected CurrentUsage to be populated")
				}
				if len(response.GlobalLimits.Limits) == 0 {
					t.Errorf("Expected Limits to be populated")
				}

				// Verify resource quantities are correct
				if usage, ok := response.GlobalLimits.CurrentUsage["cpu"]; !ok || usage == "" {
					t.Errorf("Expected CPU in CurrentUsage")
				}
				if limit, ok := response.GlobalLimits.Limits["cpu"]; !ok || limit == "" {
					t.Errorf("Expected CPU in Limits")
				}
			} else {
				if response.GlobalLimits != nil {
					t.Errorf("Expected GlobalLimits to be nil when watcher not present")
				}
			}
		})
	}
}
