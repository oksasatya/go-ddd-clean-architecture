package helpers

import (
	"os"

	"github.com/sirupsen/logrus"
)

// NewLogger creates a configured Logrus logger
func NewLogger(appName, env string) *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	if env == "development" {
		logger.SetLevel(logrus.DebugLevel)
		logger.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	} else {
		logger.SetLevel(logrus.InfoLevel)
		logger.SetFormatter(&logrus.JSONFormatter{})
	}
	logger.WithFields(logrus.Fields{"app": appName, "env": env}).Info("logger initialized")
	return logger
}

// LogError Convenience methods to keep a unified logging interface
func LogError(logger *logrus.Logger, msg string, err error, fields logrus.Fields) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	if err != nil {
		fields["error"] = err.Error()
	}
	logger.WithFields(fields).Error(msg)
}

func LogInfo(logger *logrus.Logger, msg string, fields logrus.Fields) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	logger.WithFields(fields).Info(msg)
}
