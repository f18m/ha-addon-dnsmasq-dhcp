package uibackend

import (
	"dnsmasq-dhcp-backend/pkg/ippool"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strings"
	texttemplate "text/template"
	"time"
)

// AddonOptions contains the configuration provided by the user to the Home Assistant addon
// in the HomeAssistant YAML editor
type AddonOptions struct {
	// Static IP addresses, as read from the configuration
	ipAddressReservationsByIP  map[netip.Addr]IpAddressReservation
	ipAddressReservationsByMAC map[string]IpAddressReservation

	// DHCP client friendly names, as read from the configuration
	// The key of this map is the MAC address formatted as string (since net.HardwareAddr is not a valid map key type)
	friendlyNames map[string]DhcpClientFriendlyName

	// DHCP MAC address blocklist, as read from the configuration
	// The key of this map is the MAC address formatted as string
	blockedMACs map[string]BlockedDeviceInfo

	// Multiple IP ranges all together form the DHCP pool
	dhcpPool   ippool.Pool     // this type provide the Size() and Contains() methods
	dhcpRanges []IpNetworkInfo // this type stores additional metadata for each network

	forgetPastClientsAfter time.Duration

	// Log this backend activities?
	logDHCP  bool
	logWebUI bool

	// web UI
	webUIPort            int
	webUIRefreshInterval time.Duration

	// Lease times
	defaultLease            string
	addressReservationLease string

	// DNS
	dnsEnable      bool
	dnsDomain      string
	dnsPort        int
	dnsCustomHosts []DnsCustomHost
}

