package handler

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestProxyHandler_ServeHTTP_Logging(t *testing.T) {
	// Setup a mock upstream server
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	// Create ProxyHandler pointing to mock upstream
	ph, err := NewProxyHandler(upstream.URL, 8080, 30*time.Second)
	if err != nil {
		t.Fatalf("Failed to create proxy handler: %v", err)
	}

	// Capture logrus output
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(nil) // Reset to default (stderr)

	// Perform a request
	req := httptest.NewRequest("POST", "/test-path", bytes.NewBufferString("hello"))
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()

	ph.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusAccepted {
		t.Errorf("Expected status 202, got %d", w.Code)
	}

	// Verify logs
	logOutput := buf.String()
	expectedFields := []string{
		"method=POST",
		"path=/test-path",
		"status=202",
		"remote=\"1.2.3.4:1234\"",
		"msg=\"HTTP request\"",
	}

	for _, field := range expectedFields {
		if !bytes.Contains(buf.Bytes(), []byte(field)) {
			t.Errorf("Expected log to contain %q, but it didn't. Log: %s", field, logOutput)
		}
	}
}
