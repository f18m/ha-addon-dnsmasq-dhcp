package uibackend

import (
	"dnsmasq-dhcp-backend/pkg/dnsmasqwrapper"
	"dnsmasq-dhcp-backend/pkg/trackerdb"
	"encoding/json"
	htmltemplate "html/template"
	"net"
	"net/netip"
	texttemplate "text/template"

	"github.com/b0ch3nski/go-dnsmasq-utils/dnsmasq"
)

// DhcpClientFriendlyName is the 1:1 binding between a MAC address and a human-friendly name
type DhcpClientFriendlyName struct {
	MacAddress   net.HardwareAddr
	FriendlyName string
	Link         *texttemplate.Template // maybe nil
	Tags         []string
}

// IpAddressReservation represents a static IP configuration loaded from the addon configuration file
type IpAddressReservation struct {
	Name string
	Mac  net.HardwareAddr
	IP   netip.Addr
	Link *texttemplate.Template // maybe nil
	Tags []string
}

// DhcpClientData holds all the information the backend has about a particular DHCP client,
// currently "connected" to the dnsmasq server.
// In this context "connected" means: that sent DHCP traffic since the dnsmasq server was started.
type DhcpClientData struct {
	// the lease as it is parsed from dnsmasq LEASE file:
	Lease dnsmasq.Lease

	// metadata associated with the DHCP client (obtained from configuration):

	// HasStaticIP indicates whether the DHCP server is configured to provide a specific IP address
	// (i.e. has an IP address reservation) for this client.
	// Note that static IP addresses do not need to be inside the DHCP range; indeed quite often the
	// static IP address reserved lies outside the DHCP range
	HasStaticIP bool

	// IsInsideDHCPPool indicates whether this DHCP client has an IP that lies within the DHCP pool
	// range and thus is consuming an IP address from that pool
	// (note that this DHCP client might be a client with a static reservation or not)
	IsInsideDHCPPool bool

	// Sometimes the hostname provided by the DHCP client to the DHCP server is really awkward and
	// non-informative, so we allow users to override that from configuration.
	// If such an override is available in config, this field gets populated.
	FriendlyName string

	// In the configuration file it's possible to specify a golang template that is rendered to
	// produce a string which is intended to be an URL/URI to show for each DHCP client in the web UI.
	// If such link template is available in config, this field gets populated.
	EvaluatedLink string

	// Tags is a list of user-defined labels associated with the DHCP client via configuration.
	Tags []string
}

// MarshalJSON customizes the JSON serialization for DhcpClientData
func (d DhcpClientData) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Lease struct {
			Expires  int64  `json:"expires"`
			MacAddr  string `json:"mac_addr"`
			IPAddr   string `json:"ip_addr"`
			Hostname string `json:"hostname"`
		} `json:"lease"`
		HasStaticIP      bool     `json:"has_static_ip"`
		IsInsideDHCPPool bool     `json:"is_inside_dhcp_pool"`
		FriendlyName     string   `json:"friendly_name"`
		EvaluatedLink    string   `json:"evaluated_link"`
		Tags             []string `json:"tags"`
	}{
		Lease: struct {
			Expires  int64  `json:"expires"`
			MacAddr  string `json:"mac_addr"`
			IPAddr   string `json:"ip_addr"`
			Hostname string `json:"hostname"`
		}{
			Expires:  d.Lease.Expires.Unix(), // unix time, the number of seconds elapsed since January 1, 1970 UTC
			MacAddr:  d.Lease.MacAddr.String(),
			IPAddr:   d.Lease.IPAddr.String(),
			Hostname: d.Lease.Hostname,
		},
		HasStaticIP:      d.HasStaticIP,
		IsInsideDHCPPool: d.IsInsideDHCPPool,
		FriendlyName:     d.FriendlyName,
		EvaluatedLink:    d.EvaluatedLink,
		Tags:             d.Tags,
	})
}

// PastDhcpClientData identifies a DHCP client that was connected in the past, but not anymore
type PastDhcpClientData struct {
	PastInfo     trackerdb.DhcpClient `json:"past_info"`
	HasStaticIP  bool                 `json:"has_static_ip"`
	FriendlyName string               `json:"friendly_name"`
	Notes        string               `json:"notes"`
	Tags         []string             `json:"tags"`
}

// WebSocketMessage defines which contents get transmitted over the websocket in the
// BACKEND -> UI direction.
// Any structure contained here should have a sensible JSON marshalling helper.
// This structure should contain only dynamic data, that will be updated live on the webpage;
// everything else that is "static" will be rendered as a template variable when the page is
// served to the client.
type WebSocketMessage struct {
	// CurrentClients contains the list of clients currently "connected" to the dnsmasq server.
	// In this context "connected" means: that sent DHCP traffic since the dnsmasq server was started.
	CurrentClients []DhcpClientData `json:"current_clients"`

	// PastClients contains the list of clients that were connected in the past, but never
	// obtained a DHCP lease since the last dnsmasq server restart.
	PastClients []PastDhcpClientData `json:"past_clients"`

	// DnsStats provides a live feed about DNS server basic metrics.
	DnsStats dnsmasqwrapper.DnsServerStats `json:"dns_stats"`

	// LogCounters provides counters for notable dnsmasq log warning messages.
	LogCounters dnsmasqwrapper.DnsmasqLogCounters `json:"log_counters"`
}

// HtmlTemplateIpRange is used inside HtmlTemplate
type HtmlTemplateIpRange struct {
	Start string
	End   string

	Interface string
	Gateway   string
	Netmask   string
}

// HtmlTemplate is the struct used to render the "index.templ.html" file
type HtmlTemplate struct {
	// websockets
	WebSocketURI string

	// DHCP config info that are handy to have in the UI page
	DhcpRanges                 []HtmlTemplateIpRange
	DhcpPoolSize               int64
	DefaultLease               string
	AddressReservationLease    string
	DHCPServerStartTime        int64
	DHCPForgetPastClientsAfter string

	// DNS config info
	DnsEnabled string
	DnsDomain  string

	// dnsmasq log counters (initial snapshot when the page is rendered)
	LogCounters dnsmasqwrapper.DnsmasqLogCounters

	// embedded contents
	CssFileContent        htmltemplate.CSS
	JavascriptFileContent htmltemplate.JS

	// misc
	AddonVersion string
}
