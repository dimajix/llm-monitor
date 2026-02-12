package internal

import (
	"llm-monitor/internal/config"

	"github.com/sirupsen/logrus"
)

func InitLogging(cfg config.Logging) {
	// Set log level
	logrus.SetLevel(logrus.InfoLevel)

	// Set log format according to configuration
	switch cfg.Format {
	case "json":
		logrus.SetFormatter(&logrus.JSONFormatter{})
	default: // Default to text format
		logrus.SetFormatter(&logrus.TextFormatter{})
	}
}
