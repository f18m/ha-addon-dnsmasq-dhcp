package uibackend

import (
	"bufio"
	"context"
	"dnsmasq-dhcp-backend/pkg/logger"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/miekg/dns"
)

// DnsmasqWrapper provides dnsmasq stats queries via CHAOS TXT records.
type DnsmasqWrapper struct {
	logger *logger.CustomLogger

	// counters for notable dnsmasq log messages, updated by the log-watcher goroutine
	logCounters     DnsmasqLogCounters
	logCountersLock sync.Mutex

	// PIDs of running dnsmasq processes, updated by the PID-monitor goroutine
	dnsmasqPID      int
	dnsmasqPIDsLock sync.Mutex
}

// DnsmasqLogCounters holds counters for notable dnsmasq log messages detected since startup.
type DnsmasqLogCounters struct {
	// NotUsingConfiguredAddress counts occurrences of the dnsmasq message
	// "not using configured address X because it is leased to Y".
	// A non-zero value means some DHCP clients are not receiving their configured static IP
	// because another device currently holds a lease for that address.
	NotUsingConfiguredAddress int `json:"not_using_configured_address"`
}

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

// NewDnsmasqWrapper returns an empty DnsmasqWrapper with the provided logger.
func NewDnsmasqWrapper(logger *logger.CustomLogger) *DnsmasqWrapper {
	d := DnsmasqWrapper{
		logger: logger,
	}

	// Initialize the PIDs list at startup
	d.updateDnsmasqPIDs()

	return &d
}

// chaosTXTQuery performs a DNS CHAOS TXT query against a specified DNS server.
func (w *DnsmasqWrapper) chaosTXTQuery(server, query string, timeout time.Duration) ([]string, error) {
	// Create a new DNS client.
	c := new(dns.Client)
	c.Timeout = timeout

	// Create a new DNS message.
	m := new(dns.Msg)
	m.Id = dns.Id()

	// Add the CHAOS TXT query to the message.
	m.Question = append(m.Question, dns.Question{
		Name:   query + ".",
		Qtype:  dns.TypeTXT,
		Qclass: dns.ClassCHAOS,
	})

	// Send the DNS query.
	r, _, err := c.ExchangeContext(context.Background(), m, server)
	if err != nil {
		return nil, err
	}

	// Check for errors in the response.
	if r.Rcode != dns.RcodeSuccess {
		return nil, fmt.Errorf("invalid answer name %s after querying for %s: %s", query, server, dns.RcodeToString[r.Rcode])
	}

	// Extract the TXT record from the response.
	var txt []string
	for _, ans := range r.Answer {
		if t, ok := ans.(*dns.TXT); ok {
			txt = append(txt, t.Txt...)
			break
		}
	}

	return txt, nil
}

func (w *DnsmasqWrapper) chaosTXTQueryInteger(server, query string, timeout time.Duration) (int, error) { //nolint:unparam
	// Invoke chaosTXTQuery to get the string value.
	strVal, err := w.chaosTXTQuery(server, query, timeout)
	if err != nil {
		return 0, err
	}
	if len(strVal) != 1 {
		return 0, err
	}

	// Convert the string value to an integer.
	var intVal int
	_, err = fmt.Sscan(strVal[0], &intVal)
	if err != nil {
		return 0, fmt.Errorf("failed to convert TXT record '%s' to integer: %w", strVal, err)
	}

	return intVal, nil
}

