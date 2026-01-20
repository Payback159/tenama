package handlers

import (
	"crypto/subtle"
	"log/slog"

	"github.com/Payback159/tenama/internal/models"
	"github.com/labstack/echo/v4"
)

type user struct {
	username string
	password string
}

var userList []user

func (c *Container) SetBasicAuthUserList(cfg *models.Config) {
	for _, u := range cfg.BasicAuth {
		slog.Debug("Adding user to basic auth list", "username", u.Username)
		userList = append(userList, user{username: u.Username, password: u.Password})
	}
}

func (c *Container) BasicAuthValidator(username, password string, e echo.Context) (bool, error) {
	// Be careful to use constant time comparison to prevent timing attacks
	slog.Debug("Checking user against basic auth list", "username", username)
	for _, u := range userList {
		slog.Debug("Checking against user from list", "listUser", u.username, "requestUser", username)
		if subtle.ConstantTimeCompare([]byte(username), []byte(u.username)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(u.password)) == 1 {
			return true, nil
		}
	}
	slog.Warn("User not found in basic auth list", "username", username)
	return false, nil
}
