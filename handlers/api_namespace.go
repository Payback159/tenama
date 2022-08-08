package handlers
import (
    "github.com/Payback159/tempido/models"
    "github.com/labstack/echo/v4"
    "net/http"
)

// CreateNamespace - Create a new namespace
func (c *Container) CreateNamespace(ctx echo.Context) error {
    return ctx.JSON(http.StatusOK, models.HelloWorld {
        Message: "Hello World",
    })
}


// DeleteNamespace - Deletes a namespace
func (c *Container) DeleteNamespace(ctx echo.Context) error {
    return ctx.JSON(http.StatusOK, models.HelloWorld {
        Message: "Hello World",
    })
}


// GetNamespaceByName - Find namespace by name
func (c *Container) GetNamespaceByName(ctx echo.Context) error {
    return ctx.JSON(http.StatusOK, models.HelloWorld {
        Message: "Hello World",
    })
}
