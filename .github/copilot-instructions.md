# Copilot Instructions for Tenama

This document provides guidance for AI agents contributing to the tenama project.

## Project Overview

**Tenama** is a Kubernetes namespace manager that enables non-cluster-admins to create temporary namespaces with automatic lifecycle management and optional global resource limits.

**Architecture**: Event-driven with Kubernetes Watch API (no polling)  
**Language**: Go 1.25  
**Key Dependencies**: k8s.io/client-go v0.34.2, Echo v4, gopkg.in/yaml.v2

## Critical Architecture Patterns

### 1. Event-Driven Resource Tracking (Real-Time, No Polling)

The system uses Kubernetes Watch API for **immediate** namespace lifecycle events:

**File**: `internal/handlers/watcher.go` (431 lines)

**Key Pattern**:

```
Watch Event Stream:
  Added → addToResourceTracking(ns)
  Modified → updateResourceTracking(ns)
  Deleted → removeFromResourceTracking(ns.Name)
```

**Thread Safety**: All resource tracking uses `sync.RWMutex` + `nsResources` map (namespace → resources)

**When Adding Features**:

- Watch events automatically trigger - no polling needed
- Always release locks BEFORE calling other methods (deadlock prevention)
- Namespace resources extracted from labels: `tenama/resource-cpu`, `tenama/resource-memory`, `tenama/resource-storage`

### 2. Validation Gate Pattern (O(1) Checks)

**File**: `internal/handlers/api_namespaces.go` (CreateNamespace handler, ~line 72)

**Pattern**:

```go
// 1. Check if creation would exceed limits
if !c.watcher.CanCreateNamespace(requestedResources) {
    return c.sendErrorResponse(..., http.StatusTooManyRequests)
}
// 2. Create namespace (actual reservation at ADDED watch event)
c.createNamespace(ctx, c.clientset, nsSpec, ...)
```

**Critical**: Validation is O(1) via map lookup; actual reservation happens when watcher receives ADDED event (not atomic race condition due to Kubernetes eventually-consistent model)

### 3. Configuration with camelCase YAML Fields

**File**: `internal/models/config.go`

**Convention**: ALL YAML tags use camelCase, NOT snake_case:

```yaml
globalLimits: # NOT global_limits
  enabled: true
  resources: # NOT resource
    requests:
      cpu: "5000m"
      memory: "10Gi"
      storage: "50Gi"
```

**When Editing config.yaml or config.go**: Always maintain camelCase convention

### 4. Dependency Injection via Container

**File**: `internal/handlers/container.go`

**Pattern**:

- Single `Container` struct holds clientset, config, and watcher
- Echo handlers receive Container via method receiver: `func (c *Container) CreateNamespace(ctx echo.Context)`
- Watcher attached after startup: `c.SetWatcher(watcher)`

**When Adding Handlers**: Attach dependencies to Container, not as globals

### 5. Resource Quantity Handling

**Files**: `internal/handlers/watcher.go`, `internal/handlers/api_namespaces.go`

**Pattern**: Use k8s.io/apimachinery/pkg/api/resource.Quantity for all resource math:

- Never use float64 for resources (precision loss)
- Use `.Sign()` to check negative values: `if quantity.Sign() < 0`
- Use `.Add()` and `.Sub()` for accumulation
- Use `quantity.String()` for display (not `%v` with Quantity objects)

**Helper Function**: `formatResourceQuantity(rl v1.ResourceList, resourceName) string` - converts Quantity to readable string with "not set" fallback

### 6. HTTP Status Code Convention

- **200 OK**: Namespace created successfully
- **400 Bad Request**: Invalid input (missing infix, parse errors)
- **409 Conflict**: Namespace already exists
- **429 Too Many Requests**: Global resource limits exceeded
- **500 Internal Server Error**: Kubernetes API errors

## File Organization & Key Entry Points

| File                                        | Purpose                                     | Key Exports                                           |
| ------------------------------------------- | ------------------------------------------- | ----------------------------------------------------- |
| `cmd/tenama/main.go`                        | Startup (config load, watcher init, routes) | `convertConfigResourcesToResourceList()`              |
| `internal/handlers/watcher.go`              | Event tracking & resource accounting        | `NamespaceWatcher`, `Watch()`, `CanCreateNamespace()` |
| `internal/handlers/api_namespaces.go`       | HTTP handlers for namespace CRUD            | `CreateNamespace()`, `DeleteNamespace()`              |
| `internal/handlers/api_info.go`             | /info endpoint with GlobalLimits status     | `GetBuildInfo()`, `quantityMapToStrings()`            |
| `internal/handlers/container.go`            | Dependency injection                        | `Container`, `NewContainer()`                         |
| `internal/models/config.go`                 | Configuration model (YAML parsing)          | `Config`, `GlobalLimits`, `Resources`                 |
| `internal/handlers/middleware_basicAuth.go` | Authentication                              | `BasicAuthValidator()`                                |

## Testing Patterns

### Fake Clientset for Unit Tests

