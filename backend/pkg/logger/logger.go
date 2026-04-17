// This package is a tiny wrapper on top of standard log.Logger interface
// and creates logs that mimic the dnsmasq logging style:
//
//	dnsmasq-dhcp[PID]: <UnixEpoch> <Message>
//
// with the difference that the timestamp is not in a (hard to read) UnixEpoch;
// the result looks like:
package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

// CustomLoggerWithPidPrefix implements the CustomLogger interface
type CustomLoggerWithPidPrefix struct {
	logger *log.Logger
	pid    int
	prefix string
}

func NewCustomLoggerWithPidPrefix(prefix string) *CustomLoggerWithPidPrefix {
	pid := os.Getpid()
	logger := log.New(os.Stdout, "", 0) // No flags here, we'll add timestamp manually
	return &CustomLoggerWithPidPrefix{
		logger: logger,
		pid:    pid,
		prefix: prefix,
	}
}

func (l CustomLoggerWithPidPrefix) Log(level LogLevel, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("%s[%d]: %s %s %s", l.prefix, l.pid, timestamp, level, message)
	l.logger.Print(logMessage)
}

// Info
func (l CustomLoggerWithPidPrefix) Info(message string) {
	l.Log(INFO, message)
}

// Infof
// Arguments are handled in the manner of [fmt.Printf].
func (l CustomLoggerWithPidPrefix) Infof(format string, v ...any) {
	l.Info(fmt.Sprintf(format, v...))
}

// Warn
func (l CustomLoggerWithPidPrefix) Warn(message string) {
	l.Log(WARN, message)
}

// Warnf
// Arguments are handled in the manner of [fmt.Printf].
func (l CustomLoggerWithPidPrefix) Warnf(format string, v ...any) {
	l.Warn(fmt.Sprintf(format, v...))
}

// Fatal
func (l CustomLoggerWithPidPrefix) Fatal(s string) {
	l.Log(FATAL, s)
}

// Fatal
// Arguments are handled in the manner of [fmt.Printf].
func (l CustomLoggerWithPidPrefix) Fatalf(format string, v ...any) {
	l.Fatal(fmt.Sprintf(format, v...))
}
