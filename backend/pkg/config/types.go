package config

import (
	"net"
	"net/netip"
	texttemplate "text/template"
)

// DhcpClientFriendlyName is the 1:1 binding between a MAC address and a human-friendly name
// FIXME: this needs to be removed and replaced with IpAddressReservation
type DhcpClientFriendlyName struct {
	MacAddress   net.HardwareAddr
	FriendlyName string
	Description  string
	Link         *texttemplate.Template // maybe nil
	Tags         []string
	DnsAliases   []string // optional list of DNS CNAME aliases for this client; requires FriendlyName to be a valid RFC 1123 hostname
}

// IpAddressReservation represents a static IP configuration loaded from the addon configuration file
// FIXME: this needs to be renamed to DhcpClientMetadata or DhcpClientSettings
type IpAddressReservation struct {
	Name        string
	Mac         net.HardwareAddr
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
