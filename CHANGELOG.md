# Changelog

For the full changelog please check https://github.com/f18m/ha-addon-dnsmasq-dhcp-server/releases.

This file mainly contains only migration instructions from a major version to the next major version. A new major version is released each time there is a backward-incompatible change in the config format.
Additionally some notable new features are also documented here as well.

## Version 5.0.1

Please note that version 5.0.0 has been skipped due to packaging problems.

> :warning: 💥 Breaking Changes

Compared to v4, in v5 the `dhcp_ip_address_reservations` and `dhcp_clients_friendly_names` lists have been merged together and renamed to `dhcp_client_settings`.
Now all the DHCP entries configured appear under the same list, instead of two similar but slightly-different lists.
All entries in the `dhcp_client_settings` list must:

* have a valid DNS hostname specified as `name` parameter (RFC1123 compliant; in short strings containing only letters, digits, dots and dashes)
* have a parameter named `reserved_ip` in case you want to reserve an IP for them

The **upgrade procedure from version 4.x.x** consists in:

1. In Home Assistant UI go to `Settings->Apps->Dnsmasq-DHCP`; you should have a screen indicating that version 5.0.0 is now available; at this point, do not update yet.

2. Click the `Configuration` tab for the Dnsmasq-DHCP App and click "Edit in YAML" from the more-options menu (3 vertical dots).
Then copy and paste the whole config somewhere as backup in case you ever need to rollback to version 4.x.y

3. Open the copy-pasted config in your favorite editor and use the replace-all command to replace the string `dhcp_ip_address_reservations` with `dhcp_client_settings`

4. In your favorite editor use the replace-all command to replace the string `ip: ` with `reserved_ip: ` (all occurrences; please be careful to include the colon and the space after the colon to avoid replacing by mistake
other configuration settings)

5. In your favorite editor move all entries of the `dhcp_clients_friendly_names` list in the `dhcp_client_settings` list; for each entry make sure to provide a valid hostname (containing only letters, digits and dashes) for its `name`. Entries in this list used to have a relaxed check on the `name` property. Now this is not the case any longer as `name` will be used on the DNS protocol and thus needs to comply with RFC1123 specs.
If you had spaces, underscores or other characters now invalid in the `name`, please consider now using the `description` field to store such human-friendly string.
Except for the `name`, no change is needed in remaining parameters for the entries migrated from `dhcp_clients_friendly_names` to `dhcp_client_settings` list.

6. Remove the `dhcp_clients_friendly_names:` string which should be now an empty list.
Your configuration is now migrated correctly! 

7. Go back to the Home Assistant UI and click "Stop" in the `Info` tab to stop the Dnsmasq-DHCP app and then click the "Update" button to download v5. Go to the `Configuration` tab, click "Edit in YAML" again and copy-paste the upgraded v5 config on the YAML editor for the Dnsmasq-DHCP configuration. Then Hit "Save" (bottom of the `Configuration` tab).

8. Go back to the `Info` tab for the Dnsmasq-DHCP app and click "Start". 
Then click on the `Log` tab and check the log for errors. If you see the app restarting continuously then look carefully in the log for an error. Typically you just have a syntax error in the YAML config file (fix that and save the update configuration till the App stops complaining and runs in a stable way).

Note that this procedure is designed to reduce to a minimum the downtime of the app (Step 7-8).
This is important because while you stop the App, all your DHCP clients won't be able to renew their leases and this might result in Unavailable Entitities in Home Assistant, non-functional automations, etc.


> ✅ New Feature: DNS aliases

This release adds support for `DNS aliases`; in short these are [DNS CNAME entries](https://en.wikipedia.org/wiki/CNAME_record). 

A CNAME entry is a Canonical Name (CNAME) record that maps one domain name (an alias) to another (the canonical name).
DNS aliases are specified in the `dhcp_client_settings.<entry>.dns_aliases` key.
See [DOCS.md](https://github.com/f18m/ha-addon-dnsmasq-dhcp/blob/main/DOCS.md) for more details.

This allows you to configure dnsmasq-DHCP so that a particular DHCP client is resolvable with multiple names inside your network (e.g. have all strings `network-storage.lan`, `my-nas.lan` and `nas.lan` resolve to the same IP address connected with the `nas` device of your network).

DNS Aliases are available both for DHCP clients having a reserved IP and for those having a dynamic IP.



## Version 4.3.0

This release contains only a UI refresh.
There are no changes to the configuration files and or DHCP/DNS exposed features.

## Version 4.2.0

This release adds supports for `DNS custom hosts`, i.e. additional custom entries that the DNS server will resolve to the provided IPv4/IPv6 address(es). The DNS server will create A, AAAA and PTR records for each
entry in the DNS custom hosts list. These additional entries allow you to e.g. associate a resolvable Fully Qualified Domain Name (FQDN) to devices that are configured to use a static IP address (and as such are "invisible" to the DHCP server).

This release also resolves a long-standing problem for new users: the presence of an hardcoded network interface name in the default configuration has been removed. Now by default the DHCP pool that is present
in the default configuration is associated to the `auto` network interface, which is resolved at runtime
to the first interface which can egress traffic to the Internet.
This helps to get the first configuraton right and get started with this app.

## Version 4.1.0

This release adds support for `tags`: each DHCP IP address reservation can now be 
enriched with a list of tags. Same thing for each DHCP friendly name mapping.
This helps to give categories to all your LAN devices and easily search them in the UI.

This release also adds support for a `description` associated to each DHCP IP address reservation or DHCP friendly name mapping.

Finally it adds support for a MAC address blocklist: a list of MAC addresses that
will be completely ignored by the DHCP server, see `dhcp_mac_address_blocklist`

All configuration file changes mentioned above are backward compatible; however please be aware that this version now includes many more checks for configuration file coherency. E.g. a MAC address cannot appear both as inside an DHCP IP address reservation and as part of a DHCP friendly name mapping.
In case the app detects such kind of misconfigurations, it will immediately stop at startup. So you might need to spend some time adjusting your configuration following the addon startup errors.
Or you might want to spend some time to better organize 
your DHCP mappings with tags and descriptions now :)

## Version 4.0.0

> :warning: 💥 Breaking Changes

This release removes support for the `armv7`, `i386`, and `armhf` architectures which were
announced as deprecated in the Home Assistant architecture decisions, back in April 2025.

Please see [official thread about deprecations of these architectures](https://community.home-assistant.io/t/feedback-requested-deprecating-core-supervised-i386-armhf-armv7/880968) for further information.


Please note that beside such deprecation, there are no other breaking changes.
The configuration you have for dnsmasq-dhcp version `3.x.x` remains valid in version `4.0.0`.


## Star the project

If you like this app, please give it a star on [Github](https://github.com/f18m/ha-addon-dnsmasq-dhcp). Thanks!