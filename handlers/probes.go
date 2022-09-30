package handlers

// liveness and readiness probes for kubernetes

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func (c *Container) LivenessProbe(e echo.Context) error {
	return e.String(http.StatusOK, "OK")
}

func (c *Container) ReadinessProbe(e echo.Context) error {
	return e.String(http.StatusOK, "OK")
}
