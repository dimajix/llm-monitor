package main

import (
	"errors"
	"flag"
	"fmt"
	"llm-monitor/internal"
	"llm-monitor/internal/config"
	"net"
	"net/http"

	"github.com/sirupsen/logrus"
)

func main() {
	logrus.SetLevel(logrus.InfoLevel)
	logrus.SetFormatter(&logrus.TextFormatter{})

	// Define command line flag for config file path
	configFile := flag.String("c", "config.yaml", "Path to the config file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		logrus.WithError(err).Fatal("Could not load config file, terminating")
		return
	}

	internal.InitLogging(cfg.Logging)

	// Create a custom server
	server := internal.CreateServer(*cfg)
	defer func() {
		err := server.Close()
		if err != nil {
			return
		}
	}()

	// Set up a custom listener for better control
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		logrus.WithError(err).Fatal("Failed to create listener")
	}

	logrus.Println("Proxy server starting...")
	logrus.Println("Press Ctrl+C to stop")

	// Start the server
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logrus.WithError(err).Fatal("Server error")
	}

	// Log graceful shutdown
	logrus.Println("Server stopped gracefully")
}