```go
import "k8s.io/client-go/kubernetes/fake"

fakeCS := fake.NewSimpleClientset()
watcher := NewNamespaceWatcher(fakeCS.CoreV1(), "test-")
```

### Concurrent Resource Tracking Tests

**File**: `internal/handlers/watcher_test.go`

Test concurrent add/remove operations to verify `sync.RWMutex` safety. Always test:

- Multiple goroutines adding resources simultaneously
- Read operations during modifications
- Final state consistency

## Common Tasks & Where to Make Changes

### Adding a New Resource Type (e.g., GPU)

1. **config.go**: Add field to `Resources` struct (e.g., `GPU string`)
2. **watcher.go**: Update `extractNamespaceResources()` to parse `tenama/resource-gpu` label
3. **watcher.go**: Resource math in `addToResourceTracking()`, `removeFromResourceTracking()`, `updateResourceTracking()`
4. **api_namespaces.go**: Add to error message in `formatResourceQuantity()` check
5. **api_info.go**: Already works via generic `quantityMapToStrings()`

### Fixing Mutex Issues

**Critical Pattern** (from recent bug fix):

```go
// ❌ WRONG: defer + manual unlock = double-unlock panic
func method() {
    nw.resourceMu.Lock()
    defer nw.resourceMu.Unlock()
    nw.resourceMu.Unlock()  // PANIC!
}

// ✅ CORRECT: defer OR manual unlock, never both
func method() {
    nw.resourceMu.Lock()
    // ... do work ...
    nw.resourceMu.Unlock()
}
```

If you need to unlock before calling another method:

```go
nw.resourceMu.Unlock()
// Safe to call other methods here
nw.addToResourceTracking(ns)  // Has its own Lock()
```

### Validation Error Messages

**Convention**: ALL lowercase, specific resource type:

```go
// ✅ Correct
"invalid cpu quantity: ..."
"invalid memory quantity: ..."
"invalid storage quantity: ..."

// ❌ Wrong
"Invalid CPU Quantity"
"CPU Error"
```

## Build & Test Commands

```bash
# Build
go build -o tenama ./cmd/tenama

# Run with debug config
./tenama -config config/config.yaml

# Test all handlers & watcher
go test ./internal/... -v

# Test with coverage (tracking)
go test ./internal/handlers/watcher_test.go -v
```

## Kubernetes Integration Points

- **Label-based resource extraction**: Namespaces created with `tenama/resource-*` labels
- **Watch selector**: `created-by=tenama` label required for namespace tracking
- **Cleanup trigger**: Namespace deletion via DELETED watch event
- **Resource quota**: Created per namespace in `craftNamespaceQuotaSpecification()`
- **Service account token**: Kubernetes automatically injects into mounted Secret

## Configuration Considerations

**Production vs Development**:

- **duration**: "168h" (7 days) in production, "30s" for quick testing
- **globalLimits.enabled**: true/false controls entire feature
- **logLevel**: "debug" for development, "info"/"warn" for production
- **basicAuth**: Required for all API endpoints except /info, /docs, /healthz, /readiness

## Known Limitations & Design Decisions

1. **Race Condition (Acknowledged)**: Namespace validation and creation are not atomic. Reservation happens at ADDED watch event, not at API request time. This is acceptable due to Kubernetes eventually-consistent model.

2. **No Polling**: Watcher uses Watch API exclusively. No periodic cleanup interval needed.

3. **camelCase YAML Only**: The config system is strict about camelCase. Always use `globalLimits`, not `global_limits`.

4. **Per-Namespace Resources**: Global limits apply cluster-wide to all tenama-managed namespaces. Per-namespace quotas are separate via ResourceQuota.

## Recent Bug Fixes (Reference)

All fixes implemented in commit 4e03565:

1. ✅ Mutex double-unlock → Manual unlock only in `updateResourceTracking()`
2. ✅ Negative value validation → `.Sign() < 0` check with warning logs
3. ✅ Config duration → Restored "168h" production default
4. ✅ Error capitalization → All lowercase in error messages
5. ✅ Error formatting → `formatResourceQuantity()` helper for Quantity→string
6. ✅ Race condition → Documented as acknowledged trade-off

## API Endpoints Overview

| Method | Path              | Auth      | Returns                  | Notes                                 |
| ------ | ----------------- | --------- | ------------------------ | ------------------------------------- |
| GET    | /info             | -         | BuildInfo + GlobalLimits | No auth needed                        |
| POST   | /namespace        | BasicAuth | Namespace + kubeconfig   | Returns HTTP 429 if limits exceeded   |
| GET    | /namespace        | BasicAuth | List of namespace names  | Filtered by `created-by=tenama`       |
| GET    | /namespace/{name} | BasicAuth | Namespace found message  | Validation only, no resources         |
| DELETE | /namespace/{name} | BasicAuth | Success/error message    | Triggers resource cleanup via watcher |

---

**Last Updated**: After commit 4e03565 (all PR review fixes completed)  
**Tests**: 19/19 passing (watcher, handlers, config models)  
**Status**: Production-ready with event-driven architecture
