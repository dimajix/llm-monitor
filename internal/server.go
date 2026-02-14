package internal

import (
	"fmt"
	"llm-monitor/internal/config"
	"llm-monitor/internal/handler"
	"llm-monitor/internal/interceptor"
	"llm-monitor/internal/interceptor/ollama"
	"net/http"

	"github.com/sirupsen/logrus"
)

func CreateServer(cfg config.Config) *http.Server {
	// Create proxy handler
	proxy, err := handler.NewProxyHandler(cfg.Upstream, cfg.Port)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create proxy handler")
	}

	// Register interceptors based on configuration
	for _, intercept := range cfg.Intercepts {
		interceptorInstance, err := CreateInterceptor(intercept.Interceptor)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to create proxy handler")
		}
		proxy.RegisterInterceptor(intercept.Endpoint, interceptorInstance)
		logrus.WithFields(logrus.Fields{
			"interceptor": intercept.Interceptor,
			"endpoint":    intercept.Endpoint,
		}).Info("Registered interceptor")
	}
	if len(cfg.Intercepts) == 0 {
		logrus.Println("No interceptors configured")
	}

	// Create a custom server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: proxy,
	}

	return server
}

// CreateInterceptor creates an interceptor instance based on name
func CreateInterceptor(name string) (interceptor.Interceptor, error) {
	switch name {
	case "CustomInterceptor":
		return &interceptor.CustomInterceptor{Name: name}, nil
	case "SimpleInterceptor":
		return &interceptor.SimpleInterceptor{Name: name}, nil
	case "LoggingInterceptor":
		return &interceptor.LoggingInterceptor{Name: name}, nil
	case "OllamaChatInterceptor":
		return &ollama.ChatInterceptor{Name: name}, nil
	case "OllamaGenerateInterceptor":
		return &ollama.GenerateInterceptor{Name: name}, nil
	default:
		return nil, fmt.Errorf("invalid interceptor type: %s", name)
	}
}
