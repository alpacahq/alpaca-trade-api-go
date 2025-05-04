package stream

import (
	"log"
)

// Logger wraps methods for leveled, formatted logging.
type Logger interface {
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

type defaultLogger struct{}

func (*defaultLogger) Infof(format string, v ...interface{}) {
	log.Printf("INFO "+format, v...)
}

func (*defaultLogger) Warnf(format string, v ...interface{}) {
	log.Printf("WARN "+format, v...)
}

func (*defaultLogger) Errorf(format string, v ...interface{}) {
	log.Printf("ERROR "+format, v...)
}

// DefaultLogger returns a Logger that uses the standard go log package to
// print leveled logs to the standard error.
func DefaultLogger() Logger {
	return &defaultLogger{}
}

type errorOnlyLogger struct{}

var _ Logger = (*errorOnlyLogger)(nil)

func (*errorOnlyLogger) Infof(_ string, _ ...interface{}) {}
func (*errorOnlyLogger) Warnf(_ string, _ ...interface{}) {}
func (*errorOnlyLogger) Errorf(format string, v ...interface{}) {
	log.Printf(format, v...)
}

// ErrorOnlyLogger returns a Logger that only logs errors to the standard error.
func ErrorOnlyLogger() Logger {
	return &errorOnlyLogger{}
}
