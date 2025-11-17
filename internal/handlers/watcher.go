package handlers

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/labstack/gommon/log"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
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
// and tracks global resource usage across all managed namespaces
type NamespaceWatcher struct {
	namespaceGetter NamespaceGetter
	prefix          string
	timers          map[string]*time.Timer
	mu              sync.RWMutex
	done            chan struct{}

	// Global resource tracking
	currentUsage v1.ResourceList
	globalLimits v1.ResourceList
	resourceMu   sync.RWMutex
	nsResources  map[string]v1.ResourceList // Track resources per namespace
}

// NewNamespaceWatcher creates a new watcher instance
// Accepts any NamespaceGetter (works with both real clientset and fake)
func NewNamespaceWatcher(namespaceGetter NamespaceGetter, prefix string) *NamespaceWatcher {
	return &NamespaceWatcher{
		namespaceGetter: namespaceGetter,
		prefix:          prefix,
		timers:          make(map[string]*time.Timer),
		done:            make(chan struct{}),
		currentUsage:    make(v1.ResourceList),
		globalLimits:    make(v1.ResourceList),
		nsResources:     make(map[string]v1.ResourceList),
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
			nw.addToResourceTracking(&ns)
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
					nw.addToResourceTracking(ns)
				}
			case watch.Modified:
				if nw.shouldProcess(ns) {
					nw.schedule(ns)
					nw.updateResourceTracking(ns)
				} else {
					nw.cancel(ns.Name)
					nw.removeFromResourceTracking(ns.Name)
				}
			case watch.Deleted:
				nw.cancel(ns.Name)
				nw.removeFromResourceTracking(ns.Name)
			}
		}
	}
}

