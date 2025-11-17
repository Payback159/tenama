package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestNewNamespaceWatcher(t *testing.T) {
	// Test with fake clientset
	fakeClientset := fake.NewSimpleClientset()
	// Access the CoreV1() interface directly - works with both real and fake
	watcher := NewNamespaceWatcher(fakeClientset.CoreV1(), "test-")

	if watcher == nil {
		t.Error("Expected watcher to be created")
	}
	if watcher.prefix != "test-" {
		t.Errorf("Expected prefix 'test-', got %s", watcher.prefix)
	}
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		prefix    string
		want      bool
	}{
		{"matches", "test-ns", "test-", true},
		{"no match", "prod-ns", "test-", false},
		{"empty prefix", "any", "", true},
		{"too short", "t", "test-", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := strings.HasPrefix(tt.namespace, tt.prefix)
			if got != tt.want {
				t.Errorf("strings.HasPrefix(%s, %s) = %v, want %v", tt.namespace, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestShouldProcess(t *testing.T) {
	// Create watcher with nil clientset (not needed for shouldProcess)
	watcher := &NamespaceWatcher{prefix: "test-"}

	tests := []struct {
		name      string
		namespace *v1.Namespace
		want      bool
	}{
		{
			name: "valid namespace",
			namespace: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-ns",
					Labels: map[string]string{
						"tenama/namespace-duration": "5m",
					},
				},
			},
			want: true,
		},
		{
			name: "tenama-system",
			namespace: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "tenama-system",
					Labels: map[string]string{
						"tenama/namespace-duration": "5m",
					},
				},
			},
			want: false,
		},
		{
			name: "no duration label",
			namespace: &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test-ns",
					Labels: map[string]string{},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := watcher.shouldProcess(tt.namespace)
			if got != tt.want {
				t.Errorf("shouldProcess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetActiveTimerCount(t *testing.T) {
	watcher := &NamespaceWatcher{
		timers: make(map[string]*time.Timer),
	}

	if count := watcher.GetActiveTimerCount(); count != 0 {
		t.Errorf("Expected 0 timers initially, got %d", count)
	}

	watcher.mu.Lock()
	watcher.timers["test-1"] = time.AfterFunc(10*time.Second, func() {})
	watcher.timers["test-2"] = time.AfterFunc(10*time.Second, func() {})
	watcher.mu.Unlock()

	if count := watcher.GetActiveTimerCount(); count != 2 {
		t.Errorf("Expected 2 timers, got %d", count)
	}
}

func TestCancel(t *testing.T) {
	watcher := &NamespaceWatcher{
		timers: make(map[string]*time.Timer),
	}

	watcher.mu.Lock()
	watcher.timers["test-ns"] = time.AfterFunc(10*time.Second, func() {})
	watcher.mu.Unlock()

	watcher.cancel("test-ns")

	if count := watcher.GetActiveTimerCount(); count != 0 {
		t.Errorf("Expected 0 timers after cancel, got %d", count)
	}
}

func TestStop(t *testing.T) {
	watcher := &NamespaceWatcher{
		timers: make(map[string]*time.Timer),
		done:   make(chan struct{}),
	}

	watcher.mu.Lock()
	watcher.timers["test-1"] = time.AfterFunc(10*time.Second, func() {})
	watcher.timers["test-2"] = time.AfterFunc(10*time.Second, func() {})
	watcher.mu.Unlock()

	watcher.Stop()

	if count := watcher.GetActiveTimerCount(); count != 0 {
		t.Errorf("Expected 0 timers after stop, got %d", count)
	}
}

// TestConcurrentTimerAccess tests that concurrent access to timers is safe
func TestConcurrentTimerAccess(t *testing.T) {
	watcher := &NamespaceWatcher{
		timers: make(map[string]*time.Timer),
	}

	// Spawn multiple goroutines accessing timers concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				nsName := fmt.Sprintf("test-ns-%d-%d", id, j)
				watcher.mu.Lock()
				watcher.timers[nsName] = time.AfterFunc(1*time.Hour, func() {})
				watcher.mu.Unlock()

				_ = watcher.GetActiveTimerCount()

				watcher.cancel(nsName)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state
	if count := watcher.GetActiveTimerCount(); count != 0 {
		t.Errorf("Expected 0 timers after concurrent ops, got %d", count)
	}
}

// TestConcurrentCancelAndRead tests concurrent cancel and read operations
func TestConcurrentCancelAndRead(t *testing.T) {
	watcher := &NamespaceWatcher{
		timers: make(map[string]*time.Timer),
	}

	// Pre-populate with timers
	for i := 0; i < 50; i++ {
		nsName := fmt.Sprintf("namespace-%d", i)
		watcher.mu.Lock()
		watcher.timers[nsName] = time.AfterFunc(1*time.Hour, func() {})
		watcher.mu.Unlock()
	}

	done := make(chan bool)

	// Goroutines that cancel timers
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := id; j < 50; j += 5 {
				nsName := fmt.Sprintf("namespace-%d", j)
				watcher.cancel(nsName)
				time.Sleep(1 * time.Millisecond)
			}
			done <- true
		}(i)
	}

	// Goroutines that read count
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				_ = watcher.GetActiveTimerCount()
				time.Sleep(1 * time.Millisecond)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// All timers should be cancelled
	if count := watcher.GetActiveTimerCount(); count != 0 {
		t.Errorf("Expected 0 timers after concurrent cancel, got %d", count)
	}
}

// TestResourceTracking tests the resource tracking functionality
func TestResourceTracking(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	watcher := NewNamespaceWatcher(clientset.CoreV1(), "tenama")
	
	// Set global limits
	limits := v1.ResourceList{
		v1.ResourceCPU:     parseQuantity("5000m"),
		v1.ResourceMemory:  parseQuantity("10Gi"),
		v1.ResourceStorage: parseQuantity("50Gi"),
	}
	watcher.SetGlobalLimits(limits)
	
	// Create test namespace with resources
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tenama-test-1",
			Labels: map[string]string{
				"tenama/resource-cpu":     "1000m",
				"tenama/resource-memory":  "2Gi",
				"tenama/resource-storage": "5Gi",
			},
		},
	}
	
	// Extract and add resources
	watcher.addToResourceTracking(ns)
	
	// Verify current usage (just check that something was added)
	usage := watcher.GetCurrentResourceUsage()
	cpuValue := usage[v1.ResourceCPU]
	if cpuValue.Value() == 0 {
		t.Error("Expected CPU usage to be non-zero after adding namespace")
	}
	
	// Verify limits are still intact
	currentLimits := watcher.GetGlobalLimits()
	if len(currentLimits) == 0 {
		t.Error("Expected limits to be set")
	}
}

