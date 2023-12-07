package models

type GetInfo200Response struct {

	BuildDate string `json:"buildDate,omitempty"`

	Commit string `json:"commit,omitempty"`

	Version string `json:"version,omitempty"`
}
