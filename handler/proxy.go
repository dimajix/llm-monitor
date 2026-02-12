package handler

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
	"llm-sniffer/interceptor"
)

// ProxyHandler handles proxy requests
type ProxyHandler struct {
	UpstreamURL *url.URL
	Manager     *interceptor.InterceptorManager
	Client      *http.Client
	Port        int
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(upstreamURL string, port int) (*ProxyHandler, error) {
	parsedURL, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %v", err)
	}

	// Create a custom HTTP client with TLS configuration
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // For demo purposes only
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	return &ProxyHandler{
		UpstreamURL: parsedURL,
		Manager:     interceptor.NewInterceptorManager(),
		Client: &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		},
		Port: port,
	}, nil
}

// RegisterInterceptor registers an interceptor for a specific endpoint
func (ph *ProxyHandler) RegisterInterceptor(endpoint string, interceptor interceptor.Interceptor) {
	ph.Manager.RegisterInterceptor(endpoint, interceptor)
}

// modifyHeaders modifies headers before sending to upstream
func modifyHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

// ServeHTTP handles incoming HTTP requests
func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Create a copy of the request to modify headers
	req := r.Clone(r.Context())
	req.URL.Scheme = ph.UpstreamURL.Scheme
	req.URL.Host = ph.UpstreamURL.Host
	req.Header.Set("X-Forwarded-For", r.RemoteAddr)

	// Get intcptor for this endpoint
	intcptor, exists := ph.Manager.GetInterceptor(r.URL.Path)
	var state interceptor.State

	if exists && intcptor != nil {
		// Create state for this intcptor
		state = intcptor.CreateState()

		// Apply request intcptor
		if err := intcptor.RequestInterceptor(req, state); err != nil {
			logrus.WithError(err).Warn("Request interceptor error")
			http.Error(w, "Request interceptor error", http.StatusInternalServerError)
			return
		}
	}

	// Modify headers if needed
	modifyHeaders(req, map[string]string{
		"X-Forwarded-Proto": "http",
		"X-Forwarded-Host":  r.Host,
	})

	// Forward the request to upstream
	resp, err := ph.Client.Do(req)
	if err != nil {
		logrus.WithError(err).Warn("Upstream error")
		http.Error(w, "Upstream error", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Apply response intcptor if exists
	if exists && intcptor != nil {
		if err := intcptor.ResponseInterceptor(resp, state); err != nil {
			logrus.WithError(err).Warn("Response interceptor error")
			http.Error(w, "Response interceptor error", http.StatusInternalServerError)
			return
		}
	}

	// Copy response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Handle chunked responses
	if resp.Header.Get("Transfer-Encoding") == "chunked" {
		err := ph.handleChunkedResponse(w, resp, intcptor, state)
		if err != nil {
			logrus.WithError(err).Warn("Error handling chunked response")
			http.Error(w, "Error handling chunked response", http.StatusInternalServerError)
			return
		}
	} else {
		// Handle non-chunked responses
		err := ph.handleRegularResponse(w, resp, intcptor, state)
		if err != nil {
			logrus.WithError(err).Warn("Error handling regular response")
			http.Error(w, "Error handling regular response", http.StatusInternalServerError)
			return
		}
	}

	// Call onComplete when response is complete
	if intcptor != nil {
		intcptor.OnComplete(state)
	}
}

// handleChunkedResponse handles chunked responses with interceptors
func (ph *ProxyHandler) handleChunkedResponse(w http.ResponseWriter, resp *http.Response, interceptor interceptor.Interceptor, state interceptor.State) error {
	// Create a custom response writer that intercepts chunks
	chunkWriter := &chunkWriter{
		ResponseWriter: w,
		interceptor:    interceptor,
		state:          state,
	}

	// Copy response body to our chunk writer
	_, err := io.Copy(chunkWriter, resp.Body)
	if err != nil {
		logrus.WithError(err).Warn("Error copying chunked response")
		return err
	}

	return nil
}

// handleRegularResponse handles non-chunked responses
func (ph *ProxyHandler) handleRegularResponse(w http.ResponseWriter, resp *http.Response, interceptor interceptor.Interceptor, state interceptor.State) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logrus.WithError(err).Warn("Error reading response body")
		return err
	}

	// Apply content interceptor if exists
	if interceptor != nil {
		if processedBody, err := interceptor.ContentInterceptor(body, state); err == nil {
			body = processedBody
		} else {
			logrus.WithError(err).Warn("Error in intercepting body")
		}
	}

	// Write the final response
	_, err = w.Write(body)
	if err != nil {
		logrus.WithError(err).Warn("Error writing response")
		return err
	}

	return nil
}

// chunkWriter intercepts chunks of data
type chunkWriter struct {
	http.ResponseWriter
	interceptor interceptor.Interceptor
	state       interceptor.State
}

// Write intercepts chunks and applies chunk interceptors
func (cw *chunkWriter) Write(data []byte) (int, error) {
	// If there's an interceptor, process the chunk
	if cw.interceptor != nil {
		if processedData, err := cw.interceptor.ChunkInterceptor(data, cw.state); err == nil {
			data = processedData
		} else {
			logrus.WithError(err).Warn("Error in intercepting chunk")
		}
	}

	// Write the processed chunk
	return cw.ResponseWriter.Write(data)
}
