package utils

import (
	"os"

	log "github.com/zerobotlabs/relax/Godeps/_workspace/src/github.com/Sirupsen/logrus"
)

func SetupLogging() {
	level := os.Getenv("RELAX_LOG_LEVEL")
	if level == "" {
		level = "debug"
	}
	parsedLevel, err := log.ParseLevel(level)
	if err != nil {
		parsedLevel = log.DebugLevel
	}

	log.SetLevel(parsedLevel)
}
