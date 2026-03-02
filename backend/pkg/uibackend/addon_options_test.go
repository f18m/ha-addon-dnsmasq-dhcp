package uibackend

import (
	"encoding/json"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestIsValidRFC1123Hostname(t *testing.T) {
	testCases := []struct {
		input  string
		wantOK bool
	}{
		{"myhost", true},
		{"my-host", true},
		{"my-host-123", true},
		{"a", true},
		{"abc123", true},
		// exactly 63 chars (valid)
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		// 64 chars (invalid)
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", false},
		// empty string
		{"", false},
		// starts with hyphen
		{"-myhost", false},
		// ends with hyphen
		{"myhost-", false},
		// contains underscore
		{"my_host", false},
		// contains space
		{"my host", false},
		// contains dot (FQDN not allowed here)
		{"my.host", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got := isValidRFC1123Hostname(tc.input)
			if got != tc.wantOK {
				t.Errorf("isValidRFC1123Hostname(%q) = %v, want %v", tc.input, got, tc.wantOK)
			}
		})
	}
}

func TestAddonOptionsInvalidHostname(t *testing.T) {
	baseConfig := func(name string) string {
		return `{
			"dhcp_pools": [
				{
					"interface": "eth0",
					"start": "192.168.1.50",
					"end": "192.168.1.100",
					"gateway": "192.168.1.1",
					"netmask": "255.255.255.0"
				}
			],
			"dhcp_ip_address_reservations": [
				{
					"ip": "192.168.1.10",
					"mac": "aa:bb:cc:dd:ee:ff",
					"name": "` + name + `"
				}
			],
			"dhcp_clients_friendly_names": [],
			"dhcp_server": {
				"default_lease": "1h",
				"address_reservation_lease": "1h",
				"forget_past_clients_after": "30d",
				"log_requests": false
			},
			"dns_server": {
				"enable": false,
				"dns_domain": "lan",
				"port": 53
			},
			"web_ui": {
				"log_activity": false,
				"port": 8976,
				"refresh_interval_sec": 10
			}
		}`
	}

	invalidNames := []string{
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 64 chars
		"-badstart",
		"badend-",
		"has space",
		"has.dot",
		"",
	}
	for _, name := range invalidNames {
		t.Run("invalid:"+name, func(t *testing.T) {
			var opts AddonOptions
			opts.ipAddressReservationsByIP = make(map[netip.Addr]IpAddressReservation)
			opts.ipAddressReservationsByMAC = make(map[string]IpAddressReservation)
			opts.friendlyNames = make(map[string]DhcpClientFriendlyName)
			err := json.Unmarshal([]byte(baseConfig(name)), &opts)
			if err == nil {
				t.Errorf("expected error for hostname %q, but got none", name)
			}
		})
	}

	validNames := []string{
		"myhost",
		"my-host-123",
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", // 63 chars
	}
	for _, name := range validNames {
		t.Run("valid:"+name, func(t *testing.T) {
			var opts AddonOptions
			opts.ipAddressReservationsByIP = make(map[netip.Addr]IpAddressReservation)
			opts.ipAddressReservationsByMAC = make(map[string]IpAddressReservation)
			opts.friendlyNames = make(map[string]DhcpClientFriendlyName)
			err := json.Unmarshal([]byte(baseConfig(name)), &opts)
			if err != nil {
				t.Errorf("unexpected error for hostname %q: %v", name, err)
			}
		})
	}
}

