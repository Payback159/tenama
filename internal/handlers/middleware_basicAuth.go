package handlers

import (
	"crypto/subtle"

	"github.com/Payback159/tenama/internal/models"
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"
)

type user struct {
	username string
	password string
}

var userList []user

func (c *Container) SetBasicAuthUserList(cfg *models.Config) {
	for _, u := range cfg.BasicAuth {
		log.Debugf("Adding user %s to basic auth list", u.Username)
		userList = append(userList, user{username: u.Username, password: u.Password})
	}
}

func (c *Container) BasicAuthValidator(username, password string, e echo.Context) (bool, error) {
	// Be careful to use constant time comparison to prevent timing attacks
	log.Debugf("Checking user %s against basic auth list", username)
	for _, u := range userList {
		log.Debugf("Checking user %s against user from request %s", u.username, username)
		if subtle.ConstantTimeCompare([]byte(username), []byte(u.username)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(u.password)) == 1 {
			return true, nil
		}
	}
	log.Warnf("User %s not found in basic auth list", username)
	return false, nil
}
