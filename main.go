package main

import (
	"github.com/Payback159/tempido/handlers"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	e := echo.New()

	//todo: handle the error!
	c, _ := handlers.NewContainer()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// GetVersion - Outputs the version of Tempido
	e.Static("/version", "static/swagger/")
	e.Static("/", "static/swagger/")

	// CreateNamespace - Create a new namespace
	e.POST("/namespace", c.CreateNamespace)

	// DeleteNamespace - Deletes a namespace
	e.DELETE("/namespace/:namespace", c.DeleteNamespace)

	// GetNamespaceByName - Find namespace by name
	e.GET("/namespace/:namespace", c.GetNamespaceByName)

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}