func TestAddonOptionsMACInBothLists(t *testing.T) {
	// A MAC address that appears in both dhcp_ip_address_reservations and
	// dhcp_clients_friendly_names must be rejected.
	jsonConfig := `{
		"dhcp_pools": [
			{
				"interface": "eth0",
				"start": "192.168.1.50",
				"end": "192.168.1.100",
				"gateway": "192.168.1.1",
				"netmask": "255.255.255.0"
			}
		],
		"dhcp_ip_address_reservations": [
			{
				"ip": "192.168.1.10",
				"mac": "aa:bb:cc:dd:ee:ff",
				"name": "reserved-host"
			}
		],
		"dhcp_clients_friendly_names": [
			{
				"mac": "aa:bb:cc:dd:ee:ff",
				"name": "friendly-host"
			}
		],
		"dhcp_server": {
			"default_lease": "1h",
			"address_reservation_lease": "1h",
			"forget_past_clients_after": "30d",
			"log_requests": false
		},
		"dns_server": {
			"enable": false,
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
	if err == nil {
		t.Error("expected error when the same MAC appears in both lists, but got none")
	} else if !strings.Contains(err.Error(), "appears in both") {
		t.Errorf("unexpected error message: %v", err)
	}
}

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

func TestAddonOptionsUnmarshalJSONWithTags(t *testing.T) {
	// Test that tags are correctly parsed from the JSON config
	jsonConfig := `{
		"dhcp_pools": [
			{
				"interface": "eth0",
				"start": "192.168.1.50",
				"end": "192.168.1.100",
				"gateway": "192.168.1.1",
				"netmask": "255.255.255.0"
			}
		],
		"dhcp_ip_address_reservations": [
			{
				"ip": "192.168.1.10",
				"mac": "aa:bb:cc:dd:ee:ff",
				"name": "test-host",
				"tags": ["server", "production"]
			}
		],
		"dhcp_clients_friendly_names": [
			{
				"mac": "11:22:33:44:55:66",
				"name": "friendly-host",
				"tags": ["iot", "living-room"]
			}
		],
		"dhcp_server": {
			"default_lease": "1h",
			"address_reservation_lease": "1h",
			"forget_past_clients_after": "30d",
			"log_requests": false
		},
		"dns_server": {
			"enable": false,
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
		t.Fatalf("Failed to unmarshal JSON with tags: %v", err)
	}

	// Verify tags on IP address reservation
	reservationIP := netip.MustParseAddr("192.168.1.10")
	reservation, ok := opts.ipAddressReservationsByIP[reservationIP]
	if !ok {
		t.Fatalf("Expected IP reservation for 192.168.1.10 not found")
	}
	expectedReservationTags := []string{"server", "production"}
	if len(reservation.Tags) != len(expectedReservationTags) {
		t.Fatalf("Expected %d reservation tags, got %d", len(expectedReservationTags), len(reservation.Tags))
	}
	for i, tag := range expectedReservationTags {
		if reservation.Tags[i] != tag {
			t.Errorf("Expected reservation tag[%d]=%q, got %q", i, tag, reservation.Tags[i])
		}
	}

	// Verify tags on friendly name
	friendlyName, ok := opts.friendlyNames["11:22:33:44:55:66"]
	if !ok {
		t.Fatalf("Expected friendly name for 11:22:33:44:55:66 not found")
	}
	expectedFriendlyTags := []string{"iot", "living-room"}
	if len(friendlyName.Tags) != len(expectedFriendlyTags) {
		t.Fatalf("Expected %d friendly name tags, got %d", len(expectedFriendlyTags), len(friendlyName.Tags))
	}
	for i, tag := range expectedFriendlyTags {
		if friendlyName.Tags[i] != tag {
			t.Errorf("Expected friendly name tag[%d]=%q, got %q", i, tag, friendlyName.Tags[i])
		}
	}
}

func TestAddonOptionsUnmarshalJSONWithDescription(t *testing.T) {
	// Test that description is correctly parsed from the JSON config
	jsonConfig := `{
		"dhcp_pools": [
			{
				"interface": "eth0",
				"start": "192.168.1.50",
				"end": "192.168.1.100",
				"gateway": "192.168.1.1",
				"netmask": "255.255.255.0"
			}
		],
		"dhcp_ip_address_reservations": [
			{
				"ip": "192.168.1.10",
				"mac": "aa:bb:cc:dd:ee:ff",
				"name": "test-host",
				"description": "My important server"
			}
		],
		"dhcp_clients_friendly_names": [
			{
				"mac": "11:22:33:44:55:66",
				"name": "friendly-host",
				"description": "My laptop"
			}
		],
		"dhcp_server": {
			"default_lease": "1h",
			"address_reservation_lease": "1h",
			"forget_past_clients_after": "30d",
			"log_requests": false
		},
		"dns_server": {
			"enable": false,
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
		t.Fatalf("Failed to unmarshal JSON with description: %v", err)
	}

	// Verify description on IP address reservation
	reservationIP := netip.MustParseAddr("192.168.1.10")
	reservation, ok := opts.ipAddressReservationsByIP[reservationIP]
	if !ok {
		t.Fatalf("Expected IP reservation for 192.168.1.10 not found")
	}
	if reservation.Description != "My important server" {
		t.Errorf("Expected reservation description %q, got %q", "My important server", reservation.Description)
	}

	// Verify description on friendly name
	friendlyName, ok := opts.friendlyNames["11:22:33:44:55:66"]
	if !ok {
		t.Fatalf("Expected friendly name for 11:22:33:44:55:66 not found")
	}
	if friendlyName.Description != "My laptop" {
		t.Errorf("Expected friendly name description %q, got %q", "My laptop", friendlyName.Description)
	}
}

func newTestAddonOptions() AddonOptions {
	var opts AddonOptions
	opts.ipAddressReservationsByIP = make(map[netip.Addr]IpAddressReservation)
	opts.ipAddressReservationsByMAC = make(map[string]IpAddressReservation)
	opts.friendlyNames = make(map[string]DhcpClientFriendlyName)
	opts.blockedMACs = make(map[string]BlockedDeviceInfo)
	return opts
}

func baseTestConfig(extraFields string) string {
	return `{
		"dhcp_pools": [
			{
				"interface": "eth0",
				"start": "192.168.1.50",
				"end": "192.168.1.100",
				"gateway": "192.168.1.1",
				"netmask": "255.255.255.0"
			}
		],
		"dhcp_ip_address_reservations": [],
		"dhcp_clients_friendly_names": [],
		"dhcp_server": {
			"default_lease": "1h",
			"address_reservation_lease": "1h",
			"forget_past_clients_after": "30d",
			"log_requests": false
		},
		"dns_server": {
			"enable": false,
			"dns_domain": "lan",
			"port": 53
		},
		"web_ui": {
			"log_activity": false,
			"port": 8976,
			"refresh_interval_sec": 10
		}` + extraFields + `
	}`
}

func TestAddonOptionsMacBlocklistParsed(t *testing.T) {
	jsonConfig := baseTestConfig(`,
		"dhcp_mac_address_blocklist": [
			{ 
				"mac": "11:22:33:44:55:66",
				"description": "Blocked device"
			},
			{ 
				"mac": "AA:BB:CC:DD:EE:FF",
				"description": "Blocked device"
			}
		]`)

	opts := newTestAddonOptions()
	err := json.Unmarshal([]byte(jsonConfig), &opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(opts.blockedMACs) != 2 {
		t.Fatalf("Expected 2 blocked MACs, got %d", len(opts.blockedMACs))
	}
	// MAC addresses are normalized to lowercase by net.ParseMAC
	if _, ok := opts.blockedMACs["11:22:33:44:55:66"]; !ok {
		t.Errorf("Expected 11:22:33:44:55:66 to be blocked")
	}
	if _, ok := opts.blockedMACs["aa:bb:cc:dd:ee:ff"]; !ok {
		t.Errorf("Expected aa:bb:cc:dd:ee:ff to be blocked")
	}
}

func TestAddonOptionsMacBlocklistInvalidMAC(t *testing.T) {
	jsonConfig := baseTestConfig(`,
		"dhcp_mac_address_blocklist": [
			"not-a-mac-address"
		]`)

	opts := newTestAddonOptions()
	err := json.Unmarshal([]byte(jsonConfig), &opts)
	if err == nil {
		t.Error("Expected error for invalid MAC address in blocklist, but got none")
	}
}

func TestAddonOptionsMacBlocklistConflictsWithReservation(t *testing.T) {
	jsonConfig := `{
		"dhcp_pools": [
			{
				"interface": "eth0",
				"start": "192.168.1.50",
				"end": "192.168.1.100",
				"gateway": "192.168.1.1",
				"netmask": "255.255.255.0"
			}
		],
		"dhcp_ip_address_reservations": [
			{
				"ip": "192.168.1.10",
				"mac": "aa:bb:cc:dd:ee:ff",
				"name": "reserved-host"
			}
		],
		"dhcp_clients_friendly_names": [],
		"dhcp_mac_address_blocklist": [
			{ 
				"mac": "aa:bb:cc:dd:ee:ff",
				"description": "Blocked device"
			}
		],
		"dhcp_server": {
			"default_lease": "1h",
			"address_reservation_lease": "1h",
			"forget_past_clients_after": "30d",
			"log_requests": false
		},
		"dns_server": {
			"enable": false,
			"dns_domain": "lan",
			"port": 53
		},
		"web_ui": {
			"log_activity": false,
			"port": 8976,
			"refresh_interval_sec": 10
		}
	}`

	opts := newTestAddonOptions()
	err := json.Unmarshal([]byte(jsonConfig), &opts)
	if err == nil {
		t.Error("Expected error when a MAC appears in both blocklist and reservations, but got none")
	} else if !strings.Contains(err.Error(), "dhcp_ip_address_reservations") || !strings.Contains(err.Error(), "dhcp_mac_address_blocklist") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestAddonOptionsMacBlocklistConflictsWithFriendlyNames(t *testing.T) {
	jsonConfig := `{
		"dhcp_pools": [
			{
				"interface": "eth0",
				"start": "192.168.1.50",
				"end": "192.168.1.100",
				"gateway": "192.168.1.1",
				"netmask": "255.255.255.0"
			}
		],
		"dhcp_ip_address_reservations": [],
		"dhcp_clients_friendly_names": [
			{
				"mac": "11:22:33:44:55:66",
				"name": "friendly-host"
			}
		],
		"dhcp_mac_address_blocklist": [
			{ 
				"mac": "11:22:33:44:55:66",
				"description": "Blocked device"
			}
		],
		"dhcp_server": {
			"default_lease": "1h",
			"address_reservation_lease": "1h",
			"forget_past_clients_after": "30d",
			"log_requests": false
		},
		"dns_server": {
			"enable": false,
			"dns_domain": "lan",
			"port": 53
		},
		"web_ui": {
			"log_activity": false,
			"port": 8976,
			"refresh_interval_sec": 10
		}
	}`

	opts := newTestAddonOptions()
	err := json.Unmarshal([]byte(jsonConfig), &opts)
	if err == nil {
		t.Error("Expected error when a MAC appears in both blocklist and friendly names, but got none")
	} else if !strings.Contains(err.Error(), "dhcp_clients_friendly_names") || !strings.Contains(err.Error(), "dhcp_mac_address_blocklist") {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestIsValidRFC1123DNSName(t *testing.T) {
	testCases := []struct {
		input  string
		wantOK bool
	}{
		{"myhost", true},
		{"my-host", true},
		{"my-host-123", true},
		{"www.google.com", true},
		{"tapo.lan", true},
		{"a.b.c", true},
		// single valid label
		{"abc123", true},
		// empty string
		{"", false},
		// starts with hyphen
		{"-myhost", false},
		// ends with hyphen
		{"myhost-", false},
		// label starts with hyphen
		{"good.-bad", false},
		// contains underscore
		{"my_host", false},
		// contains space
		{"my host", false},
		// trailing dot (empty label)
		{"myhost.", false},
		// leading dot (empty label)
		{".myhost", false},
		// double dot (empty label)
		{"my..host", false},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			got := isValidRFC1123DNSName(tc.input)
			if got != tc.wantOK {
				t.Errorf("isValidRFC1123DNSName(%q) = %v, want %v", tc.input, got, tc.wantOK)
			}
		})
	}
}

func TestAddonOptionsDnsHostsParsed(t *testing.T) {
	jsonConfig := baseTestConfig(`,
		"dns_custom_hosts": [
			{
				"name": "tapo.lan",
				"ipv4_address": "192.168.0.65",
				"ipv6_address": ""
			},
			{
				"name": "myserver",
				"ipv4_address": "10.0.0.1",
				"ipv6_address": "2001:db8::1"
			}
		]`)

	opts := newTestAddonOptions()
	err := json.Unmarshal([]byte(jsonConfig), &opts)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(opts.dnsCustomHosts) != 2 {
		t.Fatalf("Expected 2 dns_custom_hosts, got %d", len(opts.dnsCustomHosts))
	}
	if opts.dnsCustomHosts[0].Name != "tapo.lan" {
		t.Errorf("Expected dnsCustomHosts[0].Name = \"tapo.lan\", got %q", opts.dnsCustomHosts[0].Name)
	}
	if opts.dnsCustomHosts[0].IPv4Address != "192.168.0.65" {
		t.Errorf("Expected dnsCustomHosts[0].IPv4Address = \"192.168.0.65\", got %q", opts.dnsCustomHosts[0].IPv4Address)
	}
	if opts.dnsCustomHosts[1].Name != "myserver" {
		t.Errorf("Expected dnsCustomHosts[1].Name = \"myserver\", got %q", opts.dnsCustomHosts[1].Name)
	}
	if opts.dnsCustomHosts[1].IPv6Address != "2001:db8::1" {
		t.Errorf("Expected dnsCustomHosts[1].IPv6Address = \"2001:db8::1\", got %q", opts.dnsCustomHosts[1].IPv6Address)
	}
}

func TestAddonOptionsDnsHostsInvalidName(t *testing.T) {
	invalidNames := []string{
		"-badstart",
		"bad end",
		"has..double.dot",
		"",
	}
	for _, name := range invalidNames {
		t.Run("invalid:"+name, func(t *testing.T) {
			jsonConfig := baseTestConfig(`,
			"dns_custom_hosts": [
				{
					"name": "` + name + `",
					"ipv4_address": "192.168.0.1",
					"ipv6_address": ""
				}
			]`)
			opts := newTestAddonOptions()
			err := json.Unmarshal([]byte(jsonConfig), &opts)
			if err == nil {
				t.Errorf("Expected error for dns_hosts name %q, but got none", name)
			}
		})
	}
}

func TestAddonOptionsDnsHostsMissingIPAddress(t *testing.T) {
	jsonConfig := baseTestConfig(`,
		"dns_custom_hosts": [
			{
				"name": "myhost",
				"ipv4_address": "",
				"ipv6_address": ""
			}
		]`)
	opts := newTestAddonOptions()
	err := json.Unmarshal([]byte(jsonConfig), &opts)
	if err == nil {
		t.Error("Expected error when both ipv4_address and ipv6_address are empty, but got none")
	}
}

func TestAddonOptionsDnsHostsInvalidIPv4(t *testing.T) {
	jsonConfig := baseTestConfig(`,
		"dns_custom_hosts": [
			{
				"name": "myhost",
				"ipv4_address": "not-an-ip",
				"ipv6_address": ""
			}
		]`)
	opts := newTestAddonOptions()
	err := json.Unmarshal([]byte(jsonConfig), &opts)
	if err == nil {
		t.Error("Expected error for invalid IPv4 address in dns_hosts, but got none")
	}
}

func TestAddonOptionsDnsHostsInvalidIPv6(t *testing.T) {
	jsonConfig := baseTestConfig(`,
		"dns_custom_hosts": [
			{
				"name": "myhost",
				"ipv4_address": "",
				"ipv6_address": "not-an-ipv6"
			}
		]`)
	opts := newTestAddonOptions()
	err := json.Unmarshal([]byte(jsonConfig), &opts)
	if err == nil {
		t.Error("Expected error for invalid IPv6 address in dns_hosts, but got none")
	}
}

func TestAddonOptionsDnsHostsIPv4AsIPv6Rejected(t *testing.T) {
	// Providing an IPv4 address in the ipv6_address field should be rejected
	jsonConfig := baseTestConfig(`,
		"dns_custom_hosts": [
			{
				"name": "myhost",
				"ipv4_address": "",
				"ipv6_address": "192.168.0.1"
			}
		]`)
	opts := newTestAddonOptions()
	err := json.Unmarshal([]byte(jsonConfig), &opts)
	if err == nil {
		t.Error("Expected error when IPv4 address is given in ipv6_address field, but got none")
	}
}

func TestGetEgressInterface(t *testing.T) {
	// getEgressInterface() should return a non-empty, valid interface name.
	// This test requires a working network with a route to 1.1.1.1.
	iface, err := getEgressInterface()
	if err != nil {
		t.Skipf("Skipping test: could not detect egress interface: %v", err)
	}
	if iface == "" {
		t.Error("Expected non-empty interface name from getEgressInterface()")
	}
	// Verify the returned interface actually exists on the host
	if _, err := net.InterfaceByName(iface); err != nil {
		t.Errorf("getEgressInterface() returned %q but interface does not exist: %v", iface, err)
	}
}

func TestAddonOptionsAutoInterface(t *testing.T) {
	// When dhcp_pools[].interface is "auto", UnmarshalJSON should resolve it to
	// the actual egress interface rather than leaving it as "auto".
	// Skip if no route to the internet is available.
	autoIface, err := getEgressInterface()
	if err != nil {
		t.Skipf("Skipping test: could not detect egress interface: %v", err)
	}

	jsonConfig := `{
		"dhcp_pools": [
			{
				"interface": "auto",
				"start": "192.168.1.50",
				"end": "192.168.1.100",
				"gateway": "192.168.1.1",
				"netmask": "255.255.255.0"
			}
		],
		"dhcp_ip_address_reservations": [],
		"dhcp_clients_friendly_names": [],
		"dhcp_server": {
			"default_lease": "1h",
			"address_reservation_lease": "1h",
			"forget_past_clients_after": "30d",
			"log_requests": false
		},
		"dns_server": {
			"enable": false,
			"dns_domain": "lan",
			"port": 53
		},
		"web_ui": {
			"log_activity": false,
			"port": 8976,
			"refresh_interval_sec": 10
		}
	}`

	opts := newTestAddonOptions()
	err = json.Unmarshal([]byte(jsonConfig), &opts)
	if err != nil {
		t.Fatalf("Unexpected error when parsing config with interface=auto: %v", err)
	}

	if len(opts.dhcpRanges) != 1 {
		t.Fatalf("Expected 1 DHCP range, got %d", len(opts.dhcpRanges))
	}
	if opts.dhcpRanges[0].Interface != autoIface {
		t.Errorf("Expected interface %q, got %q", autoIface, opts.dhcpRanges[0].Interface)
	}
}
