package models

type GetNamespaces200Response struct {

	Message string `json:"message,omitempty"`

	Namespaces []string `json:"namespaces,omitempty"`
}
