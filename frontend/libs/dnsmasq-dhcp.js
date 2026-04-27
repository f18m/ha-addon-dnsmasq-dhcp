/*
    Dnsmasq-DHCP Javascript code

    Contains all client-side logic to format the tables,
    handle websocket events, handle tabs, etc.
*/

/* GLOBALS */
var config = { // this global variable is initialized via setConfig()
    "webSocketURI": null,
    "dhcpServerStartTime": null,
    "dhcpPoolSize": null,
    "dnsCustomHosts": null,
    "numRows": null,
}
var global_objects = {
    // Datatables.net instances:
    table_current: null,
    table_past: null,
    table_dns_upstreams: null,
    table_dns_hosts: null,
    // Websocket:
    backend_ws: null,
    // stats:
    num_updates: 0
}

/* HELPER FUNCTIONS */

function assert(condition, message) {
    if (!condition) {
        throw new Error(message || "Assertion failed");
    }
}

// Copy the value stored in data-copy of the clicked button to the clipboard
// and give brief visual feedback by toggling the copy-btn-success CSS class.
// On failure, toggle copy-btn-error to indicate the problem to the user.
function copyToClipboard(btn) {
    var text = btn.getAttribute('data-copy');
    
    // Use modern Clipboard API if available and in secure context
    if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(text).then(function() {
            btn.classList.add('copy-btn-success');
            setTimeout(function() {
                btn.classList.remove('copy-btn-success');
            }, 1500);
        }).catch(function(err) {
            console.error('Could not copy text to clipboard: ', err);
            btn.classList.add('copy-btn-error');
            setTimeout(function() {
                btn.classList.remove('copy-btn-error');
            }, 1500);
        });
    } else {
        // Fallback for older browsers or non-secure contexts
        var textArea = document.createElement("textarea");
        textArea.value = text;
        textArea.style.position = "fixed";
        textArea.style.left = "-999999px";
        textArea.style.top = "-999999px";
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        try {
            document.execCommand('copy');
            btn.classList.add('copy-btn-success');
            setTimeout(function() {
                btn.classList.remove('copy-btn-success');
            }, 1500);
        } catch (err) {
            console.error('Failed to copy (fallback): ', err);
            btn.classList.add('copy-btn-error');
            setTimeout(function() {
                btn.classList.remove('copy-btn-error');
            }, 1500);
        }
        document.body.removeChild(textArea);
    }
}


/* FORMATTING FUNCTIONS */

// SVG clipboard icon used by copy buttons
var COPY_ICON_SVG = '<svg xmlns="http://www.w3.org/2000/svg" width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>';

// SVG information icon used by the info button
var INFO_ICON_SVG = '<svg xmlns="http://www.w3.org/2000/svg" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="16" x2="12" y2="12"></line><line x1="12" y1="8" x2="12.01" y2="8"></line></svg>';

// SVG icons used to indicate if a DHCP client has a reserved IP.
var RESERVED_IP_YES_ICON_SVG = '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><path d="m9 12 2 2 4-4"></path></svg>';
var RESERVED_IP_NO_ICON_SVG = '<svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"></circle><path d="m15 9-6 6"></path><path d="m9 9 6 6"></path></svg>';

// Render a table cell value with a small inline copy-to-clipboard button.
// The "type" parameter is the DataTables rendering context; the button is
// only injected for the 'display' type so that sorting and filtering still
// operate on the plain text value.
function renderWithCopyButton(data, type) {
    if (type !== 'display' || !data || data === 'N/A') {
        return data;
    }
    // Escape special HTML characters to prevent injection via the attribute value and cell content
    var escaped = data.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    return '<span class="mono-address">' + escaped + '</span><button class="copy-btn" onclick="copyToClipboard(this)" data-copy="' + escaped + '" title="Copy to clipboard">' + COPY_ICON_SVG + '</button>';
}

