package main

import (
	"fmt"
	"llm-sniffer/config"
	"llm-sniffer/handler"
	"llm-sniffer/interceptor"
	"net"
	"net/http"

	"github.com/sirupsen/logrus"
)

func main() {
	// Set log level
	logrus.SetLevel(logrus.InfoLevel)

	// Set log format to JSON for better structured logging
	logrus.SetFormatter(&logrus.JSONFormatter{})

	// Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		logrus.WithError(err).Warn("Could not load config file, using default configuration")
		logrus.Println("Using default configuration...")

		// Default configuration
		cfg = &config.Config{
			Port:     8080,
			Upstream: "http://httpbin.org",
			Intercepts: []config.InterceptConfig{
				{Endpoint: "/api/users", Interceptor: "CustomInterceptor"},
				{Endpoint: "/api/products", Interceptor: "SimpleInterceptor"},
				{Endpoint: "/api/logs", Interceptor: "LoggingInterceptor"},
			},
		}
	}

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

	// Create a custom server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: proxy,
	}

	// Set up a custom listener for better control
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create listener")
	}

	logrus.Println("Proxy server starting...")
	logrus.WithFields(logrus.Fields{
		"port":     cfg.Port,
		"upstream": cfg.Upstream,
	}).Info("Server configuration")

	if len(cfg.Intercepts) > 0 {
		logrus.Println("Endpoints with interceptors:")
		for _, intercept := range cfg.Intercepts {
			logrus.WithFields(logrus.Fields{
				"endpoint":    intercept.Endpoint,
				"interceptor": intercept.Interceptor,
			}).Info("Interceptor configuration")
		}
	} else {
		logrus.Println("No interceptors configured")
	}

	logrus.Println("Press Ctrl+C to stop")

	// Start the server
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		logrus.WithError(err).Fatal("Server error")
	}

	// Log graceful shutdown
	logrus.Println("Server stopped gracefully")
}
