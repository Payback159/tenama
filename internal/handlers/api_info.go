package handlers

import (
	"net/http"

	"github.com/Payback159/tenama/internal/models"
	"github.com/labstack/echo/v4"
)

var version = "development"
var builddate = "unknown"
var commit = "unknown"

func (c *Container) GetBuildInfo(e echo.Context) error {
	return e.JSON(http.StatusOK, models.GetInfo200Response{
		Version:   version,
		BuildDate: builddate,
		Commit:    commit,
	})
}
