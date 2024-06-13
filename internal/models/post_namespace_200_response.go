package models

type PostNamespace200Response struct {
	Message    string   `json:"message"`
	Namespace  string   `json:"namespace,omitempty"`
	Namespaces []string `json:"namespaces,omitempty"`
	KubeConfig []byte   `json:"kubeconfig,omitempty"`
}
