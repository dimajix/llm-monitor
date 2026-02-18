package proxy

import (
	"fmt"
	"llm-monitor/internal/config"
	interceptor2 "llm-monitor/internal/proxy/interceptor"
	ollama2 "llm-monitor/internal/proxy/interceptor/ollama"
	"llm-monitor/internal/storage"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

func CreateServer(cfg config.Config) *http.Server {
	// Parse timeouts
	upstreamTimeout := 30 * time.Second
	if cfg.Proxy.Upstream.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Proxy.Upstream.Timeout); err == nil {
			upstreamTimeout = d
		} else {
			logrus.WithError(err).Warnf("Failed to parse upstream timeout '%s', using default 30s", cfg.Proxy.Upstream.Timeout)
		}
	}

	storageTimeout := 30 * time.Second
	if cfg.Storage.Timeout != "" {
		if d, err := time.ParseDuration(cfg.Storage.Timeout); err == nil {
			storageTimeout = d
		} else {
			logrus.WithError(err).Warnf("Failed to parse storage timeout '%s', using default 30s", cfg.Storage.Timeout)
		}
	}

	// Initialize storage
	store, err := storage.CreateStorage(cfg.Storage)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to initialize storage")
	}
	if store != nil {
		logrus.Info("Initialized storage backend")
	}

	// Create proxy handler
	proxy, err := NewProxyHandler(cfg.Proxy.Upstream.URL, cfg.Proxy.Port, upstreamTimeout)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create proxy handler")
	}

	// Register interceptors based on configuration
	for _, intercept := range cfg.Proxy.Intercepts {
		interceptorInstance, err := CreateInterceptor(intercept.Interceptor, store, storageTimeout)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to create interceptor")
		}
		proxy.RegisterInterceptor(intercept.Endpoint, intercept.Method, interceptorInstance)
		logrus.WithFields(logrus.Fields{
			"interceptor": intercept.Interceptor,
			"endpoint":    intercept.Endpoint,
			"method":      intercept.Method,
		}).Info("Registered interceptor")
	}
	if len(cfg.Proxy.Intercepts) == 0 {
		logrus.Println("No interceptors configured")
	}

	// Create a custom server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Proxy.Port),
		Handler: proxy,
	}

	return server
}

// CreateInterceptor creates an interceptor instance based on name
func CreateInterceptor(name string, store storage.Storage, timeout time.Duration) (interceptor2.Interceptor, error) {
	switch name {
	case "CustomInterceptor":
		return &interceptor2.CustomInterceptor{Name: name}, nil
	case "SimpleInterceptor":
		return &interceptor2.SimpleInterceptor{Name: name}, nil
	case "LoggingInterceptor":
		return &interceptor2.LoggingInterceptor{Name: name}, nil
	case "OllamaChatInterceptor":
		return &ollama2.ChatInterceptor{
			SavingInterceptor: interceptor2.SavingInterceptor{
				Name:    name,
				Storage: store,
				Timeout: timeout,
			},
		}, nil
	case "OllamaGenerateInterceptor":
		return &ollama2.GenerateInterceptor{
			SavingInterceptor: interceptor2.SavingInterceptor{
				Name:    name,
				Storage: store,
				Timeout: timeout,
			},
		}, nil
	default:
		return nil, fmt.Errorf("invalid interceptor type: %s", name)
	}
}
