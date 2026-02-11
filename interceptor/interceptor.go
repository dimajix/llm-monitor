package interceptor

import (
	"net/http"
	"sync"
)

// InterceptorState is a generic interface for state management
type InterceptorState interface {
	GetID() string
	SetID(id string)
	GetData() []byte
	SetData(data []byte)
}

// BaseInterceptorState implements the basic InterceptorState interface
type BaseInterceptorState struct {
	ID   string
	Data []byte
}

func (bis *BaseInterceptorState) GetID() string {
	return bis.ID
}

func (bis *BaseInterceptorState) SetID(id string) {
	bis.ID = id
}

func (bis *BaseInterceptorState) GetData() []byte {
	return bis.Data
}

func (bis *BaseInterceptorState) SetData(data []byte) {
	bis.Data = data
}

// ChunkInterceptorState extends the base state with chunk-specific information
type ChunkInterceptorState struct {
	*BaseInterceptorState
	IsChunked  bool
	ChunkCount int
	TotalSize  int
	LastChunk  bool
	Chunks     []string
}

func NewChunkInterceptorState(id string) *ChunkInterceptorState {
	return &ChunkInterceptorState{
		BaseInterceptorState: &BaseInterceptorState{ID: id},
		Chunks:               make([]string, 0),
	}
}

// Interceptor defines the interface for interceptors
type Interceptor interface {
	// CreateState creates a new state object for this interceptor
	CreateState() InterceptorState

	// RequestInterceptor modifies the request before forwarding
	RequestInterceptor(req *http.Request, state InterceptorState) error

	// ResponseInterceptor modifies the response before sending to client
	ResponseInterceptor(resp *http.Response, state InterceptorState) error

	// ContentInterceptor modifies the content before sending to client
	ContentInterceptor(content []byte, state InterceptorState) ([]byte, error)

	// ChunkInterceptor processes chunks of content (for chunked responses)
	ChunkInterceptor(chunk []byte, state InterceptorState) ([]byte, error)

	// OnComplete is called when the response is complete
	OnComplete(state InterceptorState) error
}

// InterceptorManager manages all interceptors
type InterceptorManager struct {
	interceptors map[string]Interceptor
	mu           sync.RWMutex
}

// NewInterceptorManager creates a new interceptor manager
func NewInterceptorManager() *InterceptorManager {
	return &InterceptorManager{
		interceptors: make(map[string]Interceptor),
	}
}

// RegisterInterceptor registers an interceptor for a specific endpoint
func (im *InterceptorManager) RegisterInterceptor(endpoint string, interceptor Interceptor) {
	im.mu.Lock()
	defer im.mu.Unlock()
	im.interceptors[endpoint] = interceptor
}

// GetInterceptor retrieves an interceptor for an endpoint
func (im *InterceptorManager) GetInterceptor(endpoint string) (Interceptor, bool) {
	im.mu.RLock()
	defer im.mu.RUnlock()
	interceptor, exists := im.interceptors[endpoint]
	return interceptor, exists
}

// CreateInterceptor creates an interceptor instance based on name
func CreateInterceptor(name string) Interceptor {
	switch name {
	case "CustomInterceptor":
		return &CustomInterceptor{Name: name}
	case "SimpleInterceptor":
		return &SimpleInterceptor{Name: name}
	case "LoggingInterceptor":
		return &LoggingInterceptor{Name: name}
	default:
		return &SimpleInterceptor{Name: name}
	}
}
