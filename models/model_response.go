package models

type Response struct {
	Message    string `json:"message"`
	Namespace  string `json:"namespace"`
	KubeConfig []byte `json:"kubeconfig,omitempty"`
}
