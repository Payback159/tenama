package handlers

// liveness and readiness probes for kubernetes

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

var version = "development"
var builddate = "unknown"
var commit = "unknown"

func (c *Container) GetBuildInfo(e echo.Context) error {
	return e.JSON(http.StatusOK, map[string]string{
		"version":   version,
		"buildDate": builddate,
		"commit":    commit,
	})
}
