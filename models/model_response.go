package models

type Response struct {
	Message    string   `json:"message"`
	Namespace  string   `json:"namespace"`
	Namespaces []string `json:"namespaces, omitempty"`
	KubeConfig []byte   `json:"kubeconfig,omitempty"`
}