// getDnsStats queries the external dnsmasq process for its internal DNS stats and
// returns them in a structured format.
func (w *DnsmasqWrapper) getDnsStats(serverHost string, serverPort int) (DnsServerStats, error) {
	dnsServer := fmt.Sprintf("%s:%d", serverHost, serverPort)

	// since the server is local, the max query duration is expected to be small
	dnsTimeout := 500 * time.Millisecond

	ret := DnsServerStats{}

	// From dnsmasq manpage:
	// "The domain names are cachesize.bind, insertions.bind, evictions.bind, misses.bind,
	// hits.bind, auth.bind and servers.bind unless disabled at compile-time."

	// Start querying all cache-related stats
	var intStat int
	var err, lastErr error
	intStat, err = w.chaosTXTQueryInteger(dnsServer, "cachesize.bind", dnsTimeout)
	if err == nil {
		ret.CacheSize = intStat
	} else {
		lastErr = err
	}
	intStat, err = w.chaosTXTQueryInteger(dnsServer, "insertions.bind", dnsTimeout)
	if err == nil {
		ret.CacheInsertions = intStat
	} else {
		lastErr = err
	}
	intStat, err = w.chaosTXTQueryInteger(dnsServer, "evictions.bind", dnsTimeout)
	if err == nil {
		ret.CacheEvictions = intStat
	} else {
		lastErr = err
	}
	intStat, err = w.chaosTXTQueryInteger(dnsServer, "misses.bind", dnsTimeout)
	if err == nil {
		ret.CacheMisses = intStat
	} else {
		lastErr = err
	}
	intStat, err = w.chaosTXTQueryInteger(dnsServer, "hits.bind", dnsTimeout)
	if err == nil {
		ret.CacheHits = intStat
	} else {
		lastErr = err
	}

	// Interpret the servers.bind output
	var serversEncodedStr []string
	serversEncodedStr, err = w.chaosTXTQuery(dnsServer, "servers.bind", dnsTimeout)
	if err != nil {
		lastErr = err
	}
	for _, svrStat := range serversEncodedStr {
		// srvStat would look like "8.8.8.8#53 30048 0"
		fields := strings.Fields(svrStat)
		if len(fields) == 3 {
			svr := fields[0]
			queries, err := strconv.Atoi(fields[1])
			if err != nil {
				return ret, fmt.Errorf("failed to convert queries to integer: %w", err)
			}
			failures, err := strconv.Atoi(fields[2])
			if err != nil {
				return ret, fmt.Errorf("failed to convert failures to integer: %w", err)
			}
			ret.UpstreamServers = append(ret.UpstreamServers, DnsUpstreamStats{
				ServerURL:     svr,
				QueriesSent:   queries,
				QueriesFailed: failures,
			})
		}
	}

	return ret, lastErr
}

// processLogLine checks a single log line against all warning patterns and increments
// the matching counters.
func (b *DnsmasqWrapper) processLogLine(line string) {
	for _, w := range dnsmasqLogWarnings {
		if w.pattern.MatchString(line) {
			b.logCountersLock.Lock()
			switch w.counterField { //nolint:gocritic
			case "not_using_configured_address":
				b.logCounters.NotUsingConfiguredAddress++
			}
			b.logCountersLock.Unlock()
			b.logger.Warnf("dnsmasq log warning [%s] detected: %s", w.counterField, line)
		}
	}

	// print the log line to stderr -- this is mimicking a "tee" instance
	// running on the dnsmasq log file.
	fmt.Fprint(os.Stderr, line)
}

