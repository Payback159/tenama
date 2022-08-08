package main

import (
	"crypto/subtle"

	"github.com/Payback159/tempido/handlers"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	e := echo.New()
	ag := e.Group("/namespace")

	//todo: handle the error!
	c, _ := handlers.NewContainer()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	ag.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
		// Be careful to use constant time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(username), []byte("admin")) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte("admin")) == 1 {
			return true, nil
		}
		return false, nil
	}))

	// GetVersion - Outputs the version of Tempido
	e.Static("/version", "docs/swagger/")
	e.Static("/", "docs/swagger/")

	// CreateNamespace - Create a new namespace
	ag.POST("/namespace", c.CreateNamespace)

	// DeleteNamespace - Deletes a namespace
	ag.DELETE("/namespace/:namespace", c.DeleteNamespace)

	// GetNamespaceByName - Find namespace by name
	ag.GET("/namespace/:namespace", c.GetNamespaceByName)

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
