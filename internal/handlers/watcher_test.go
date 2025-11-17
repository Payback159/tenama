package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
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
