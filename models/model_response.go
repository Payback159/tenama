package models

type Response struct {
	Message    string `json:"message"`
	Namespace  string `json:"namespace"`
	KubeConfig string `json:"kubeconfig,omitempty"`
}
