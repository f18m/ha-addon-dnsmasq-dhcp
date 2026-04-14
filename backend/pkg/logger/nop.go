// This package is a tiny wrapper on top of standard log.Logger interface
// and creates logs that mimic the dnsmasq logging style:
//
//	dnsmasq-dhcp[PID]: <UnixEpoch> <Message>
//
// with the difference that the timestamp is not in a (hard to read) UnixEpoch;
// the result looks like:
package logger

// NopCustomLogger is a no-op implementation of the CustomLogger interface.
type NopCustomLogger struct{}

func NewNopCustomLogger(prefix string) *NopCustomLogger {
	return &NopCustomLogger{}
}

// Info
func (l NopCustomLogger) Info(message string) {
	// NOP
}

// Infof
// Arguments are handled in the manner of [fmt.Printf].
func (l NopCustomLogger) Infof(format string, v ...any) {
	// NOP
}

// Warn
func (l NopCustomLogger) Warn(message string) {
	// NOP
}

// Warnf
// Arguments are handled in the manner of [fmt.Printf].
func (l NopCustomLogger) Warnf(format string, v ...any) {
	// NOP
}

// Fatal
func (l NopCustomLogger) Fatal(s string) {
	// NOP
}

// Fatal
// Arguments are handled in the manner of [fmt.Printf].
func (l NopCustomLogger) Fatalf(format string, v ...any) {
	// NOP
}
