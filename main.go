package main

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"proxy/config"
	"proxy/handler"
	"proxy/interceptor"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Printf("Warning: Could not load config file: %v", err)
		log.Println("Using default configuration...")

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
		log.Fatal(err)
	}

	// Register interceptors based on configuration
	for _, intercept := range cfg.Intercepts {
		interceptorInstance := interceptor.CreateInterceptor(intercept.Interceptor)
		proxy.RegisterInterceptor(intercept.Endpoint, interceptorInstance)
		log.Printf("Registered interceptor %s for endpoint %s", intercept.Interceptor, intercept.Endpoint)
	}

	// Create a custom server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: proxy,
	}

	// Set up a custom listener for better control
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Proxy server starting...")
	log.Printf("Listening on port %d", cfg.Port)
	log.Printf("Upstream server: %s", cfg.Upstream)

	if len(cfg.Intercepts) > 0 {
		log.Println("Endpoints with interceptors:")
		for _, intercept := range cfg.Intercepts {
			log.Printf("  %s -> %s", intercept.Endpoint, intercept.Interceptor)
		}
	} else {
		log.Println("No interceptors configured")
	}
	log.Println("Press Ctrl+C to stop")

	// Start the server
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
