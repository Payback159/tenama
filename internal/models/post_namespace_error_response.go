package models

type PostNamespaceErrorResponse struct {
	Message   string `json:"message"`
	Namespace string `json:"namespace,omitempty"`
}