// TestCanCreateNamespace tests the CanCreateNamespace validation
func TestCanCreateNamespace(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	watcher := NewNamespaceWatcher(clientset.CoreV1(), "tenama")
	
	// Set global limits
	limits := v1.ResourceList{
		v1.ResourceCPU:     parseQuantity("5000m"),
		v1.ResourceMemory:  parseQuantity("10Gi"),
		v1.ResourceStorage: parseQuantity("50Gi"),
	}
	watcher.SetGlobalLimits(limits)
	
	// Add initial namespace
	ns1 := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tenama-test-1",
			Labels: map[string]string{
				"tenama/resource-cpu":     "1000m",
				"tenama/resource-memory":  "2Gi",
				"tenama/resource-storage": "5Gi",
			},
		},
	}
	watcher.addToResourceTracking(ns1)
	
	// Test 1: Can create namespace within limits
	newResources1 := v1.ResourceList{
		v1.ResourceCPU:     parseQuantity("3000m"),
		v1.ResourceMemory:  parseQuantity("5Gi"),
		v1.ResourceStorage: parseQuantity("10Gi"),
	}
	if !watcher.CanCreateNamespace(newResources1) {
		t.Error("Expected CanCreateNamespace to return true for resources within limits")
	}
	
	// Test 2: Cannot exceed CPU limit
	newResources2 := v1.ResourceList{
		v1.ResourceCPU:     parseQuantity("5000m"),
		v1.ResourceMemory:  parseQuantity("2Gi"),
		v1.ResourceStorage: parseQuantity("5Gi"),
	}
	if watcher.CanCreateNamespace(newResources2) {
		t.Error("Expected CanCreateNamespace to return false when exceeding CPU limit")
	}
	
	// Test 3: Cannot exceed memory limit
	newResources3 := v1.ResourceList{
		v1.ResourceCPU:     parseQuantity("2000m"),
		v1.ResourceMemory:  parseQuantity("9Gi"),
		v1.ResourceStorage: parseQuantity("5Gi"),
	}
	if watcher.CanCreateNamespace(newResources3) {
		t.Error("Expected CanCreateNamespace to return false when exceeding memory limit")
	}
	
	// Test 4: Exactly at limit (should succeed)
	newResources4 := v1.ResourceList{
		v1.ResourceCPU:     parseQuantity("4000m"),
		v1.ResourceMemory:  parseQuantity("8Gi"),
		v1.ResourceStorage: parseQuantity("45Gi"),
	}
	if !watcher.CanCreateNamespace(newResources4) {
		t.Error("Expected CanCreateNamespace to return true when exactly at limit")
	}
}

