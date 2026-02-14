package handler

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"llm-monitor/internal/interceptor"

	"github.com/sirupsen/logrus"
)

// ProxyHandler handles proxy requests
type ProxyHandler struct {
	UpstreamURL *url.URL
	Manager     *interceptor.Manager
	Client      *http.Client
	Port        int
}

func createHttpTransport() *http.Transport {
	// Create a custom HTTP client with TLS configuration
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			// Remove InsecureSkipVerify for production use
			// InsecureSkipVerify: true, // For demo purposes only
		},
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		// Configure proxy using standard environment variables
		Proxy: http.ProxyFromEnvironment,
	}
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(upstreamURL string, port int) (*ProxyHandler, error) {
	parsedURL, err := url.Parse(upstreamURL)
	if err != nil {
		return nil, fmt.Errorf("invalid upstream URL: %v", err)
	}

	// Create a custom HTTP client with TLS configuration
	transport := createHttpTransport()

	logrus.WithFields(logrus.Fields{
		"port":     port,
		"upstream": upstreamURL,
	}).Info("Server configuration")

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
	// Get interceptor for this endpoint
	intcptor := ph.Manager.GetInterceptor(r.URL.Path)
	var state interceptor.State

	if intcptor != nil {
		// Create state for this interceptor
		state = intcptor.CreateState()
	}

	err := ph.ServeHTTP2(w, r, intcptor, state)

	if intcptor != nil {
		if err != nil {
			intcptor.OnError(state, err)
		} else {
			intcptor.OnComplete(state)
		}
	}
}

func (ph *ProxyHandler) ServeHTTP2(w http.ResponseWriter, r *http.Request, intcptor interceptor.Interceptor, state interceptor.State) error {
	// Create a copy of the request to modify headers
	req := r.Clone(r.Context())
	req.RequestURI = ""
	req.Host = ""
	req.RemoteAddr = ""
	req.URL.Scheme = ph.UpstreamURL.Scheme
	req.URL.Host = ph.UpstreamURL.Host
	modifyHeaders(req, map[string]string{
		"X-Forwarded-Proto": "http",
		"X-Forwarded-Host":  r.Host,
		"X-Forwarded-For":   r.RemoteAddr,
	})

	if intcptor != nil {
		// Apply request interceptor
		if err := intcptor.RequestInterceptor(req, state); err != nil {
			logrus.WithError(err).Warn("Error in intercepting request")
		}
	}

	// Forward the request to upstream
	logrus.Printf(req.URL.String())
	resp, err := ph.Client.Do(req)
	if err != nil {
		http.Error(w, "Upstream error", http.StatusBadGateway)
		return err
	}
	defer func() {
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}()

	// Apply response interceptor if exists
	if intcptor != nil {
		if err := intcptor.ResponseInterceptor(resp, state); err != nil {
			logrus.WithError(err).Warn("Error in intercepting response")
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
	if len(resp.TransferEncoding) > 0 && resp.TransferEncoding[0] == "chunked" {
		err := ph.handleChunkedResponse(w, resp, intcptor, state)
		if err != nil {
			// Don't send error response here - we already wrote headers
			return err
		}
	} else {
		// Handle non-chunked responses
		err := ph.handleRegularResponse(w, resp, intcptor, state)
		if err != nil {
			// Don't send error response here - we already wrote headers
			return err
		}
	}

	// Trigger error if upstream returned an error status code
	if resp.StatusCode >= 400 {
		return fmt.Errorf("upstream returned status code %d", resp.StatusCode)
	}
	return nil
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
			// Continue with original data if chunk processing fails
		}
	}

	// Write the processed chunk
	return cw.ResponseWriter.Write(data)
}
