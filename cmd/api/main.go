package main

import (
	"flag"
	"fmt"
	"llm-monitor/internal"
	"llm-monitor/internal/api"
	"llm-monitor/internal/config"
	"llm-monitor/internal/storage"
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

	store, err := storage.CreateStorage(cfg.Storage)
	if err != nil {
		logrus.WithError(err).Fatal("Failed to connect to storage")
	}

	apiHandler := api.NewAPIHandler(store)

	logrus.Infof("API server starting on port %d...", cfg.API.Port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.API.Port), apiHandler); err != nil {
		logrus.WithError(err).Fatal("API server failed")
	}
}
