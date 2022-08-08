package handlers
import (
    "github.com/Payback159/tempido/models"
    "github.com/labstack/echo/v4"
    "net/http"
)

// GetVersion - Outputs the version of Tempido and SwaggerUI
func (c *Container) GetVersion(ctx echo.Context) error {
    return ctx.JSON(http.StatusOK, models.HelloWorld {
        Message: "Hello World",
    })
}
