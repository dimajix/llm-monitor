package interceptor

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
)

// CustomInterceptor implements the Interceptor interface
type CustomInterceptor struct {
	Name string
}

func (ci *CustomInterceptor) CreateState() InterceptorState {
	return NewChunkInterceptorState(ci.Name)
}

// RequestInterceptor modifies the request
func (ci *CustomInterceptor) RequestInterceptor(req *http.Request, state InterceptorState) error {
	log.Printf("[%s] Request intercepted: %s %s", ci.Name, req.Method, req.URL.Path)
	// Add custom header
	req.Header.Set("X-Intercepted-By", ci.Name)

	// Update state
	if chunkState, ok := state.(*ChunkInterceptorState); ok {
		chunkState.IsChunked = true
	}

	return nil
}

// ResponseInterceptor modifies the response
func (ci *CustomInterceptor) ResponseInterceptor(resp *http.Response, state InterceptorState) error {
	log.Printf("[%s] Response intercepted: Status %d", ci.Name, resp.StatusCode)
	// Add custom header
	resp.Header.Set("X-Intercepted-Response", ci.Name)

	// Update state
	if chunkState, ok := state.(*ChunkInterceptorState); ok {
		chunkState.TotalSize = resp.ContentLength
	}

	return nil
}

// ContentInterceptor modifies the content
func (ci *CustomInterceptor) ContentInterceptor(content []byte, state InterceptorState) ([]byte, error) {
	log.Printf("[%s] Content intercepted: %d bytes", ci.Name, len(content))
	// Simple content modification example
	modified := bytes.ReplaceAll(content, []byte("Hello"), []byte("Hi"))
	return modified, nil
}

// ChunkInterceptor processes chunks of content
func (ci *CustomInterceptor) ChunkInterceptor(chunk []byte, state InterceptorState) ([]byte, error) {
	log.Printf("[%s] Chunk intercepted: %d bytes", ci.Name, len(chunk))

	// Update state
	if chunkState, ok := state.(*ChunkInterceptorState); ok {
		chunkState.ChunkCount++
		chunkState.TotalSize += len(chunk)
		chunkState.Chunks = append(chunkState.Chunks, string(chunk))
	}

	// Process chunk
	processed := bytes.ReplaceAll(chunk, []byte("chunk"), []byte("modified_chunk"))

	return processed, nil
}

// OnComplete is called when response is complete
func (ci *CustomInterceptor) OnComplete(state InterceptorState) error {
	log.Printf("[%s] Response complete. Total chunks: %d, Total bytes: %d", ci.Name,
		func() int {
			if chunkState, ok := state.(*ChunkInterceptorState); ok {
				return chunkState.ChunkCount
			}
			return 0
		}(),
		func() int {
			if chunkState, ok := state.(*ChunkInterceptorState); ok {
				return chunkState.TotalSize
			}
			return 0
		}())

	return nil
}