// Open the DHCP client details dialog for the given device.
function showInfoDialog(friendlyname, hostname, dnsNames, macAddr, description, link, tags, hasStaticIP) {
    var dialog = document.getElementById('info_dialog');
    var title = document.getElementById('info_dialog_title');
    var nameElem = document.getElementById('info_dialog_name');
    var nameSourceElem = document.getElementById('info_dialog_name_source');
    var macElem = document.getElementById('info_dialog_mac');
    var macCopyBtn = document.getElementById('info_dialog_mac_copy');
    var reservedElem = document.getElementById('info_dialog_reserved');
    var reservedIconElem = document.getElementById('info_dialog_reserved_icon');
    var reservedTextElem = document.getElementById('info_dialog_reserved_text');
    var descriptionElem = document.getElementById('info_dialog_description');
    var linkElem = document.getElementById('info_dialog_link');
    var noLinkElem = document.getElementById('info_dialog_no_link');
    var tagsElem = document.getElementById('info_dialog_tags');
    var list = document.getElementById('info_dialog_list');
    var noDnsElem = document.getElementById('info_dialog_no_dns');

    title.textContent = 'DHCP Client Information';

    // Populate the name row and explain the source
    var bestName;
    if (friendlyname && friendlyname.length > 0) {
        bestName = friendlyname;
        nameSourceElem.textContent = 'Friendly name — defined in \'dhcp_client_settings\' in App configuration';
    } else if (hostname && hostname.length > 0) {
        bestName = hostname;
        nameSourceElem.textContent = 'No friendly name defined in App configuration for MAC address ' + (macAddr || 'N/A') + '; showing the DHCP hostname provided by the client via DHCP protocol';
    } else {
        bestName = "N/A";
        nameSourceElem.textContent = 'No name available';
    }
    nameElem.textContent = bestName;

    // Populate MAC address section
    var displayedMac = macAddr || 'N/A';
    macElem.textContent = displayedMac;
    if (macAddr) {
        macCopyBtn.style.display = '';
        macCopyBtn.setAttribute('data-copy', macAddr);
        macCopyBtn.innerHTML = COPY_ICON_SVG;
    } else {
        macCopyBtn.style.display = 'none';
        macCopyBtn.removeAttribute('data-copy');
    }

    // Populate reserved IP section
    if (hasStaticIP) {
        reservedElem.classList.add('is-reserved');
        reservedElem.classList.remove('is-dynamic');
        reservedIconElem.innerHTML = RESERVED_IP_YES_ICON_SVG;
        reservedTextElem.textContent = 'This client has an IP reservation';
    } else {
        reservedElem.classList.add('is-dynamic');
        reservedElem.classList.remove('is-reserved');
        reservedIconElem.innerHTML = RESERVED_IP_NO_ICON_SVG;
        reservedTextElem.textContent = 'This client uses dynamic DHCP addresses';
    }

    // Populate description section
    descriptionElem.textContent = (description && description.length > 0) ? description : 'N/A';

    // Populate link section
    if (link && link.length > 0) {
        linkElem.style.display = '';
        noLinkElem.style.display = 'none';
        linkElem.href = link;
        linkElem.textContent = link;
    } else {
        linkElem.style.display = 'none';
        linkElem.removeAttribute('href');
        linkElem.textContent = '';
        noLinkElem.style.display = '';
    }

    // Populate tags section
    tagsElem.innerHTML = formatTags(tags);

    // Populate DNS names list
    list.innerHTML = '';
    if (dnsNames && dnsNames.length > 0) {
        noDnsElem.style.display = 'none';
        list.style.display = '';
        dnsNames.forEach(function(name) {
            var li = document.createElement('li');
            li.className = 'dns-name-entry';

            var nameSpan = document.createElement('span');
            nameSpan.className = 'dns-name-text';
            nameSpan.textContent = name;

            var copyBtn = document.createElement('button');
            copyBtn.className = 'copy-btn';
            copyBtn.title = 'Copy to clipboard';
            copyBtn.setAttribute('data-copy', name);
            copyBtn.setAttribute('onclick', 'copyToClipboard(this)');
            copyBtn.innerHTML = COPY_ICON_SVG;

            li.appendChild(nameSpan);
            li.appendChild(copyBtn);
            list.appendChild(li);
        });
    } else {
        list.style.display = 'none';
        noDnsElem.style.display = '';
    }

    dialog.showModal();
}