// isValidRFC1123Hostname checks whether the given string is a valid hostname
// label as per RFC 1123: 1–63 characters, consisting only of letters, digits,
// and hyphens, and must not start or end with a hyphen.
func isValidRFC1123Hostname(name string) bool {
	if len(name) == 0 || len(name) > 63 {
		return false
	}
	validHostname := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?$`)
	return validHostname.MatchString(name)
}

// isValidRFC1123DNSName checks whether the given string is a valid DNS name
// as per RFC 1123: one or more labels separated by dots, where each label
// satisfies isValidRFC1123Hostname.
func isValidRFC1123DNSName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if !isValidRFC1123Hostname(label) {
			return false
		}
	}
	return true
}

// ParseDuration parses a duration string.
// examples: "10d", "-1.5w" or "3Y4M5d".
// Add time units are "d"="D", "w"="W", "M", "y"="Y".
// Taken from https://gist.github.com/xhit/79c9e137e1cfe332076cdda9f5e24699
func parseDuration(s string) (time.Duration, error) {
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg = true
		s = s[1:]
	}

	re := regexp.MustCompile(`(\d*\.\d+|\d+)[^\d]*`)
	unitMap := map[string]int{
		"d": 24,
		"D": 24,
		"w": 7 * 24,
		"W": 7 * 24,
		"M": 30 * 24,
		"y": 365 * 24,
		"Y": 365 * 24,
	}

	strs := re.FindAllString(s, -1)
	if len(strs) == 0 {
		return 0, fmt.Errorf("invalid duration string: %s", s)
	}

	var sumDur time.Duration
	for _, str := range strs {
		_hours := 1
		for unit, hours := range unitMap {
			if strings.Contains(str, unit) {
				str = strings.ReplaceAll(str, unit, "h")
				_hours = hours
				break
			}
		}

		dur, err := time.ParseDuration(str)
		if err != nil {
			return 0, err
		}

		sumDur += time.Duration(int(dur) * _hours)
	}

	if neg {
		sumDur = -sumDur
	}
	return sumDur, nil
}

// UnmarshalJSON reads the configuration of this Home Assistant addon and converts it
// into maps and slices that get stored into the UIBackend instance
func (o *AddonOptions) UnmarshalJSON(data []byte) error {
	// JSON structure.
	// This must be updated every time the config.yaml of the addon is changed;
	// however this structure contains only fields that are relevant to the
	// UI backend behavior. In other words the addon config.yaml might contain
	// more settings than those listed here.
	var cfg struct {
		DhcpIpAddressReservations []struct {
			Name        string   `json:"name"`
			Mac         string   `json:"mac"`
			IP          string   `json:"ip"`
			Description string   `json:"description"`
			Link        string   `json:"link"`
			Tags        []string `json:"tags"`
		} `json:"dhcp_ip_address_reservations"`

		DhcpClientsFriendlyNames []struct {
			Name        string   `json:"name"`
			Mac         string   `json:"mac"`
			Description string   `json:"description"`
			Link        string   `json:"link"`
			Tags        []string `json:"tags"`
		} `json:"dhcp_clients_friendly_names"`

		DhcpMacAddressBlocklist []struct {
			Mac         string `json:"mac"`
			Description string `json:"description"`
		} `json:"dhcp_mac_address_blocklist"`

		DhcpServer struct {
			LogDHCP                 bool   `json:"log_requests"`
			DefaultLease            string `json:"default_lease"`
			AddressReservationLease string `json:"address_reservation_lease"`
			ForgetPastClientsAfter  string `json:"forget_past_clients_after"`
		} `json:"dhcp_server"`

		DhcpPool []struct {
			Interface string `json:"interface"`
			Start     string `json:"start"`
			End       string `json:"end"`
			Gateway   string `json:"gateway"`
			Netmask   string `json:"netmask"`
		} `json:"dhcp_pools"`

		DnsServer struct {
			Enable    bool   `json:"enable"`
			DnsDomain string `json:"dns_domain"`
			Port      int    `json:"port"`
		} `json:"dns_server"`

		DnsCustomHosts []struct {
			Name        string `json:"name"`
			IPv4Address string `json:"ipv4_address"`
			IPv6Address string `json:"ipv6_address"`
		} `json:"dns_custom_hosts"`

		WebUI struct {
			Log                bool `json:"log_activity"`
			Port               int  `json:"port"`
			RefreshIntervalSec int  `json:"refresh_interval_sec"`
		} `json:"web_ui"`
	}
	err := json.Unmarshal(data, &cfg)
	if err != nil {
		return err
	}

	// convert DHCP IP addresses (strings) to iprange.Pool == []iprange.Range
	for _, r := range cfg.DhcpPool {
		dhcpR := ippool.NewRangeFromString(r.Start, r.End)
		if !dhcpR.IsValid() {
			return fmt.Errorf("invalid DHCP range %s-%s found in addon config file", r.Start, r.End)
		}

		// create also the IpNetworkInfo obj associated:
		ipNetInfo := IpNetworkInfo{
			Interface: r.Interface,
			Start:     dhcpR.Start,
			End:       dhcpR.End,
			Gateway:   net.ParseIP(strings.TrimSpace(r.Gateway)),
		}

		m := net.ParseIP(strings.TrimSpace(r.Netmask))
		if m.To4() != nil {
			ipNetInfo.Netmask = net.IPMask(m.To4())
		} else {
			ipNetInfo.Netmask = net.IPMask(m.To16())
		}

		// check network definition
		if !ipNetInfo.HasValidIPs() {
			// RFC 1918 (IPv4 addresses) and RFC 4193 (IPv6 addresses).
			return fmt.Errorf("invalid DHCP network/range [%s] found in addon config file: the IP addresses should define a coherent network: a) they should be private IPs only according to RFC 1918 and RFC 4193; b) their start and end IPs must be within the same network", ipNetInfo.String())
		}
		if !ipNetInfo.HasValidGateway() {
			return fmt.Errorf("invalid DHCP network/range [%s] found in addon config file: the gateway must be an IP address within the network defined by the startIP/endIP/netmask parameters", ipNetInfo.String())
		}

		// all good: store the info
		o.dhcpPool.Ranges = append(o.dhcpPool.Ranges, dhcpR)
		o.dhcpRanges = append(o.dhcpRanges, ipNetInfo)
	}

	// ensure we have a valid port for web UI
	if cfg.WebUI.Port <= 0 || cfg.WebUI.Port > 32768 {
		return fmt.Errorf("invalid web UI port number: %d", cfg.WebUI.Port)
	}

	// convert IP address reservations to a map indexed by IP
	for _, r := range cfg.DhcpIpAddressReservations {
		// validate (host)name
		if !isValidRFC1123Hostname(r.Name) {
			return fmt.Errorf("invalid hostname found inside 'dhcp_ip_address_reservations': %q (must be 1–63 chars, letters/digits/hyphens only, not starting or ending with a hyphen)", r.Name)
		}

		// validate and normalize IP and MAC address
		ipAddr, err := netip.ParseAddr(strings.TrimSpace(r.IP))
		if err != nil {
			return fmt.Errorf("invalid IP address found inside 'ip_address_reservations': %s", r.IP)
		}
		macAddr, err := net.ParseMAC(strings.TrimSpace(r.Mac))
		if err != nil {
			return fmt.Errorf("invalid MAC address found inside 'ip_address_reservations': %s", r.Mac)
		}

		// validate the golang template provided in the "link" field, if any
		var linkTemplate *texttemplate.Template
		if r.Link != "" {
			linkTemplate, err = texttemplate.New("linkTemplate").Parse(r.Link)
			if err != nil {
				return fmt.Errorf("invalid golang template found inside 'link': %s", r.Link)
			}
		}

		// normalize the IP and MAC address format (e.g. to lowercase)
		r.IP = ipAddr.String()
		r.Mac = macAddr.String()

		ipReservation := IpAddressReservation{
			Name:        r.Name,
			Mac:         macAddr,
			IP:          ipAddr,
			Description: r.Description,
			Link:        linkTemplate,
			Tags:        r.Tags,
		}

		// check for duplicates in IP/MAC address reservations
		if _, exists := o.ipAddressReservationsByIP[ipAddr]; exists {
			return fmt.Errorf("duplicate IP address found inside 'dhcp_ip_address_reservations': the IP %s is assigned to both %s and %s", ipAddr, o.ipAddressReservationsByIP[ipAddr].Name, r.Name)
		}
		if _, exists := o.ipAddressReservationsByMAC[macAddr.String()]; exists {
			return fmt.Errorf("duplicate MAC address found inside 'dhcp_ip_address_reservations': the MAC %s is assigned to both %s and %s", macAddr, o.ipAddressReservationsByMAC[macAddr.String()].Name, r.Name)
		}

		o.ipAddressReservationsByIP[ipAddr] = ipReservation
		o.ipAddressReservationsByMAC[macAddr.String()] = ipReservation
	}

	// convert friendly names to a map of DhcpClientFriendlyName instances indexed by MAC address
	for _, client := range cfg.DhcpClientsFriendlyNames {
		macAddr, err := net.ParseMAC(strings.TrimSpace(client.Mac))
		if err != nil {
			return fmt.Errorf("invalid MAC address found inside 'dhcp_clients_friendly_names': %s", client.Mac)
		}

		var linkTemplate *texttemplate.Template
		if client.Link != "" {
			linkTemplate, err = texttemplate.New("linkTemplate").Parse(client.Link)
			if err != nil {
				return fmt.Errorf("invalid golang template found inside 'link': %s", client.Link)
			}
		}

		// check that this MAC address is not already used in IP address reservations
		if _, exists := o.ipAddressReservationsByMAC[macAddr.String()]; exists {
			return fmt.Errorf("MAC address %s appears in both 'dhcp_ip_address_reservations' and 'dhcp_clients_friendly_names'; a MAC address can only be in one of the two lists", macAddr)
		}

		o.friendlyNames[macAddr.String()] = DhcpClientFriendlyName{
			MacAddress:   macAddr,
			FriendlyName: client.Name,
			Description:  client.Description,
			Link:         linkTemplate,
			Tags:         client.Tags,
		}
	}

	// parse MAC address blocklist
	for _, devInfo := range cfg.DhcpMacAddressBlocklist {
		macAddr, err := net.ParseMAC(strings.TrimSpace(devInfo.Mac))
		if err != nil {
			return fmt.Errorf("invalid MAC address found inside 'dhcp_mac_address_blocklist': %s", devInfo.Mac)
		}

		// check that this MAC address is not already used in IP address reservations
		if _, exists := o.ipAddressReservationsByMAC[macAddr.String()]; exists {
			return fmt.Errorf("MAC address %s appears in both 'dhcp_ip_address_reservations' and 'dhcp_mac_address_blocklist'; a MAC address can only be in one of the two lists", macAddr)
		}

		// check that this MAC address is not already used in friendly names
		if _, exists := o.friendlyNames[macAddr.String()]; exists {
			return fmt.Errorf("MAC address %s appears in both 'dhcp_clients_friendly_names' and 'dhcp_mac_address_blocklist'; a MAC address can only be in one of the two lists", macAddr)
		}

		o.blockedMACs[macAddr.String()] = BlockedDeviceInfo{
			Mac:         macAddr,
			Description: devInfo.Description,
		}
	}

	// parse custom DNS host records
	for _, h := range cfg.DnsCustomHosts {
		if !isValidRFC1123DNSName(h.Name) {
			return fmt.Errorf("invalid DNS name found inside 'dns_custom_hosts': %q (must consist of RFC 1123 labels separated by dots)", h.Name)
		}
		ipv4 := strings.TrimSpace(h.IPv4Address)
		ipv6 := strings.TrimSpace(h.IPv6Address)
		if ipv4 == "" && ipv6 == "" {
			return fmt.Errorf("dns_custom_hosts entry %q must have at least one of 'ipv4_address' or 'ipv6_address'", h.Name)
		}
		if ipv4 != "" {
			if ip := net.ParseIP(ipv4); ip == nil || ip.To4() == nil {
				return fmt.Errorf("invalid IPv4 address found inside 'dns_custom_hosts' for %q: %s", h.Name, ipv4)
			}
		}
		if ipv6 != "" {
			if ip := net.ParseIP(ipv6); ip == nil || ip.To4() != nil {
				return fmt.Errorf("invalid IPv6 address found inside 'dns_custom_hosts' for %q: %s", h.Name, ipv6)
			}
		}
		o.dnsCustomHosts = append(o.dnsCustomHosts, DnsCustomHost{
			Name:        h.Name,
			IPv4Address: ipv4,
			IPv6Address: ipv6,
		})
	}

	// parse time duration
	o.forgetPastClientsAfter, err = parseDuration(cfg.DhcpServer.ForgetPastClientsAfter)
	if err != nil {
		return fmt.Errorf("invalid time duration found inside 'forget_past_clients_after': %s", cfg.DhcpServer.ForgetPastClientsAfter)
	}

	o.webUIRefreshInterval = time.Duration(cfg.WebUI.RefreshIntervalSec) * time.Second

	// copy basic settings
	o.logDHCP = cfg.DhcpServer.LogDHCP
	o.logWebUI = cfg.WebUI.Log
	o.webUIPort = cfg.WebUI.Port
	o.defaultLease = cfg.DhcpServer.DefaultLease
	o.addressReservationLease = cfg.DhcpServer.AddressReservationLease
	o.dnsEnable = cfg.DnsServer.Enable
	o.dnsDomain = cfg.DnsServer.DnsDomain
	o.dnsPort = cfg.DnsServer.Port

	return nil
}
