package uibackend

import (
	"encoding/json"
	"net"
	"net/netip"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	testCases := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"1h", time.Hour, false},
		{"1.5h", 90 * time.Minute, false},
		{"1d", 24 * time.Hour, false},
		{"2w", 14 * 24 * time.Hour, false},
		{"1M", 30 * 24 * time.Hour, false}, // Assuming 30 days in a month
		{"1y", 365 * 24 * time.Hour, false},
		{"-1h", -time.Hour, false},
		{"-1.5h", -90 * time.Minute, false},
		{"-2w", -14 * 24 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"1.5h30m", 120 * time.Minute, false},                 // mixed units
		{"2.5d", 60 * time.Hour, false},                       // decimal days
		{"1.5h30m2s", 120*time.Minute + 2*time.Second, false}, // more complex cases
		{"2D", 48 * time.Hour, false},
		{"1W", 7 * 24 * time.Hour, false},
		{"1Y", 365 * 24 * time.Hour, false},
		{"-2D", -48 * time.Hour, false},
		{"-1W", -7 * 24 * time.Hour, false},

		// error cases
		{"", 0, true},         // empty string
		{"invalid", 0, true},  // invalid input
		{"1.5h 30m", 0, true}, // space between values
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got, err := parseDuration(tc.input)
			if (err != nil) != tc.wantErr {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tc.input, err, tc.wantErr)
				return
			}
			if got != tc.expected {
				t.Errorf("parseDuration(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestAddonOptionsUnmarshalJSONWithWhitespace(t *testing.T) {
	// Test that whitespace is properly stripped from IP and MAC address fields
	jsonConfig := `{
		"dhcp_pools": [
			{
				"interface": "eth0",
				"start": " 192.168.1.50",
				"end": "192.168.1.100 ",
				"gateway": " 192.168.1.1 ",
				"netmask": " 255.255.255.0 "
			}
		],
		"dhcp_ip_address_reservations": [
			{
				"ip": " 192.168.1.10 ",
				"mac": " aa:bb:cc:dd:ee:ff ",
				"name": "test-host"
			}
		],
		"dhcp_clients_friendly_names": [
			{
				"mac": " 11:22:33:44:55:66 ",
				"name": "friendly-host"
			}
		],
		"dhcp_server": {
			"default_lease": "1h",
			"address_reservation_lease": "1h",
			"forget_past_clients_after": "30d",
			"log_requests": true
		},
		"dns_server": {
			"enable": true,
			"dns_domain": "lan",
			"port": 53
		},
		"web_ui": {
			"log_activity": false,
			"port": 8976,
			"refresh_interval_sec": 10
		}
	}`

	var opts AddonOptions
	opts.ipAddressReservationsByIP = make(map[netip.Addr]IpAddressReservation)
	opts.ipAddressReservationsByMAC = make(map[string]IpAddressReservation)
	opts.friendlyNames = make(map[string]DhcpClientFriendlyName)

	err := json.Unmarshal([]byte(jsonConfig), &opts)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON with whitespace: %v", err)
	}

	// Verify that the DHCP pool was parsed correctly despite whitespace
	if len(opts.dhcpRanges) != 1 {
		t.Fatalf("Expected 1 DHCP range, got %d", len(opts.dhcpRanges))
	}

	dhcpRange := opts.dhcpRanges[0]

	// Check that gateway was parsed correctly (whitespace trimmed)
	expectedGateway := net.ParseIP("192.168.1.1")
	if !dhcpRange.Gateway.Equal(expectedGateway) {
		t.Errorf("Expected gateway %v, got %v", expectedGateway, dhcpRange.Gateway)
	}

	// Check that netmask was parsed correctly (whitespace trimmed)
	expectedNetmask := net.IPMask(net.ParseIP("255.255.255.0").To4())
	if !net.IP(dhcpRange.Netmask).Equal(net.IP(expectedNetmask)) {
		t.Errorf("Expected netmask %v, got %v", expectedNetmask, dhcpRange.Netmask)
	}

	// Check that start/end IPs were parsed correctly (whitespace trimmed)
	expectedStart := net.ParseIP("192.168.1.50")
	expectedEnd := net.ParseIP("192.168.1.100")
	if !dhcpRange.Start.Equal(expectedStart) {
		t.Errorf("Expected start IP %v, got %v", expectedStart, dhcpRange.Start)
	}
	if !dhcpRange.End.Equal(expectedEnd) {
		t.Errorf("Expected end IP %v, got %v", expectedEnd, dhcpRange.End)
	}

	// Verify IP address reservation was parsed correctly
	if len(opts.ipAddressReservationsByIP) != 1 {
		t.Fatalf("Expected 1 IP reservation, got %d", len(opts.ipAddressReservationsByIP))
	}

	// Verify friendly name was parsed correctly
	if len(opts.friendlyNames) != 1 {
		t.Fatalf("Expected 1 friendly name, got %d", len(opts.friendlyNames))
	}
}
