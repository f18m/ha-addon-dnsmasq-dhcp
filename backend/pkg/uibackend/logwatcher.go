package uibackend

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"time"
)

// dnsmasqLogWarning describes a warning pattern to search for in dnsmasq logs
type dnsmasqLogWarning struct {
	// counterField is the name of the corresponding field in DnsmasqLogCounters
	counterField string
	pattern      *regexp.Regexp
}

// dnsmasqLogWarnings is the list of log patterns to watch for in dnsmasq logs
var dnsmasqLogWarnings = []dnsmasqLogWarning{
	{
		// dnsmasq emits this when it cannot use a configured static IP because another
		// client currently holds a lease for it; e.g.:
		//   "not using configured address 192.168.1.106 because it is leased to 1c:db:d4:13:89:f0"
		counterField: "not_using_configured_address",
		pattern:      regexp.MustCompile(`not using configured address`),
	},
}

// processLogLine checks a single log line against all warning patterns and increments
// the matching counters.
func (b *UIBackend) processLogLine(line string) {
	for _, w := range dnsmasqLogWarnings {
		if w.pattern.MatchString(line) {
			b.logCountersLock.Lock()
			switch w.counterField {
			case "not_using_configured_address":
				b.logCounters.NotUsingConfiguredAddress++
			}
			b.logCountersLock.Unlock()
			b.logger.Warnf("dnsmasq log warning [%s] detected: %s", w.counterField, line)
		}
	}
}

// watchDnsmasqLog tails the dnsmasq log file and calls processLogLine for each new line.
// It retries until the file becomes available, then follows it indefinitely.
// Intended to run in a separate goroutine.
func (b *UIBackend) watchDnsmasqLog(logFilePath string) {
	// Wait for the log file to appear (dnsmasq may not have started yet)
	var file *os.File
	var err error
	for {
		file, err = os.Open(logFilePath) //nolint:gosec
		if err == nil {
			break
		}
		b.logger.Warnf("dnsmasq log file %s not available yet, retrying in 1s: %s", logFilePath, err.Error())
		time.Sleep(1 * time.Second)
	}
	defer func() { _ = file.Close() }()

	// Seek to the end so we only process new lines written after startup
	if _, err = file.Seek(0, io.SeekEnd); err != nil {
		b.logger.Warnf("failed to seek dnsmasq log file: %s", err.Error())
	}

	reader := bufio.NewReader(file)
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			if readErr == io.EOF {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			b.logger.Warnf("error reading dnsmasq log: %s", readErr.Error())
			return
		}
		if line != "" {
			b.processLogLine(line)
		}
	}
}
