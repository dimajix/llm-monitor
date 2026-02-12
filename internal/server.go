package internal

import (
	"fmt"
	"llm-monitor/internal/config"
	"llm-monitor/internal/handler"
	"llm-monitor/internal/interceptor"
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
		interceptorInstance := interceptor.CreateInterceptor(intercept.Interceptor)
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