// Render the hostname cell, appending a small info button.
// The "type" parameter is the DataTables rendering context; the button is
// only injected for the 'display' type so that sorting and filtering still
// operate on the plain text value.
function renderNameWithInfoBtn(friendlyname, hostname, dnsNames, macAddr, description, evaluatedLink, tags, hasStaticIP, type) {
    if (type !== 'display') {
        if (friendlyname && friendlyname.length > 0) {
            return friendlyname;
        }
        return hostname;
    }
    // guard against XSS: escape special chars
    var displayedName = friendlyname || hostname || 'N/A';
    var escaped = displayedName.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    var namesAttr = JSON.stringify(dnsNames || []).replace(/"/g, '&quot;');
    var tagsAttr = JSON.stringify(tags || []).replace(/"/g, '&quot;');
    var escapedFriendlyname = (friendlyname || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    var escapedHostname = (hostname || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    var escapedMac = (macAddr || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    var escapedDescription = (description || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    var escapedLink = (evaluatedLink || '').replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    var hasStaticIPAttr = hasStaticIP ? 'true' : 'false';
    // Use data attributes only; the click handler is set up via event delegation (see InitInfoDialogHandler).
    // This avoids putting JS-escaped values into inline onclick attributes.
    return escaped +
        '<button class="info-btn" ' +
        'data-friendly-name="' + escapedFriendlyname + '" ' +
        'data-hostname="' + escapedHostname + '" ' +
        'data-mac="' + escapedMac + '" ' +
        'data-dns-names="' + namesAttr + '" ' +
        'data-description="' + escapedDescription + '" ' +
        'data-link="' + escapedLink + '" ' +
        'data-tags="' + tagsAttr + '" ' +
        'data-has-static-ip="' + hasStaticIPAttr + '" ' +
        'title="Info">' + INFO_ICON_SVG + '</button>';
}

// Set up a single delegated click handler for all DNS-names buttons.
// Called once during initAll() so that dynamically-rendered table rows are covered.
function InitInfoDialogHandler() {
    document.addEventListener('click', function(e) {
        var btn = e.target.closest('.info-btn');
        if (!btn) 
            return;

        // get the data specific to the clicked DHCP client:
        var friendlyName = btn.getAttribute('data-friendly-name');
        var hostname = btn.getAttribute('data-hostname');
        var macAddr = btn.getAttribute('data-mac');
        var rawNames = btn.getAttribute('data-dns-names');
        var description = btn.getAttribute('data-description');
        var link = btn.getAttribute('data-link');
        var rawTags = btn.getAttribute('data-tags');
        var hasStaticIP = btn.getAttribute('data-has-static-ip') === 'true';
        try {
            var dnsNames = JSON.parse(rawNames);
            var tags = JSON.parse(rawTags || '[]');
            showInfoDialog(friendlyName, hostname, dnsNames, macAddr, description, link, tags, hasStaticIP);
        } catch (err) {
            console.error('Failed to parse info dialog attributes:', err);
        }
    });
}

// Generate a consistent color for a tag based on its string
function getTagColor(tagString) {
    // Simple hash function to generate consistent colors for tags
    let hash = 0;
    for (let i = 0; i < tagString.length; i++) {
        hash = tagString.charCodeAt(i) + ((hash << 5) - hash);
    }
    
    // Generate HSL color with fixed saturation and lightness for better readability
    const hue = Math.abs(hash % 360);
    return `hsl(${hue}, 65%, 45%)`;
}

// Format tags as colored labels
function formatTags(tags) {
    if (!tags || tags.length === 0) {
        return '<span class="tag-label tag-none">none</span>';
    }
    
    return tags.map(tag => {
        const color = getTagColor(tag);
        const escapedTag = tag.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
        return `<span class="tag-label" style="background-color: ${color};">${escapedTag}</span>`;
    }).join(' ');
}

function formatTimeLeft(unixFutureTimestamp) {
    if (unixFutureTimestamp == 0) {
        return "Never expires";
    }

    // Calculate the difference in milliseconds between the timestamp and the current time
    const now = new Date();
    const timestampInMillis = unixFutureTimestamp * 1000;
    const timeDifference = timestampInMillis - now.getTime();

    // If the time has already passed, return 0
    if (timeDifference <= 0) {
        return "Already expired";
    }

    // Calculate the remaining time in hours, minutes, and seconds
    const hoursLeft = Math.floor(timeDifference / (1000 * 60 * 60));
    const minutesLeft = Math.floor((timeDifference % (1000 * 60 * 60)) / (1000 * 60));
    const secondsLeft = Math.floor((timeDifference % (1000 * 60)) / 1000);

    // Format the remaining time as a string "HH:MM:SS"
    return `${hoursLeft.toString().padStart(2, '0')}:${minutesLeft.toString().padStart(2, '0')}:${secondsLeft.toString().padStart(2, '0')}`;
}

function formatTimeSince(unixPastTimestamp) {
    if (unixPastTimestamp == 0) {
        return "Invalid timestamp";
    }

    // Calculate the difference in milliseconds between the timestamp and the current time
    const now = new Date();
    const timestampInMillis = unixPastTimestamp * 1000;
    const timeDifference = now.getTime() - timestampInMillis;

    // If the time has already passed, return 0
    if (timeDifference <= 0) {
        return "Timestamp in future?";
    }

    // Calculate the time difference in days, hours, minutes, and seconds
    const msecsInDay = 1000 * 60 * 60 * 24;
    const msecsInHour = 1000 * 60 * 60;
    const msecsInMinute = 1000 * 60;

    const days = Math.floor(timeDifference / msecsInDay);
    const hours = Math.floor((timeDifference % msecsInDay) / msecsInHour);
    const minutes = Math.floor((timeDifference % msecsInHour) / msecsInMinute);
    const seconds = Math.floor((timeDifference % msecsInMinute) / 1000);

    // Format the time as a string
    const dayPart = days > 0 ? `${days}d, ` : '';
    const timePart = `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${seconds.toString().padStart(2, '0')}`;
    
    return dayPart + timePart;
}


/* INIT FUNCTIONS */

function initTabs() {
    const tabButtons = document.querySelectorAll('.tabs__pills .btn');
    const tabContents = document.querySelectorAll('.tabs__panels > div');

    if (tabButtons && tabContents) {
        tabButtons.forEach((tabBtn) => {
            tabBtn.addEventListener('click', () => {
                // console.log("click intercepted")
                const tabId = tabBtn.getAttribute('data-id');

                tabButtons.forEach((btn) => btn.classList.remove('active'));
                tabBtn.classList.add('active');

                tabContents.forEach((content) => {
                    content.classList.remove('active');

                    if (content.id === tabId) {
                    content.classList.add('active');
                    }
                });
            });
        });
    }
}

function initCurrentTable() {
    console.log("Initializing table for current DHCP clients");

    // custom sorting for content formatted as HH:MM:SS
    $.fn.dataTable.ext.order['custom-time-order'] = function (settings, colIndex) {
        return this.api().column(colIndex, { order: 'index' }).nodes().map(function (td, i) {
            var time = $(td).text().split(':');
            // convert to seconds (HH * 3600 + MM * 60 + SS)
            return (parseInt(time[0], 10) * 3600) + (parseInt(time[1], 10) * 60) + parseInt(time[2], 10);
        });
    };
    global_objects.table_current = new DataTable('#current_table', {
            columns: [
                { title: '#', type: 'num' },
                { title: 'Name', type: 'string' },
                { title: 'Description', type: 'string' },
                { title: 'Link', type: 'string' },
                { title: 'IP Address', type: 'ip-address', render: renderWithCopyButton },
                { title: 'MAC Address', type: 'string', render: renderWithCopyButton },
                { title: 'Expires in', 'orderDataType': 'custom-time-order' },
                { title: 'Reserved IP?', type: 'string', width: '8%' },
                { title: 'Tags', type: 'string', width: '15%' }
            ],
            data: [],
            pageLength: config["numRows"],
            responsive: true,
            className: 'data-table',
            layout: {
                topStart: {
                    buttons: [
                        'copy', 'excel'
                    ]
                },
                topEnd: 'search',
                bottomStart: 'pageLength'
            }
        });
}

function initPastTable() {
    console.log("Initializing table for past DHCP clients");

    // custom sorting for content formatted as HH:MM:SS
    $.fn.dataTable.ext.order['custom-time-order'] = function (settings, colIndex) {
        return this.api().column(colIndex, { order: 'index' }).nodes().map(function (td, i) {
            var time = $(td).text().split(':');
            // convert to seconds (HH * 3600 + MM * 60 + SS)
            return (parseInt(time[0], 10) * 3600) + (parseInt(time[1], 10) * 60) + parseInt(time[2], 10);
        });
    };
    global_objects.table_past = new DataTable('#past_table', {
            columns: [
                { title: '#', type: 'num' },
                { title: 'Name', type: 'string' },
                { title: 'Description', type: 'string' },
                { title: 'MAC Address', type: 'string', render: renderWithCopyButton },
                { title: 'Reserved IP?', type: 'string', width: '8%' },
                { title: 'Last Seen hh:mm:ss ago', 'orderDataType': 'custom-time-order', width: '10%' },
                { title: 'Notes', type: 'string', width: '25%' },
                { title: 'Tags', type: 'string', width: '20%' }
            ],
            data: [],
            pageLength: config["numRows"],
            responsive: true,
            className: 'data-table',
            layout: {
                topStart: {
                    buttons: [
                        'copy', 'excel'
                    ]
                },
                topEnd: 'search',
                bottomStart: 'pageLength'
            }
        });
}

function initDnsCustomHostsTable(dnsCustomHosts) {
    console.log("Initializing table for custom DNS host records");

    global_objects.table_dns_hosts = new DataTable('#dns_hosts_table', {
            columns: [
                { title: '#', type: 'num' },
                { title: 'Custom DNS Host', type: 'string' },
                { title: 'IPv4 Address', type: 'ip-address' },
                { title: 'IPv6 Address', type: 'string' },
            ],
            data: [],
            pageLength: config["numRows"],
            responsive: true,
            className: 'data-table',
            layout: {
                topStart: null,
                topEnd: null
            }
        });

    if (dnsCustomHosts == null || dnsCustomHosts.length == 0) {
        console.log("No custom DNS host records found in the configuration file");
        global_objects.table_dns_hosts.clear().draw(false);
    } else {
        var tableData = [];
        dnsCustomHosts.forEach(function (item, index) {
            tableData.push([index + 1,
                item.name,
                item.ipv4_address || 'N/A',
                item.ipv6_address || 'N/A']);
        });
        global_objects.table_dns_hosts.clear().rows.add(tableData).draw(false);
    }
}

function initDnsUpstreamServersTable() {
    console.log("Initializing table for DNS upstream servers");

    global_objects.table_dns_upstreams = new DataTable('#dns_upstream_servers', {
            columns: [
                { title: '#', type: 'num' },
                { title: 'Upstream DNS server', type: 'string' },
                { title: 'Queries sent', type: 'num' },
                { title: 'Queries failed', type: 'num' },
            ],
            data: [],
            pageLength: config["numRows"],
            responsive: true,
            className: 'data-table',
            layout: {
                topStart: null,
                topEnd: null
            }
        });
}

function initTableDarkOrLightTheme() {
    let prefers = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    let html = document.querySelector('html');
    
    // see https://datatables.net/manual/styling/dark-mode#Auto-detection
    html.classList.add(prefers);
    html.setAttribute('data-bs-theme', prefers);

    console.log("Adapting the web UI to the auto-detected color-scheme: " + prefers);
}

function initAll() {
    initCurrentTable()
    initPastTable()
    initDnsCustomHostsTable(config["dnsCustomHosts"])
    initDnsUpstreamServersTable()
    initTabs()
    initTableDarkOrLightTheme()
    InitInfoDialogHandler()
}

function setConfig(webSocketURI, dhcpServerStartTime, dhcpPoolSize, dnsCustomHosts, numRows) {
    var parsedNumRows = parseInt(numRows, 10);
    if (!Number.isInteger(parsedNumRows) || parsedNumRows <= 0) {
        parsedNumRows = 20;
    }

    // update the global config variable
    config = {
        "webSocketURI": webSocketURI,
        "dhcpServerStartTime": dhcpServerStartTime,
        "dhcpPoolSize": dhcpPoolSize,
        "dnsCustomHosts": dnsCustomHosts,
        "numRows": parsedNumRows
    }

    // now that we have the URI of the websocket server, we can open the connection
    global_objects.backend_ws = new WebSocket(webSocketURI)

    global_objects.backend_ws.onopen = function (event) {
        console.log("Websocket connection to " + config["webSocketURI"] + " was successfully opened");
    };

    global_objects.backend_ws.onclose = function (event) {
        console.log("Websocket connection closed", event.code, event.reason, event.wasClean)
        updateLiveIndicator(false)
    }

    global_objects.backend_ws.onerror = function (event) {
        console.log("Websocket connection closed due to error", event.code, event.reason, event.wasClean)
        updateLiveIndicator(false)
    }

    global_objects.backend_ws.onmessage = function (event) {
        console.log("Websocket received event", event.code, event.reason, event.wasClean)
        processWebSocketEvent(event)
    }
}


/* DYNAMIC UPDATES PROCESSING FUNCTIONS */

function compareArraysIgnoringColumns(a, b, columnsToIgnore) {
    //console.log("ARRAY A:", a.toString());
    //console.log("ARRAY B:", b.toString());
    //return a.toString() === b.toString();

    if (a.length !== b.length) {
        return false;
    } else {
      // This is a 2D array: first go through rows
      for (var i = 0; i < a.length; i++) {

        // then go through columns
        for (var j = 0; j < a[i].length; j++) {

            if (columnsToIgnore.includes(j)) {
                continue; // Skip the columns to ignore
            }
            if (a[i][j] !== b[i][j]) {
                console.log("DIFFERENT AT ROW" + i + " COLUMN" + j + ": A=" + a[i][j] + " B=" + b[i][j]);
                return false;
            }
        }
      }
      
      return true;
    }
}
  
function processWebSocketDHCPCurrentClients(data) {
    console.log("Websocket connection: received " + data.current_clients.length + " current DHCP clients from websocket");

    // rerender the CURRENT table
    var newData = [];
    var newTimeLeftColumn = [];
    var dhcp_addresses_used = 0;
    var dhcp_static_ip = 0;
    data.current_clients.forEach(function (item, index) {
        console.log(`CurrentItem ${index + 1}:`, item);

        if (item.is_inside_dhcp_pool)
            dhcp_addresses_used += 1;

        var static_ip_str = "NO";
        if (item.has_static_ip) {
            static_ip_str = "YES";
            dhcp_static_ip += 1;
        }

        // Apparently not all browsers use fonts supporting the U+1F855 symbol... 
        // E.g. my Android phone does not render it
        //external_link_symbol="🡕" // https://www.compart.com/en/unicode/U+1F855

        // hopefully the U+29C9 symbol is more commonly supported:
        var external_link_symbol="⧉"; // https://www.compart.com/en/unicode/U+29C9
        var link_str;
        if (item.evaluated_link) {
            var escapedLink = item.evaluated_link.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
            link_str = "<a href=\"" + escapedLink + "\" target=\"_blank\">" + escapedLink + " " + external_link_symbol + "</a>";
        } else {
            link_str = "N/A";
        }

        // append new row
        var time_left_str = formatTimeLeft(item.lease.expires);
        var tags_str = formatTags(item.tags);
        var description_str = (item.description && item.description.length > 0) ? item.description : 'N/A';
        var hostname_str = renderNameWithInfoBtn(
            item.friendly_name,
            item.lease.hostname,
            item.dns_names,
            item.lease.mac_addr,
            item.description,
            item.evaluated_link,
            item.tags,
            item.has_static_ip,
            'display');
        newData.push([index + 1,
            hostname_str, description_str, link_str,
            item.lease.ip_addr, item.lease.mac_addr, 
            time_left_str, static_ip_str, tags_str]);
        newTimeLeftColumn.push(time_left_str);
    });

    var index_of_time_left_column = 6;
    var currentData = global_objects.table_current.data().toArray();
    if (compareArraysIgnoringColumns(currentData, newData, [index_of_time_left_column])) {
        console.log("No change in current DHCP clients, updating only the time-left column");

        // selective update to avoid unwanted resets of the current position (this is specially annoying
        // when using the responsive plugin and the user has expanded a collapsed row!!)
        for (var i = 0; i < currentData.length; i++) {
            global_objects.table_current.cell(i, index_of_time_left_column).data(newTimeLeftColumn[i]);
        }
        global_objects.table_current.draw(false);
    } else {
        console.log("There are changes for the current DHCP clients, refreshing the table");
        global_objects.table_current.clear().rows.add(newData).draw(false /* do not reset page position */);
    }

    return [dhcp_static_ip, dhcp_addresses_used]
}

function processWebSocketDHCPPastClients(data) {
    console.log("Websocket connection: received " + data.past_clients.length + " past DHCP clients from websocket");

    // rerender the PAST table
    var newData = [];
    var newLastSeenColumn = [];
    data.past_clients.forEach(function (item, index) {
        // console.log(`PastItem ${index + 1}:`, item);

        var static_ip_str = "NO";
        if (item.has_static_ip) {
            static_ip_str = "YES";
        }

        // append new row
        var last_seen_str = formatTimeSince(item.past_info.last_seen);
        var tags_str = formatTags(item.tags);
        var description_str = (item.description && item.description.length > 0) ? item.description : 'N/A';
        var hostname_str = renderNameWithInfoBtn(
            item.friendly_name,
            item.past_info.hostname,
            item.dns_names,
            item.past_info.mac_addr,
            item.description,
            item.evaluated_link,
            item.tags,
            item.has_static_ip,
            'display');
        newData.push([index + 1,
            hostname_str, description_str,
            item.past_info.mac_addr, static_ip_str, 
            last_seen_str, item.notes, tags_str]);
        newLastSeenColumn.push(last_seen_str);
    });

    var index_of_time_last_seen_column = 5;
    var currentData = global_objects.table_past.data().toArray();
    if (compareArraysIgnoringColumns(currentData, newData, [index_of_time_last_seen_column])) {
        console.log("No change in past DHCP clients, updating only the last-seen column");

        // selective update to avoid unwanted resets of the current position (this is specially annoying
        // when using the responsive plugin and the user has expanded a collapsed row!!)
        for (var i = 0; i < currentData.length; i++) {
            global_objects.table_past.cell(i, index_of_time_last_seen_column).data(newLastSeenColumn[i]);
        }
        global_objects.table_past.draw(false);

    } else {
        console.log("There are changes for the past DHCP clients, refreshing the table");
        global_objects.table_past.clear().rows.add(newData).draw(false /* do not reset page position */);
    }
}

function updateDHCPStatus(data, dhcp_static_ip, dhcp_addresses_used, messageElem) {
    // compute DHCP pool usage
    var usagePerc = 0
    if (config["dhcpPoolSize"] > 0) {
        usagePerc = 100 * dhcp_addresses_used / config["dhcpPoolSize"]

        // truncate to only 1 digit accuracy
        usagePerc = Math.round(usagePerc * 10) / 10
    }

    var usage_str = dhcp_addresses_used + " clients are within the DHCP pool. DHCP pool contains " + config["dhcpPoolSize"] + " IP addresses and its usage is at " + usagePerc + "%.<br/>"
    if (usagePerc > 90) {
        usage_str += "<span class='boldText warningText'>⚠ Warning: consider increasing the pool size in the configuration file if you are close to the limit. ⚠</span>";
    }

    // past clients string
    var uptime_str = formatTimeSince(config["dhcpServerStartTime"])
    var past_client_str = "<span class='boldText'>" + data.past_clients.length + " past clients</span> contacted the server some time ago but failed to do so since last DHCP server restart, " + 
                        uptime_str + " hh:mm:ss ago.<br/>"

    // build warning message if some clients are not using their configured address
    var counters_str = ""
    if (data.log_counters.not_using_configured_address > 0) {
        counters_str = "<span class='boldText warningText'>⚠ Warning: " + 
            data.log_counters.not_using_configured_address + 
            " time(s) a configured static address was not assigned to the configured MAC address because it was already leased to another device. " +
            "Please wait for the conflicting leases to expire or restart the affected devices.</span><br/>"
    } else {
        counters_str = "<span class='boldText'>0</span> occurrences of DHCP lease conflict.<br/>"
    }

    // update the message
    messageElem.innerHTML = "<span class='boldText'>" + data.current_clients.length + " clients</span> currently hold a DHCP lease.<br/>" + 
                        dhcp_static_ip + " clients have a reserved IP address.<br/>" +
                        usage_str +
                        past_client_str +
                        counters_str;
}

function updateDNSStatus(data, messageElem) {
    console.log(`DnsStats:`, data.dns_stats);

    // rerender the UPSTREAM SERVERS table
    var tableData = [];
    if (data.dns_stats.upstream_servers_stats != null) {
        data.dns_stats.upstream_servers_stats.forEach(function (item, index) {
            console.log(`Upstream ${index + 1}:`, item);

            // append new row
            tableData.push([index + 1,
                item.server_url, 
                item.queries_sent, 
                item.queries_failed]);
        });
        global_objects.table_dns_upstreams.clear().rows.add(tableData).draw(false /* do not reset page position */);
    }

    // update the message
    messageElem.innerHTML = 
        "Cache size: <span class='boldText'>" + data.dns_stats.cache_size + "</span><br/>" +
        "Cache insertions: <span class='boldText'>" + data.dns_stats.cache_insertions + "</span><br/>" +
        "Cache evictions: <span class='boldText'>" + data.dns_stats.cache_evictions + "</span><br/>" +
        "Cache misses: <span class='boldText'>" + data.dns_stats.cache_misses + "</span><br/>" +
        "Cache hits: <span class='boldText'>" + data.dns_stats.cache_hits + "</span><br/>"
        ;
}

function updateLiveIndicator(isLive) {
    var liveElem = document.getElementById("websocket_conn_status");

    // change the source image for the live indicator
    liveElem.src = isLive ? "static/ok.png" : "static/ko.png";
    console.log("Updated live indicator to " + liveElem.src);
}

function processWebSocketEvent(event) {

    try {
        var data = JSON.parse(event.data);
    } catch (error) {
        console.error('Error while parsing JSON:', error);
        return;
    }

    var dhcpMsgElem = document.getElementById("dhcp_stats_message");
    var dnsMsgElem = document.getElementById("dns_stats_message");
    assert(dhcpMsgElem != null);
    assert(dnsMsgElem != null);

    if (data === null) {
        console.log("Websocket connection: received an empty JSON");

        // clear the table
        global_objects.table_current.clear().draw();
        global_objects.table_past.clear().draw();

        dhcpMsgElem.innerText = "No DHCP clients so far.";
        dnsMsgElem.innerText = "No DNS stats so far.";

    } else if (!("current_clients" in data) || 
                !("past_clients" in data) ||
                !("dns_stats" in data)) {
        console.error("Websocket connection: expecting a JSON matching the golang WebSocketMessage type, received something else", data);

        // clear the table
        global_objects.table_current.clear().draw();
        global_objects.table_past.clear().draw();

        dhcpMsgElem.innerText = "Internal error. Please report upstream together with Javascript logs.";
        dnsMsgElem.innerText = "Internal error. Please report upstream together with Javascript logs.";

    } else {
        // console.log("DEBUG:" + JSON.stringify(data))
        global_objects.num_updates += 1
        console.log("****** Update " + global_objects.num_updates + " ******");

        // process DHCP 
        var [dhcp_static_ip, dhcp_addresses_used] = processWebSocketDHCPCurrentClients(data)
        processWebSocketDHCPPastClients(data)
        updateDHCPStatus(data, dhcp_static_ip, dhcp_addresses_used, dhcpMsgElem)

        // process DNS
        updateDNSStatus(data, dnsMsgElem)

        // update live update indicator
        updateLiveIndicator(true)
    }
}


// init code
document.addEventListener('DOMContentLoaded', initAll, false);
