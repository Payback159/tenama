package handlers

import (
	"net/http"

	"github.com/Payback159/tenama/internal/models"
	"github.com/labstack/echo/v4"
	v1 "k8s.io/api/core/v1"
)

var version = "development"
var builddate = "unknown"
var commit = "unknown"

func (c *Container) GetBuildInfo(e echo.Context) error {
	response := models.GetInfo200Response{
		Version:   version,
		BuildDate: builddate,
		Commit:    commit,
	}

	// Add GlobalLimits status if watcher is available
	if c.watcher != nil {
		currentUsage := c.watcher.GetCurrentResourceUsage()
		globalLimits := c.watcher.GetGlobalLimits()

		// Check if any limits are configured
		isEnabled := len(globalLimits) > 0

		response.GlobalLimits = &models.GlobalLimitsStatus{
			Enabled:      isEnabled,
			CurrentUsage: quantityMapToStrings(currentUsage),
			Limits:       quantityMapToStrings(globalLimits),
		}
	}

	return e.JSON(http.StatusOK, response)
}

// Helper function to convert ResourceList to map[string]string
// ResourceList is map[ResourceName]Quantity
func quantityMapToStrings(resources v1.ResourceList) map[string]string {
	result := make(map[string]string)
	for key, quantity := range resources {
		// key is a ResourceName like "cpu", "memory", "storage"
		// quantity is a *resource.Quantity
		result[key.String()] = quantity.String()
	}
	return result
}