// shouldProcess checks if namespace should be cleaned up
func (nw *NamespaceWatcher) shouldProcess(ns *v1.Namespace) bool {
	if ns.Name == "tenama-system" {
		return false
	}

	if !strings.HasPrefix(ns.Name, nw.prefix) {
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

// stopAllTimers stops all active timers and clears resource tracking
func (nw *NamespaceWatcher) stopAllTimers() {
	nw.mu.Lock()
	defer nw.mu.Unlock()

	for _, timer := range nw.timers {
		timer.Stop()
	}
	nw.timers = make(map[string]*time.Timer)

	nw.resourceMu.Lock()
	defer nw.resourceMu.Unlock()
	nw.currentUsage = make(v1.ResourceList)
	nw.nsResources = make(map[string]v1.ResourceList)
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

// SetGlobalLimits sets the global resource limits for all namespaces
func (nw *NamespaceWatcher) SetGlobalLimits(limits v1.ResourceList) {
	nw.resourceMu.Lock()
	defer nw.resourceMu.Unlock()
	nw.globalLimits = limits.DeepCopy()
}

// addToResourceTracking adds namespace resources to the current usage
func (nw *NamespaceWatcher) addToResourceTracking(ns *v1.Namespace) {
	if ns == nil {
		return
	}

	nw.resourceMu.Lock()
	defer nw.resourceMu.Unlock()

	// Extract resources from namespace spec (from requests)
	resources := extractNamespaceResources(ns)
	nw.nsResources[ns.Name] = resources.DeepCopy()

	// Add to current usage
	for key, val := range resources {
		if current, ok := nw.currentUsage[key]; ok {
			current.Add(val)
			nw.currentUsage[key] = current
		} else {
			nw.currentUsage[key] = val.DeepCopy()
		}
	}

	log.Debugf("Added resources for namespace %s, current usage: %v", ns.Name, nw.currentUsage)
}

// removeFromResourceTracking removes namespace resources from the current usage
func (nw *NamespaceWatcher) removeFromResourceTracking(namespaceName string) {
	nw.resourceMu.Lock()
	defer nw.resourceMu.Unlock()

	resources, exists := nw.nsResources[namespaceName]
	if !exists {
		return
	}

	// Subtract from current usage
	for key, val := range resources {
		if current, ok := nw.currentUsage[key]; ok {
			current.Sub(val)
			// Validate that we don't end up with negative values (indicates tracking inconsistency)
			if current.Sign() < 0 {
				log.Warnf("Resource tracking inconsistency detected: %s became negative after removing namespace %s", key, namespaceName)
				delete(nw.currentUsage, key)
			} else if current.IsZero() {
				delete(nw.currentUsage, key)
			} else {
				nw.currentUsage[key] = current
			}
		}
	}

	delete(nw.nsResources, namespaceName)
	log.Debugf("Removed resources for namespace %s, current usage: %v", namespaceName, nw.currentUsage)
}

// updateResourceTracking updates resources for a modified namespace
func (nw *NamespaceWatcher) updateResourceTracking(ns *v1.Namespace) {
	if ns == nil {
		return
	}

	nw.resourceMu.Lock()

	oldResources, exists := nw.nsResources[ns.Name]
	if !exists {
		// If not tracked yet, treat as add (must unlock before calling to avoid deadlock)
		nw.resourceMu.Unlock()
		nw.addToResourceTracking(ns)
		return
	}

	newResources := extractNamespaceResources(ns)

	// Remove old resources
	for key, val := range oldResources {
		if current, ok := nw.currentUsage[key]; ok {
			current.Sub(val)
			if current.IsZero() {
				delete(nw.currentUsage, key)
			} else {
				nw.currentUsage[key] = current
			}
		}
	}

	// Add new resources
	for key, val := range newResources {
		if current, ok := nw.currentUsage[key]; ok {
			current.Add(val)
			nw.currentUsage[key] = current
		} else {
			nw.currentUsage[key] = val.DeepCopy()
		}
	}

	nw.nsResources[ns.Name] = newResources.DeepCopy()
	log.Debugf("Updated resources for namespace %s, current usage: %v", ns.Name, nw.currentUsage)
	nw.resourceMu.Unlock()
}

// CanCreateNamespace checks if creating a new namespace would exceed global limits
func (nw *NamespaceWatcher) CanCreateNamespace(newNamespaceResources v1.ResourceList) bool {
	if len(nw.globalLimits) == 0 {
		// No limits set, allow creation
		return true
	}

	nw.resourceMu.RLock()
	defer nw.resourceMu.RUnlock()

	// Check each resource type
	for resourceType, limit := range nw.globalLimits {
		currentVal, exists := nw.currentUsage[resourceType]
		if !exists {
			currentVal = *resource.NewQuantity(0, resource.DecimalSI)
		}

		newVal, newExists := newNamespaceResources[resourceType]
		if !newExists {
			continue
		}

		// Calculate total that would be used
		total := currentVal.DeepCopy()
		total.Add(newVal)

		// Compare with limit
		if total.Cmp(limit) > 0 {
			log.Warnf("Global limit exceeded for %s: current=%v, new=%v, limit=%v",
				resourceType, currentVal.String(), newVal.String(), limit.String())
			return false
		}
	}

	return true
}

// GetCurrentResourceUsage returns current global resource usage
func (nw *NamespaceWatcher) GetCurrentResourceUsage() v1.ResourceList {
	nw.resourceMu.RLock()
	defer nw.resourceMu.RUnlock()
	return nw.currentUsage.DeepCopy()
}

// GetGlobalLimits returns the configured global limits
func (nw *NamespaceWatcher) GetGlobalLimits() v1.ResourceList {
	nw.resourceMu.RLock()
	defer nw.resourceMu.RUnlock()
	return nw.globalLimits.DeepCopy()
}

// extractNamespaceResources extracts resource requests from a namespace's labels/annotations
// Resources are stored from the namespace creation request in labels
func extractNamespaceResources(ns *v1.Namespace) v1.ResourceList {
	if ns == nil {
		return make(v1.ResourceList)
	}

	resources := make(v1.ResourceList)

	// Extract from labels set during namespace creation
	// Labels are set like: "tenama/resource-cpu": "100m", "tenama/resource-memory": "128Mi", etc.
	if cpu, ok := ns.Labels["tenama/resource-cpu"]; ok {
		if quantity, err := resource.ParseQuantity(cpu); err == nil {
			resources[v1.ResourceCPU] = quantity
		}
	}

	if memory, ok := ns.Labels["tenama/resource-memory"]; ok {
		if quantity, err := resource.ParseQuantity(memory); err == nil {
			resources[v1.ResourceMemory] = quantity
		}
	}

	if storage, ok := ns.Labels["tenama/resource-storage"]; ok {
		if quantity, err := resource.ParseQuantity(storage); err == nil {
			resources[v1.ResourceStorage] = quantity
		}
	}

	return resources
}
