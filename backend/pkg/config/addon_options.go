package config

import (
	"dnsmasq-dhcp-backend/pkg/ippool"
	"dnsmasq-dhcp-backend/pkg/logger"
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"regexp"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/davidbanham/human_duration/v3"
)

// AddonOptions contains the configuration provided by the user to the Home Assistant addon
// in the HomeAssistant YAML editor for App configurations.
// This configuration is loaded at runtime by the UI backend.
type AddonOptions struct {
	// Static IP addresses, as read from the configuration
	IpAddressReservationsByIP  map[netip.Addr]IpAddressReservation
	IpAddressReservationsByMAC map[string]IpAddressReservation

	// DHCP client friendly names, as read from the configuration
	// The key of this map is the MAC address formatted as string (since net.HardwareAddr is not a valid map key type)
	FriendlyNames map[string]DhcpClientFriendlyName

	// DHCP MAC address blocklist, as read from the configuration
	// The key of this map is the MAC address formatted as string
	BlockedMACs map[string]BlockedDeviceInfo

	// Multiple IP ranges all together form the DHCP pool
	DhcpPool   ippool.Pool     // this type provide the Size() and Contains() methods
	DhcpRanges []IpNetworkInfo // this type stores additional metadata for each network

	ForgetPastClientsAfter time.Duration

	// Log this backend activities?
	LogDHCP  bool
	LogWebUI bool

	// web UI
	WebUIPort            int
	WebUIRefreshInterval time.Duration

	// Lease times
	DefaultLease            string
	AddressReservationLease string

	// DNS
	DnsEnable      bool
	DnsDomain      string
	DnsPort        int
	DnsCustomHosts []DnsCustomHost
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

func (o *AddonOptions) loadDnsAliases(dhcpClientName string, dnsAliases []string, dnsDomain string, logger logger.CustomLogger) ([]string, error) {
	// validate DNS aliases, if any, trimming whitespace once during validation
	var normalizedAliases []string
	if len(dnsAliases) > 0 {
		normalizedAliases = make([]string, 0, len(dnsAliases))
		for _, alias := range dnsAliases {
			alias = strings.TrimSpace(alias)
			if !isValidRFC1123DNSName(alias) {
				return nil, fmt.Errorf("invalid DNS alias found inside 'dhcp_client_settings' for host %q: %q (must consist of RFC 1123 labels separated by dots)",
					dhcpClientName, alias)
			}

			// Why do we require that the DNS aliases end with the configured DNS domain?
			// Because dnsmasq will automatically append the DNS domain to any alias that doesn't already
			// end with it, and this can lead to confusion if the user accidentally adds an alias without
			// the DNS domain suffix.
			// By enforcing this rule in the UI backend, we remove this confusion.
			// Also note that providing as dns_alias an FQDN with a different domain than the one configured
			// in dns_domain won't work for 2 reasons:
			// 1) dnsmasq will still append the configured dns_domain to the alias, resulting in a final FQDN
			//    that is different from the one provided by the user
			//    (e.g. "printer.testdomain" -> "printer.testdomain.lan" if dns_domain is "lan")
			//    So you cannot really create an entry for "printer.testdomain"!
			// 2) even if dnsmasq didn't append the dns_domain, the DNS resolution of that alias would
			//    still fail because dnsmasq only resolves names within the configured dns_domain.
			//    Moreover devices in your network would probably fail to route the request to the dnsmasq DNS server
			//    because they know (by DHCP) that dnsmasq is authoritative only for the configured dns_domain
			if !strings.HasSuffix(strings.ToLower(alias), "."+strings.ToLower(dnsDomain)) {
				// do not error out:
				// return fmt.Errorf("invalid DNS alias found inside 'dhcp_client_settings' for host %q: %q (must end with .%s)", dhcpClientName, alias, dnsDomain)

				// do not error out, just add a warning log and automatically append the DNS domain suffix to the alias
				logger.Warnf("DNS alias %q for host %q does not end with the configured DNS domain %q; automatically appending the DNS domain suffix",
					alias, dhcpClientName, dnsDomain)
				alias = fmt.Sprintf("%s.%s", alias, dnsDomain)
			}
			normalizedAliases = append(normalizedAliases, alias)
		}
	}

	return normalizedAliases, nil
}

// LoadFromJSON reads the configuration of this Home Assistant addon and converts it
// into maps and slices that get stored into the UIBackend instance
//
// The logger instance will be used to emit WARNING logs about potentially misconfigured settings
// in the addon configuration file, but that are not severe enough to cause an error
func (o *AddonOptions) LoadFromJSON(data []byte, logger logger.CustomLogger) error {
	// JSON structure.
	// This must be updated every time the config.yaml of the addon is changed;
	// however this structure contains only fields that are relevant to the
	// UI backend behavior. In other words the addon config.yaml might contain
	// more settings than those listed here.
	var cfg struct {
		DhcpClientSettings []struct {
			Name        string   `json:"name"`
			Mac         string   `json:"mac"`
			ReservedIP  string   `json:"reserved_ip"`
			Description string   `json:"description"`
			Link        string   `json:"link"`
			Tags        []string `json:"tags"`
			DnsAliases  []string `json:"dns_aliases"`
		} `json:"dhcp_client_settings"`

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

		// please be aware that the interface name might be something like "auto";
		// in that case the real interface name is resolved only for dnsmasq by the dnsmasq-init s6 service;
		// if we ever need the actual interface name in this Backend, then we might want to save the
		// dnsmasq-init processing output to a file and read it here. Or just load the dnsmasq config file here.
		ifaceName := strings.TrimSpace(r.Interface)

		// create also the IpNetworkInfo obj associated:
		ipNetInfo := IpNetworkInfo{
			Interface: ifaceName,
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
		o.DhcpPool.Ranges = append(o.DhcpPool.Ranges, dhcpR)
		o.DhcpRanges = append(o.DhcpRanges, ipNetInfo)
	}

	// ensure we have a valid port for web UI
	if cfg.WebUI.Port <= 0 || cfg.WebUI.Port > 32768 {
		return fmt.Errorf("invalid web UI port number: %d", cfg.WebUI.Port)
	}

	// "local" is reserved by mDNS and must not be used as dnsmasq DNS domain.
	dnsDomain := strings.TrimSpace(cfg.DnsServer.DnsDomain)
	if strings.EqualFold(dnsDomain, "local") {
		return fmt.Errorf("invalid DNS domain found inside 'dns_server': %q ('local' is reserved for mDNS)", cfg.DnsServer.DnsDomain)
	}

	rfc1123Explanation := "a valid hostname must consist of 1 to 63 characters, and can only contain letters, digits, and hyphens, and must not start or end with a hyphen"

	// convert dhcp_client_settings entries to IP address reservations or friendly names
	// depending on whether the reserved_ip field is set
	for _, entry := range cfg.DhcpClientSettings {
		// validate (host)name
		if !isValidRFC1123Hostname(entry.Name) {
			return fmt.Errorf("invalid hostname found inside 'dhcp_client_settings': %q (%s)", entry.Name, rfc1123Explanation)
		}

		// validate MAC address
		macAddr, err := net.ParseMAC(strings.TrimSpace(entry.Mac))
		if err != nil {
			return fmt.Errorf("invalid MAC address found inside 'dhcp_client_settings': %s", entry.Mac)
		}

		// validate the golang template provided in the "link" field, if any
		var linkTemplate *texttemplate.Template
		if entry.Link != "" {
			linkTemplate, err = texttemplate.New("linkTemplate").Parse(entry.Link)
			if err != nil {
				return fmt.Errorf("invalid golang template found inside 'link': %s", entry.Link)
			}
		}

		// normalize DNS aliases if any
		normalizedAliases, err := o.loadDnsAliases(entry.Name, entry.DnsAliases, dnsDomain, logger)
		if err != nil {
			return err
		}

		reservedIP := strings.TrimSpace(entry.ReservedIP)
		if reservedIP != "" {
			// entry has a reserved IP → treat as an IP address reservation
			ipAddr, err := netip.ParseAddr(reservedIP)
			if err != nil {
				return fmt.Errorf("invalid IP address found inside 'dhcp_client_settings': %s", entry.ReservedIP)
			}

			ipReservation := IpAddressReservation{
				Name:        entry.Name,
				Mac:         macAddr,
				IP:          ipAddr,
				Description: entry.Description,
				Link:        linkTemplate,
				Tags:        entry.Tags,
				DnsAliases:  normalizedAliases,
			}

			// check for duplicates
			if _, exists := o.IpAddressReservationsByIP[ipAddr]; exists {
				return fmt.Errorf("duplicate IP address found inside 'dhcp_client_settings': the IP %s is assigned to both %s and %s", ipAddr, o.IpAddressReservationsByIP[ipAddr].Name, entry.Name)
			}
			if _, exists := o.IpAddressReservationsByMAC[macAddr.String()]; exists {
				return fmt.Errorf("duplicate MAC address found inside 'dhcp_client_settings': the MAC %s is assigned to both %s and %s", macAddr, o.IpAddressReservationsByMAC[macAddr.String()].Name, entry.Name)
			}

			o.IpAddressReservationsByIP[ipAddr] = ipReservation
			o.IpAddressReservationsByMAC[macAddr.String()] = ipReservation
		} else {
			// entry has no reserved IP → treat as a friendly name
			if _, exists := o.IpAddressReservationsByMAC[macAddr.String()]; exists {
				return fmt.Errorf("duplicate MAC address found inside 'dhcp_client_settings': the MAC %s is assigned to both %s and %s", macAddr, o.IpAddressReservationsByMAC[macAddr.String()].Name, entry.Name)
			}
			if _, exists := o.FriendlyNames[macAddr.String()]; exists {
				return fmt.Errorf("duplicate MAC address found inside 'dhcp_client_settings': the MAC %s appears more than once", macAddr)
			}

			o.FriendlyNames[macAddr.String()] = DhcpClientFriendlyName{
				MacAddress:   macAddr,
				FriendlyName: entry.Name,
				Description:  entry.Description,
				Link:         linkTemplate,
				Tags:         entry.Tags,
				DnsAliases:   normalizedAliases,
			}
		}
	}

	// parse MAC address blocklist
	for _, devInfo := range cfg.DhcpMacAddressBlocklist {
		macAddr, err := net.ParseMAC(strings.TrimSpace(devInfo.Mac))
		if err != nil {
			return fmt.Errorf("invalid MAC address found inside 'dhcp_mac_address_blocklist': %s", devInfo.Mac)
		}

		// check that this MAC address is not already used in IP address reservations
		if _, exists := o.IpAddressReservationsByMAC[macAddr.String()]; exists {
			return fmt.Errorf("MAC address %s appears in both 'dhcp_client_settings' and 'dhcp_mac_address_blocklist'; a MAC address can only be in one of the two lists", macAddr)
		}

		// check that this MAC address is not already used in friendly names
		if _, exists := o.FriendlyNames[macAddr.String()]; exists {
			return fmt.Errorf("MAC address %s appears in both 'dhcp_client_settings' and 'dhcp_mac_address_blocklist'; a MAC address can only be in one of the two lists", macAddr)
		}

		o.BlockedMACs[macAddr.String()] = BlockedDeviceInfo{
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
		o.DnsCustomHosts = append(o.DnsCustomHosts, DnsCustomHost{
			Name:        h.Name,
			IPv4Address: ipv4,
			IPv6Address: ipv6,
		})
	}

	// parse time duration
	o.ForgetPastClientsAfter, err = parseDuration(cfg.DhcpServer.ForgetPastClientsAfter)
	if err != nil {
		return fmt.Errorf("invalid time duration found inside 'forget_past_clients_after': %s", cfg.DhcpServer.ForgetPastClientsAfter)
	}

	o.WebUIRefreshInterval = time.Duration(cfg.WebUI.RefreshIntervalSec) * time.Second

	// copy basic settings
	o.LogDHCP = cfg.DhcpServer.LogDHCP
	o.LogWebUI = cfg.WebUI.Log
	o.WebUIPort = cfg.WebUI.Port
	o.DefaultLease = cfg.DhcpServer.DefaultLease
	o.AddressReservationLease = cfg.DhcpServer.AddressReservationLease
	o.DnsEnable = cfg.DnsServer.Enable
	o.DnsDomain = dnsDomain
	o.DnsPort = cfg.DnsServer.Port

	// print summary
	logger.Infof("Acquired %d DHCP network/ranges\n", len(o.DhcpRanges))
	logger.Infof("Acquired %d IP address reservations\n", len(o.IpAddressReservationsByIP))
	logger.Infof("Acquired %d friendly name definitions\n", len(o.FriendlyNames))
	logger.Infof("Acquired %d blocked MAC addresses\n", len(o.BlockedMACs))
	logger.Infof("Acquired %d custom DNS host records\n", len(o.DnsCustomHosts))
	logger.Infof("DHCP requests logging enabled=%t; cleanup threshold for past DHCP clients set to %s\n",
		o.LogDHCP, human_duration.ShortString(o.ForgetPastClientsAfter, human_duration.Minute))
	logger.Infof("Web server on port %d; Web UI logging enabled=%t; Web UI refresh interval=%s\n",
		o.WebUIPort, o.LogWebUI, o.WebUIRefreshInterval.String())

	return nil
}

func (o *AddonOptions) GetIpReservationByMAC(mac net.HardwareAddr) (IpAddressReservation, bool) {
	reservation, exists := o.IpAddressReservationsByMAC[mac.String()]
	return reservation, exists
}

func (o *AddonOptions) GetFriendlyNameByMAC(mac net.HardwareAddr) (DhcpClientFriendlyName, bool) {
	friendlyName, exists := o.FriendlyNames[mac.String()]
	return friendlyName, exists
}

func (o *AddonOptions) GetIpReservationByIP(ip netip.Addr) (IpAddressReservation, bool) {
	reservation, exists := o.IpAddressReservationsByIP[ip]
	return reservation, exists
}
