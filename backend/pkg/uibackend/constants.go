package uibackend

import "time"

// the dnsmasq lease file is configured in the dnsmasq config file: the value
// here has to match the server config file!
var defaultDnsmasqLeasesFile = "/data/dnsmasq.leases"

// the home assistant addon options is fixed and cannot be changed actually:
var defaultHomeAssistantOptionsFile = "/data/options.json"

// the home assistant addon config is fixed and cannot be changed actually:
var defaultHomeAssistantConfigFile = "/opt/bin/addon-config.yaml"

// location for our small DB tracking DHCP clients:
var defaultDhcpClientTrackerDB = "/data/trackerdb.sqlite3"

// location for a basic counter that is used to tag entries in the tracker DB
// to understand if they are stale or not
var defaultStartEpoch = "/data/startepoch"

// interval for checking past DHCP clients that need to be removed from the tracker DB
var pastClientsCheckInterval = 5 * time.Minute

// These absolute paths must be in sync with the Dockerfile
var (
	staticWebFilesDir = "/opt/web/static"
	templatesDir      = "/opt/web/templates"
)

// other constants
var (
	dnsmasqMarkerForMissingHostname = "*"
	websocketRelativeUrl            = "/ws"
	unknownHostnameHtmlString       = "&lt;unknown&gt;"
)
