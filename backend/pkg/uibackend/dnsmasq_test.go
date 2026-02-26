package uibackend

import (
	"context"
	"dnsmasq-dhcp-backend/pkg/logger"
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestGetDnsStats_NoUpstreamServers(t *testing.T) {
	dnsmasq := NewDnsmasqWrapper(logger.NewCustomLogger("unit tests"))

	// Start a temporary dnsmasq instance
	dnsmasqCmd := exec.CommandContext(context.Background(), "dnsmasq", "--port=12345", "--cache-size=100", "--no-daemon", "--no-resolv") // Adjust arguments as needed
	if err := dnsmasqCmd.Start(); err != nil {
		t.Fatalf("Failed to start dnsmasq: %v", err)
	}
	defer func() {
		if err := dnsmasqCmd.Process.Kill(); err != nil {
			t.Errorf("Failed to kill dnsmasq: %v", err)
		}
	}()

	// Wait for dnsmasq to start listening
	for i := 0; i < 10; i++ {
		if conn, err := net.DialTimeout("tcp", "localhost:12345", 1*time.Second); err == nil { //nolint:noctx
			_ = conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	stats, err := dnsmasq.GetDnsStats("localhost", 12345)
	if err != nil {
		t.Fatalf("GetDnsStats failed: %v", err)
	}

	// Assertions
	if stats.CacheSize != 100 {
		t.Errorf("Unexpected CacheSize: got %d, want %d", stats.CacheSize, 100)
	}

	// Check for upstream servers.  Since we started with --no-resolv, there shouldn't be any upstream servers initially.
	if len(stats.UpstreamServers) != 0 {
		t.Errorf("Unexpected Upstream Servers found: %v", stats.UpstreamServers)
	}
}

func TestGetDnsStats_WithUpstreamServers(t *testing.T) {
	dnsmasq := NewDnsmasqWrapper(logger.NewCustomLogger("unit tests"))

	// Test with resolv-file, simulating an upstream server
	resolvFileContent := "nameserver 8.8.4.4" // Example upstream server
	resolvFilePath := "/tmp/resolv.conf"      // Choose a temporary file
	err := writeTempFile(resolvFilePath, resolvFileContent)
	if err != nil {
		t.Fatalf("Failed to write temporary resolv file: %v", err)
	}

	// Restart dnsmasq with the resolv file
	dnsmasqCmd := exec.CommandContext(context.Background(), "dnsmasq", "--port=12346", "--cache-size=100", "--no-daemon", fmt.Sprintf("--resolv-file=%s", resolvFilePath)) //nolint:gosec
	if err := dnsmasqCmd.Start(); err != nil {
		t.Fatalf("Failed to restart dnsmasq with resolv-file: %v", err)
	}
	defer func() {
		if err := dnsmasqCmd.Process.Kill(); err != nil {
			t.Errorf("Failed to kill dnsmasq: %v", err)
		}
	}()
	for i := 0; i < 10; i++ {
		if conn, err := net.DialTimeout("tcp", "localhost:12346", 1*time.Second); err == nil { //nolint:noctx
			_ = conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	stats, err := dnsmasq.GetDnsStats("localhost", 12346)
	if err != nil {
		t.Fatalf("getDnsStats failed with resolv-file: %v", err)
	}
	if len(stats.UpstreamServers) != 1 {
		t.Errorf("Expected Upstream Servers but found none")
	}
	if stats.UpstreamServers[0].ServerURL != "8.8.4.4#53" {
		t.Errorf("Expected google upstream Servers but found something else")
	}
}

// Helper function to write to a temporary file
func writeTempFile(filePath, content string) error {
	file, err := os.Create(filePath) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	_, err = file.WriteString(content)
	return err
}

func TestProcessLogLine_NotUsingConfiguredAddress(t *testing.T) {
	dnsmasq := NewDnsmasqWrapper(logger.NewCustomLogger("unit tests"))

	// Simulate a dnsmasq log line matching the "not using configured address" pattern
	dnsmasq.processLogLine("dnsmasq-dhcp[1234]: not using configured address 192.168.1.106 because it is leased to 1c:db:d4:13:89:f0\n")

	dnsmasq.logCountersLock.Lock()
	count := dnsmasq.logCounters.NotUsingConfiguredAddress
	dnsmasq.logCountersLock.Unlock()

	if count != 1 {
		t.Errorf("expected NotUsingConfiguredAddress=1, got %d", count)
	}
}

func TestProcessLogLine_NoMatch(t *testing.T) {
	dnsmasq := NewDnsmasqWrapper(logger.NewCustomLogger("unit tests"))

	// A normal dnsmasq log line that should not match any warning pattern
	dnsmasq.processLogLine("dnsmasq-dhcp[1234]: DHCPREQUEST(eth0) 192.168.1.50 aa:bb:cc:dd:ee:ff\n")

	dnsmasq.logCountersLock.Lock()
	count := dnsmasq.logCounters.NotUsingConfiguredAddress
	dnsmasq.logCountersLock.Unlock()

	if count != 0 {
		t.Errorf("expected NotUsingConfiguredAddress=0, got %d", count)
	}
}

func TestProcessLogLine_MultipleMatches(t *testing.T) {
	dnsmasq := NewDnsmasqWrapper(logger.NewCustomLogger("unit tests"))

	lines := []string{
		"dnsmasq-dhcp[1234]: not using configured address 192.168.1.100 because it is leased to aa:bb:cc:dd:ee:ff\n",
		"dnsmasq-dhcp[1234]: DHCPREQUEST(eth0) 192.168.1.50 11:22:33:44:55:66\n",
		"dnsmasq-dhcp[1234]: not using configured address 192.168.1.200 because it is leased to 11:22:33:44:55:66\n",
	}
	for _, line := range lines {
		dnsmasq.processLogLine(line)
	}

	dnsmasq.logCountersLock.Lock()
	count := dnsmasq.logCounters.NotUsingConfiguredAddress
	dnsmasq.logCountersLock.Unlock()

	if count != 2 {
		t.Errorf("expected NotUsingConfiguredAddress=2, got %d", count)
	}
}
