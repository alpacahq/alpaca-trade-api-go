package stream

import (
	"log"
	"os"
)

type Logger interface {
	Infof(format string, v ...interface{})
	Warnf(format string, v ...interface{})
	Errorf(format string, v ...interface{})
}

type stdLog struct {
	logger *log.Logger
}

var _ Logger = (*stdLog)(nil)

func (s *stdLog) Infof(format string, v ...interface{}) {
	// NOTE: there is no concept of levels in log
}

func (s *stdLog) Warnf(format string, v ...interface{}) {
	// NOTE: there is no concept of levels in log
}

func (s *stdLog) Errorf(format string, v ...interface{}) {
	s.logger.Printf(format, v...)
}

func newStdLog() Logger {
	// Note log.Default() is also available in go 1.16
	return &stdLog{logger: log.New(os.Stderr, "", log.LstdFlags)}
}
