# Changelog

For the changelog please check https://github.com/f18m/ha-addon-dnsmasq-dhcp-server/releases.

This file contains only migration instructions from a major version to the next major version.
A new major version is released each time there is a backward-incompatible change in the config format.


## Version 4.0.0

> :warning: 💥 Breaking Changes

This release removes support for the `armv7`, `i386`, and `armhf` architectures which were
announced as deprecated in the Home Assistant architecture decisions, back in April 2025.

Please see [official thread about deprecations of these architectures](https://community.home-assistant.io/t/feedback-requested-deprecating-core-supervised-i386-armhf-armv7/880968) for further information.


Please note that beside such deprecation, there are no other breaking changes.
The configuration you have for dnsmasq-dhcp version `3.x.x` remains valid in version `4.0.0`.

## Migrating from version 2.0.x to 3.0

If you have a valid configuration for version 2.0.x, you need to adjust the YAML configuration when migrating
to version 3.0.

1. The top-level "interface" key has been renamed to "interfaces" (plural) and now expects a YAML list of network interface names.
1. A new top-level "dhcp_pools" key has been created taking a list of IP ranges and the network interfaces on which these IP ranges should be served by the DHCP server. Additionally it also takes a "gateway" and "netmask" keys to specify critical aspects of each IP network.
1. The top-level "dhcp_network" key does not exist anymore. Some of its contents ("gateway" and "netmask" keys) 
have been moved in the new top-level "dhcp_pools" key. Some of its contents ("dns_domain", "dns_servers" and "ntp_servers") have been moved in the pre-existing top-level "dhcp_server" key.
Finally the "broadcast" key has been removed.
