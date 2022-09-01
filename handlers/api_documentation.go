package handlers
import (
    "github.com/Payback159/tenama/models"
    "github.com/labstack/echo/v4"
    "net/http"
)

// GetDocumentation - Outputs the openAPI specification
func (c *Container) GetDocumentation(ctx echo.Context) error {
    return ctx.JSON(http.StatusOK, models.HelloWorld {
        Message: "Hello World",
    })
}
