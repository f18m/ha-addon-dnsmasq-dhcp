// This package is a tiny wrapper on top of standard log.Logger interface
// and creates logs that mimic the dnsmasq logging style:
//
//	dnsmasq-dhcp[PID]: <UnixEpoch> <Message>
//
// with the difference that the timestamp is not in a (hard to read) UnixEpoch;
// the result looks like:
package logger

type LogLevel string

const (
	INFO  LogLevel = "INFO"
	WARN  LogLevel = "WARN"
	FATAL LogLevel = "FATAL"
)

type CustomLogger interface {
	// Log(level LogLevel, message string)
	Info(message string)
	Infof(format string, v ...any)
	Warn(message string)
	Warnf(format string, v ...any)
	Fatal(message string)
	Fatalf(format string, v ...any)
}
