package interceptor

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// CustomInterceptorState extends the base state with chunk-specific information
type CustomInterceptorState struct {
	IsChunked  bool
	ChunkCount int
	TotalSize  int
	LastChunk  bool
	Chunks     []string
}

func NewChunkInterceptorState() *CustomInterceptorState {
	return &CustomInterceptorState{
		Chunks: make([]string, 0),
	}
}

// CustomInterceptor implements the Interceptor interface
type CustomInterceptor struct {
	Name string
}

func (ci *CustomInterceptor) CreateState() State {
	return NewChunkInterceptorState()
}

// RequestInterceptor modifies the request
func (ci *CustomInterceptor) RequestInterceptor(req *http.Request, state State) error {
	logrus.WithFields(logrus.Fields{
		"interceptor": ci.Name,
		"method":      req.Method,
		"path":        req.URL.Path,
		"timestamp":   time.Now().Format(time.RFC3339),
	}).Info("Request intercepted")

	// Add custom header
	req.Header.Set("X-Intercepted-By", ci.Name)

	// Update state
	if chunkState, ok := state.(*CustomInterceptorState); ok {
		chunkState.IsChunked = true
	}

	return nil
}

// ResponseInterceptor modifies the response
func (ci *CustomInterceptor) ResponseInterceptor(resp *http.Response, state State) error {
	logrus.WithFields(logrus.Fields{
		"interceptor": ci.Name,
		"status":      resp.StatusCode,
		"timestamp":   time.Now().Format(time.RFC3339),
	}).Info("Response intercepted")

	// Add custom header
	resp.Header.Set("X-Intercepted-Response", ci.Name)

	// Update state
	if chunkState, ok := state.(*CustomInterceptorState); ok {
		chunkState.TotalSize = int(resp.ContentLength)
	}

	return nil
}

// ContentInterceptor modifies the content
func (ci *CustomInterceptor) ContentInterceptor(content []byte, _ State) ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"interceptor": ci.Name,
		"bytes":       len(content),
		"timestamp":   time.Now().Format(time.RFC3339),
	}).Info("Content intercepted")

	// Simple content modification example
	modified := bytes.ReplaceAll(content, []byte("Hello"), []byte("Hi"))
	return modified, nil
}

// ChunkInterceptor processes chunks of content
func (ci *CustomInterceptor) ChunkInterceptor(chunk []byte, state State) ([]byte, error) {
	logrus.WithFields(logrus.Fields{
		"interceptor": ci.Name,
		"bytes":       len(chunk),
		"timestamp":   time.Now().Format(time.RFC3339),
	}).Info("Chunk intercepted")

	// Update state
	if chunkState, ok := state.(*CustomInterceptorState); ok {
		chunkState.ChunkCount++
		chunkState.TotalSize += len(chunk)
		chunkState.Chunks = append(chunkState.Chunks, string(chunk))
	}

	// Process chunk
	processed := bytes.ReplaceAll(chunk, []byte("chunk"), []byte("modified_chunk"))

	return processed, nil
}

// OnComplete is called when response is complete
func (ci *CustomInterceptor) OnComplete(state State) {
	chunkCount := 0
	totalSize := 0

	if chunkState, ok := state.(*CustomInterceptorState); ok {
		chunkCount = chunkState.ChunkCount
		totalSize = chunkState.TotalSize
	}

	logrus.WithFields(logrus.Fields{
		"interceptor":  ci.Name,
		"total_chunks": chunkCount,
		"total_bytes":  totalSize,
		"timestamp":    time.Now().Format(time.RFC3339),
	}).Info("Response complete")
}

func (li *CustomInterceptor) OnError(state State, _ error) {
	log.Printf("[%s] Logging completion", li.Name)
}
