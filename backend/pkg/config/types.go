package config

import (
	"net"
	"net/netip"
	texttemplate "text/template"
)

// DhcpClientSettings represents DHCP client metadata loaded from the addon configuration file.
// If IP is valid, this entry is also a static reservation.
type DhcpClientSettings struct {
	Name        string
	MacAddress  net.HardwareAddr
	IP          netip.Addr
	Description string
	Link        *texttemplate.Template // maybe nil
	Tags        []string
	DnsAliases  []string // optional list of DNS CNAME aliases for this reservation
}

// BlockedDeviceInfo represents a blocked MAC address loaded from the addon configuration file
type BlockedDeviceInfo struct {
	Mac         net.HardwareAddr
	Description string
}

// DnsCustomHost represents a custom DNS host record loaded from the addon configuration file
type DnsCustomHost struct {
	Name        string `json:"name"`
	IPv4Address string `json:"ipv4_address"`
	IPv6Address string `json:"ipv6_address"`
}
