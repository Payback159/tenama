package handlers

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/labstack/gommon/log"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NamespaceGetter is an interface for getting the namespace API
type NamespaceGetter interface {
	Namespaces() corev1.NamespaceInterface
}

// NamespaceWatcher manages event-based cleanup of temporary namespaces
type NamespaceWatcher struct {
	namespaceGetter NamespaceGetter
	prefix          string
	timers          map[string]*time.Timer
	mu              sync.RWMutex
	done            chan struct{}
}

// NewNamespaceWatcher creates a new watcher instance
// Accepts any NamespaceGetter (works with both real clientset and fake)
func NewNamespaceWatcher(namespaceGetter NamespaceGetter, prefix string) *NamespaceWatcher {
	return &NamespaceWatcher{
		namespaceGetter: namespaceGetter,
		prefix:          prefix,
		timers:          make(map[string]*time.Timer),
		done:            make(chan struct{}),
	}
}

// NewNamespaceWatcherFromClientset creates a watcher from a Kubernetes clientset
func NewNamespaceWatcherFromClientset(clientset *kubernetes.Clientset, prefix string) *NamespaceWatcher {
	return NewNamespaceWatcher(clientset.CoreV1(), prefix)
}

// Start begins watching namespaces
func (nw *NamespaceWatcher) Start(ctx context.Context) error {
	log.Infof("Starting namespace watcher with prefix: %s", nw.prefix)

	if err := nw.initializeExisting(ctx); err != nil {
		log.Errorf("Error initializing namespaces: %s", err)
	}

	go nw.watch(ctx)
	return nil
}

// Stop shuts down the watcher
func (nw *NamespaceWatcher) Stop() {
	log.Info("Stopping namespace watcher")
	close(nw.done)
	nw.stopAllTimers()
}

// initializeExisting schedules cleanup for existing namespaces
func (nw *NamespaceWatcher) initializeExisting(ctx context.Context) error {
	list, err := nw.namespaceGetter.Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: "created-by=tenama",
	})
	if err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	log.Debugf("Found %d existing namespaces", len(list.Items))

	for _, ns := range list.Items {
		if nw.shouldProcess(&ns) {
			nw.schedule(&ns)
		}
	}
	return nil
}

// watch observes namespace events
func (nw *NamespaceWatcher) watch(ctx context.Context) {
	watcher, err := nw.namespaceGetter.Namespaces().Watch(ctx, metav1.ListOptions{
		LabelSelector: "created-by=tenama",
	})
	if err != nil {
		log.Errorf("Error watching namespaces: %s", err)
		return
	}
	defer watcher.Stop()

	log.Info("Namespace watcher running")

	for {
		select {
		case <-nw.done:
			return
		case event, ok := <-watcher.ResultChan():
			if !ok {
				log.Warn("Watcher channel closed")
				return
			}

			ns, ok := event.Object.(*v1.Namespace)
			if !ok {
				continue
			}

			switch event.Type {
			case watch.Added:
				if nw.shouldProcess(ns) {
					nw.schedule(ns)
				}
			case watch.Deleted:
				nw.cancel(ns.Name)
			}
		}
	}
}

// shouldProcess checks if namespace should be cleaned up
func (nw *NamespaceWatcher) shouldProcess(ns *v1.Namespace) bool {
	if ns.Name == "tenama-system" {
		return false
	}

	if !hasPrefix(ns.Name, nw.prefix) {
		return false
	}

	_, ok := ns.Labels["tenama/namespace-duration"]
	return ok
}

// schedule creates a cleanup timer for a namespace
func (nw *NamespaceWatcher) schedule(ns *v1.Namespace) {
	durationStr := ns.Labels["tenama/namespace-duration"]
	duration, err := time.ParseDuration(durationStr)
	if err != nil {
		log.Errorf("Failed to parse duration for %s: %s", ns.Name, err)
		return
	}

	creationTime := ns.ObjectMeta.CreationTimestamp.Time
	expirationTime := creationTime.Add(duration)
	timeUntilExpiration := time.Until(expirationTime)

	if timeUntilExpiration <= 0 {
		log.Infof("Namespace %s already expired, deleting", ns.Name)
		nw.delete(ns.Name)
		return
	}

	nw.mu.Lock()
	if existing, ok := nw.timers[ns.Name]; ok {
		existing.Stop()
	}

	nw.timers[ns.Name] = time.AfterFunc(timeUntilExpiration, func() {
		log.Infof("Deleting namespace %s (lifetime expired)", ns.Name)
		nw.delete(ns.Name)
		nw.mu.Lock()
		delete(nw.timers, ns.Name)
		nw.mu.Unlock()
	})
	nw.mu.Unlock()

	log.Infof("Scheduled cleanup for %s in %v", ns.Name, timeUntilExpiration)
}

// cancel stops cleanup timer for a namespace
func (nw *NamespaceWatcher) cancel(namespaceName string) {
	nw.mu.Lock()
	defer nw.mu.Unlock()

	if timer, ok := nw.timers[namespaceName]; ok {
		timer.Stop()
		delete(nw.timers, namespaceName)
	}
}

// stopAllTimers stops all active timers
func (nw *NamespaceWatcher) stopAllTimers() {
	nw.mu.Lock()
	defer nw.mu.Unlock()

	for _, timer := range nw.timers {
		timer.Stop()
	}
	nw.timers = make(map[string]*time.Timer)
}

// delete removes a namespace
func (nw *NamespaceWatcher) delete(namespaceName string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := nw.namespaceGetter.Namespaces().Delete(ctx, namespaceName, metav1.DeleteOptions{})
	if err != nil {
		log.Errorf("Error deleting namespace %s: %s", namespaceName, err)
	} else {
		log.Infof("Successfully deleted namespace %s", namespaceName)
	}
}

// GetActiveTimerCount returns the number of active timers
func (nw *NamespaceWatcher) GetActiveTimerCount() int {
	nw.mu.RLock()
	defer nw.mu.RUnlock()
	return len(nw.timers)
}

// hasPrefix checks if string has prefix
func hasPrefix(s, prefix string) bool {
	if prefix == "" {
		return true
	}
	if len(s) < len(prefix) {
		return false
	}
	return s[:len(prefix)] == prefix
}
