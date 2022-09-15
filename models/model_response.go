package models

type Response struct {
	Message     string `json:"message"`
	Namespace   string `json:"namespace"`
	AccessToken string `json:"access_token,omitempty"`
}
