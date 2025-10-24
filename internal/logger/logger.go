package logger

import (
	"log"
	"os"
)

var InfoLogger,
	ErrorLogger,
	WarnLogger *log.Logger

func init() {
	// Use standard library loggers; info -> stdout, errors -> stderr.
	InfoLogger = log.New(os.Stdout, "INFO: ", log.LstdFlags)
	ErrorLogger = log.New(os.Stderr, "ERROR: ", log.LstdFlags)
	WarnLogger = log.New(os.Stderr, "WARN: ", log.LstdFlags)
}
