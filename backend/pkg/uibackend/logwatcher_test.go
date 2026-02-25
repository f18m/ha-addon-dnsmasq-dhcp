package uibackend

import (
	"testing"
)

func TestProcessLogLine_NotUsingConfiguredAddress(t *testing.T) {
	backend := getMockUIBackend()

	// Simulate a dnsmasq log line matching the "not using configured address" pattern
	backend.processLogLine("dnsmasq-dhcp[1234]: not using configured address 192.168.1.106 because it is leased to 1c:db:d4:13:89:f0\n")

	backend.logCountersLock.Lock()
	count := backend.logCounters.NotUsingConfiguredAddress
	backend.logCountersLock.Unlock()

	if count != 1 {
		t.Errorf("expected NotUsingConfiguredAddress=1, got %d", count)
	}
}

func TestProcessLogLine_NoMatch(t *testing.T) {
	backend := getMockUIBackend()

	// A normal dnsmasq log line that should not match any warning pattern
	backend.processLogLine("dnsmasq-dhcp[1234]: DHCPREQUEST(eth0) 192.168.1.50 aa:bb:cc:dd:ee:ff\n")

	backend.logCountersLock.Lock()
	count := backend.logCounters.NotUsingConfiguredAddress
	backend.logCountersLock.Unlock()

	if count != 0 {
		t.Errorf("expected NotUsingConfiguredAddress=0, got %d", count)
	}
}

func TestProcessLogLine_MultipleMatches(t *testing.T) {
	backend := getMockUIBackend()

	lines := []string{
		"dnsmasq-dhcp[1234]: not using configured address 192.168.1.100 because it is leased to aa:bb:cc:dd:ee:ff\n",
		"dnsmasq-dhcp[1234]: DHCPREQUEST(eth0) 192.168.1.50 11:22:33:44:55:66\n",
		"dnsmasq-dhcp[1234]: not using configured address 192.168.1.200 because it is leased to 11:22:33:44:55:66\n",
	}
	for _, line := range lines {
		backend.processLogLine(line)
	}

	backend.logCountersLock.Lock()
	count := backend.logCounters.NotUsingConfiguredAddress
	backend.logCountersLock.Unlock()

	if count != 2 {
		t.Errorf("expected NotUsingConfiguredAddress=2, got %d", count)
	}
}
