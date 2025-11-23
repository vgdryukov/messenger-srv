package server

import (
	"log"
	"os"
)

type Logger struct {
	*log.Logger
	environment string
}

func NewLogger(environment string) *Logger {
	return &Logger{
		Logger:      log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds),
		environment: environment,
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	l.Printf("INFO: "+format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.Printf("ERROR: "+format, v...)

	// В production можно добавить отправку в Sentry/ELK
	if l.environment == "production" {
		// integration with external monitoring
	}
}

func (l *Logger) Debug(format string, v ...interface{}) {
	if l.environment == "development" {
		l.Printf("DEBUG: "+format, v...)
	}
}

func (l *Logger) Warn(format string, v ...interface{}) {
	l.Printf("WARN: "+format, v...)
}
