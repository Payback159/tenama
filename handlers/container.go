package handlers

import (
	"github.com/Payback159/tenama/models"
	"k8s.io/client-go/kubernetes"
)

// Container will hold all dependencies for your application.
type Container struct {
	clientset *kubernetes.Clientset
	config    *models.Config
}

// NewContainer returns an empty or an initialized container for your handlers.
func NewContainer(clientset *kubernetes.Clientset, cfg *models.Config) (*Container, error) {
	c := Container{
		clientset: clientset,
		config:    cfg,
	}
	return &c, nil
}