// watchDnsmasqLog tails the dnsmasq log file and calls processLogLine for each new line.
// It retries until the file becomes available, then follows it indefinitely.
// Intended to run in a separate goroutine.
func (b *DnsmasqWrapper) watchDnsmasqLog(logFilePath string) {
	// Wait for the log file to appear (dnsmasq may not have started yet)
	var file *os.File
	var err error
	for {
		// open in read-write so we can truncate it later
		file, err = os.OpenFile(logFilePath, os.O_RDWR, 0) //nolint:gosec
		if err == nil {
			break
		}
		b.logger.Warnf("dnsmasq log file %s not available yet, retrying in 1s: %s", logFilePath, err.Error())
		time.Sleep(1 * time.Second)
	}
	defer func() { _ = file.Close() }()
	/*
		// Seek to the end so we only process new lines written after startup
		if _, err = file.Seek(0, io.SeekEnd); err != nil {
			b.logger.Warnf("failed to seek dnsmasq log file: %s", err.Error())
		}
	*/
	const maxLinesBeforeTruncate = 2000
	linesRead := 0

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
			linesRead++

			// Truncate the log file periodically to prevent unbounded growth
			if linesRead >= maxLinesBeforeTruncate {

				// make sure we know the PID of the dnsmasq process before truncating the log
				if !b.updateDnsmasqPIDs() {
					// failed somehow... skip any truncation
					continue
				}

				if err := file.Truncate(0); err != nil {
					b.logger.Warnf("failed to truncate dnsmasq log file: %s", err.Error())
				}

				_, err = file.Seek(0, io.SeekStart)
				if err != nil {
					b.logger.Warnf("failed to seek to the start of dnsmasq log file: %s", err.Error())
					continue
				}

				/*
						offset, err := file.Seek(0, io.SeekCurrent)
						if err != nil {
							b.logger.Warnf("failed to get current offset in dnsmasq log file: %s", err.Error())
							continue
						}

						b.logger.Infof("truncating dnsmasq log file after %d lines, at offset %d", linesRead, offset)


					/*
						_ = file.Close()
						if err := os.Remove(logFilePath, 0); err != nil {
							b.logger.Warnf("failed to truncate dnsmasq log file: %s", err.Error())
						}
				*/
				reader.Reset(file)
				linesRead = 0

				// send SIGUSR2 to dnsmasq to reopen the log file
				_ = syscall.Kill(b.GetDnsmasqPID(), syscall.SIGUSR2)
			}
		}
	}
}

func (w *DnsmasqWrapper) GetLogCounters() DnsmasqLogCounters {
	w.logCountersLock.Lock()
	defer w.logCountersLock.Unlock()
	return w.logCounters
}

// updateDnsmasqPIDs searches for all running dnsmasq processes and updates the PIDs list.
// It iterates through /proc to find processes with "dnsmasq" in their command line.
// Intended to run in a separate goroutine and periodically poll for process changes.
func (w *DnsmasqWrapper) updateDnsmasqPIDs() bool {
	procDir := "/proc"
	entries, err := os.ReadDir(procDir)
	if err != nil {
		w.logger.Warnf("failed to read /proc directory: %s", err.Error())
		return false
	}

	var pids []int

	// Iterate through all entries in /proc
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Try to parse the directory name as a PID
		pidStr := entry.Name()
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			// Not a PID directory, skip
			continue
		}

		// Read the cmdline file to check if this is a dnsmasq process
		cmdlineFile := fmt.Sprintf("%s/%s/cmdline", procDir, pidStr)
		cmdline, err := os.ReadFile(cmdlineFile) //nolint:gosec
		if err != nil {
			// Process may have exited or we don't have permission, skip
			continue
		}

		// The cmdline file contains null-terminated arguments; convert to a searchable string
		cmdlineStr := strings.ReplaceAll(string(cmdline), "\x00", " ")
		if strings.Contains(cmdlineStr, "dnsmasq") {
			pids = append(pids, pid)
		}
	}

	switch len(pids) {
	case 0:
		w.logger.Warnf("no dnsmasq processes found")
		return false

	case 1:
		w.logger.Infof("found the dnsmasq process having PID: %v", pids[0])

		// Update the PIDs list with the new values
		w.dnsmasqPIDsLock.Lock()
		w.dnsmasqPID = pids[0]
		w.dnsmasqPIDsLock.Unlock()

		return true

	default:
		w.logger.Warnf("found multiple dnsmasq processes with PIDs: %v; this is unexpected", pids)
		return false
	}
}

// GetDnsmasqPID returns the PID of the current dnsmasq process.
func (w *DnsmasqWrapper) GetDnsmasqPID() int {
	w.dnsmasqPIDsLock.Lock()
	defer w.dnsmasqPIDsLock.Unlock()
	// Return a copy to prevent external modifications
	if w.dnsmasqPID == 0 {
		return 0
	}
	return w.dnsmasqPID
}
