package handlers

import (
	"crypto/subtle"

	"github.com/Payback159/tenama/models"
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
	for _, u := range userList {
		log.Debugf("Checking user %s against basic auth list", u.username)
		if subtle.ConstantTimeCompare([]byte(username), []byte(u.username)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(u.password)) == 1 {
			return true, nil
		}
	}
	log.Warn("User %s not found in basic auth list", username)
	return false, nil
}
