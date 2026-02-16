package proxy

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestProxyHandler_Streaming(t *testing.T) {
	logrus.SetOutput(io.Discard) // Avoid panic due to concurrent log output setting in other tests
	defer logrus.SetOutput(nil)

	// Setup a mock upstream server that sends chunks with delays
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Transfer-Encoding", "chunked")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Errorf("expected flusher")
			return
		}

		for i := 1; i <= 3; i++ {
			fmt.Fprintf(w, "chunk-%d", i)
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	// Create ProxyHandler pointing to mock upstream
	ph, err := NewProxyHandler(upstream.URL, 8080, 30*time.Second)
	if err != nil {
		t.Fatalf("Failed to create proxy handler: %v", err)
	}

	// Perform a request to the proxy
	req := httptest.NewRequest("GET", "/", nil)

	// We need a response writer that supports flushing to test if the proxy flushes
	w := httptest.NewRecorder()

	// Start proxying in a goroutine
	done := make(chan bool)
	go func() {
		ph.ServeHTTP(w, req)
		done <- true
	}()

	// Monitor the response body as it grows
	lastSize := 0
	chunksReceived := 0
	start := time.Now()

	for {
		select {
		case <-done:
			if chunksReceived < 3 {
				t.Errorf("Expected 3 chunks, got only %d", chunksReceived)
			}
			return
		case <-time.After(50 * time.Millisecond):
			currentSize := w.Body.Len()
			if currentSize > lastSize {
				chunksReceived++
				lastSize = currentSize
				t.Logf("Received chunk %d at %v", chunksReceived, time.Since(start))
			}
			if time.Since(start) > 2*time.Second {
				t.Fatal("Timeout waiting for chunks")
			}
		}
	}
}