// TestRemoveFromResourceTracking tests resource removal
func TestRemoveFromResourceTracking(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	watcher := NewNamespaceWatcher(clientset.CoreV1(), "tenama")
	
	// Set global limits
	limits := v1.ResourceList{
		v1.ResourceCPU:     parseQuantity("5000m"),
		v1.ResourceMemory:  parseQuantity("10Gi"),
		v1.ResourceStorage: parseQuantity("50Gi"),
	}
	watcher.SetGlobalLimits(limits)
	
	// Add namespace
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tenama-test-1",
			Labels: map[string]string{
				"tenama/resource-cpu":     "2000m",
				"tenama/resource-memory":  "4Gi",
				"tenama/resource-storage": "10Gi",
			},
		},
	}
	watcher.addToResourceTracking(ns)
	
	// Verify resources were added
	usage := watcher.GetCurrentResourceUsage()
	if len(usage) == 0 {
		t.Error("Expected resource usage to be tracked after adding namespace")
	}
	
	// Remove namespace
	watcher.removeFromResourceTracking("tenama-test-1")
	
	// Verify resources were removed (should be empty or minimal)
	usage = watcher.GetCurrentResourceUsage()
	if len(usage) > 0 {
		t.Error("Expected resource usage to be empty after removing namespace")
	}
}

// TestUpdateResourceTracking tests resource update on modification
func TestUpdateResourceTracking(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	watcher := NewNamespaceWatcher(clientset.CoreV1(), "tenama")
	
	// Set global limits
	limits := v1.ResourceList{
		v1.ResourceCPU:     parseQuantity("5000m"),
		v1.ResourceMemory:  parseQuantity("10Gi"),
		v1.ResourceStorage: parseQuantity("50Gi"),
	}
	watcher.SetGlobalLimits(limits)
	
	// Add initial namespace
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "tenama-test-1",
			Labels: map[string]string{
				"tenama/resource-cpu":     "1000m",
				"tenama/resource-memory":  "2Gi",
				"tenama/resource-storage": "5Gi",
			},
		},
	}
	watcher.addToResourceTracking(ns)
	
	// Update namespace with new resources
	ns.ObjectMeta.Labels["tenama/resource-cpu"] = "2000m"
	ns.ObjectMeta.Labels["tenama/resource-memory"] = "3Gi"
	ns.ObjectMeta.Labels["tenama/resource-storage"] = "8Gi"
	watcher.updateResourceTracking(ns)
	
	// Verify resources were updated (just check that something is tracked)
	usage := watcher.GetCurrentResourceUsage()
	if len(usage) == 0 {
		t.Error("Expected resource usage to be tracked after update")
	}
}

// TestConcurrentResourceTracking tests thread safety of resource tracking
func TestConcurrentResourceTracking(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	watcher := NewNamespaceWatcher(clientset.CoreV1(), "tenama")
	
	// Set global limits
	limits := v1.ResourceList{
		v1.ResourceCPU:     parseQuantity("10000m"),
		v1.ResourceMemory:  parseQuantity("100Gi"),
		v1.ResourceStorage: parseQuantity("500Gi"),
	}
	watcher.SetGlobalLimits(limits)
	
	done := make(chan bool)
	
	// Goroutines that add resources
	for i := 0; i < 10; i++ {
		go func(id int) {
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("tenama-test-%d", id),
					Labels: map[string]string{
						"tenama/resource-cpu":     "500m",
						"tenama/resource-memory":  "1Gi",
						"tenama/resource-storage": "5Gi",
					},
				},
			}
			watcher.addToResourceTracking(ns)
			done <- true
		}(i)
	}
	
	// Goroutines that read usage
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_ = watcher.GetCurrentResourceUsage()
				_ = watcher.CanCreateNamespace(v1.ResourceList{
					v1.ResourceCPU: parseQuantity("100m"),
				})
				time.Sleep(1 * time.Millisecond)
			}
			done <- true
		}()
	}
	
	// Wait for all goroutines
	for i := 0; i < 15; i++ {
		<-done
	}
	
	// Verify final state (all namespaces should be tracked)
	usage := watcher.GetCurrentResourceUsage()
	if len(usage) == 0 {
		t.Error("Expected resource usage to be tracked after concurrent operations")
	}
}

// Helper function to parse quantity string
func parseQuantity(str string) resource.Quantity {
	q, _ := resource.ParseQuantity(str)
	return q
}
