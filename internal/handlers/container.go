package handlers

import (
	"github.com/Payback159/tenama/internal/models"
	"k8s.io/client-go/kubernetes"
)

// Container will hold all dependencies for your application.
type Container struct {
	clientset *kubernetes.Clientset
	config    *models.Config
	watcher   *NamespaceWatcher
}

// NewContainer returns an empty or an initialized container for your handlers.
func NewContainer(clientset *kubernetes.Clientset, cfg *models.Config) (*Container, error) {
	c := Container{
		clientset: clientset,
		config:    cfg,
		watcher:   nil, // Will be set later via SetWatcher
	}
	return &c, nil
}

// SetWatcher sets the NamespaceWatcher for the container
func (c *Container) SetWatcher(watcher *NamespaceWatcher) {
	c.watcher = watcher
}
