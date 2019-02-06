// Package log provide loggers
package log

import (
	"os"

	"github.com/juju/loggo"
)

// GetLogger returns a logger handler for defined package
//
// Log level is INFO by default, to activate DEBUG, environment variable LOGOL_DEBUG must be set to 1 or true
func GetLogger(packageName string) (logger loggo.Logger) {
	logger = loggo.GetLogger(packageName)
	osDebug := os.Getenv("HERO_DEBUG")
	if osDebug != "" && osDebug != "0" {
		logger.SetLogLevel(loggo.DEBUG)
	} else {
		logger.SetLogLevel(loggo.INFO)
	}
	return logger
}
