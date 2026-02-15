package interceptor

import (
	"net/http"
	"sync"
)

// State is now an empty interface
type State interface{}

// EmptyState implements the State interface with no additional fields
// This can be used by simple interceptors that don't need to track any specific information
type EmptyState struct{}

// Interceptor defines the interface for interceptors
type Interceptor interface {
	// CreateState creates a new state object for this interceptor
	CreateState() State

	// RequestInterceptor modifies the request before forwarding
	RequestInterceptor(req *http.Request, state State) error

	// ResponseInterceptor modifies the response before sending to client
	ResponseInterceptor(resp *http.Response, state State) error

	// ContentInterceptor modifies the content before sending to client
	ContentInterceptor(content []byte, state State) ([]byte, error)

	// ChunkInterceptor processes chunks of content (for chunked responses)
	ChunkInterceptor(chunk []byte, state State) ([]byte, error)

	// OnComplete is called when the response is complete
	OnComplete(state State)

	// OnError is called when an error occurs during processing
	OnError(state State, err error)
}

// Manager InterceptorManager manages all interceptors
type Manager struct {
	interceptors map[string]map[string]Interceptor
	mu           sync.RWMutex
}

// NewInterceptorManager creates a new interceptor manager
func NewInterceptorManager() *Manager {
	return &Manager{
		interceptors: make(map[string]map[string]Interceptor),
	}
}

// RegisterInterceptor registers an interceptor for a specific endpoint and method
func (im *Manager) RegisterInterceptor(endpoint string, method string, interceptor Interceptor) {
	im.mu.Lock()
	defer im.mu.Unlock()
	if _, exists := im.interceptors[endpoint]; !exists {
		im.interceptors[endpoint] = make(map[string]Interceptor)
	}
	im.interceptors[endpoint][method] = interceptor
}

// GetInterceptor retrieves an interceptor for an endpoint and method
func (im *Manager) GetInterceptor(endpoint string, method string) Interceptor {
	im.mu.RLock()
	defer im.mu.RUnlock()

	methods, exists := im.interceptors[endpoint]
	if !exists {
		return nil
	}

	// Try exact method match
	if interceptor, exists := methods[method]; exists {
		return interceptor
	}

	// Try wildcard method match
	if interceptor, exists := methods["*"]; exists {
		return interceptor
	}

	return nil
}
