# Changelog

For the changelog please check https://github.com/f18m/ha-addon-dnsmasq-dhcp-server/releases.

This file contains only migration instructions from a major version to the next major version.
A new major version is released each time there is a backward-incompatible change in the config format.


## Version 4.1.0

This release adds support for "tags": each DHCP IP address reservation can now be 
enriched with a list of tags. Same thing for each DHCP Friendly Name mapping.
This change does not require you to do any change on your configuration and is 
backward compatible. But you might want to spend some time to better organize 
your DHCP mappings with tags now :)


## Version 4.0.0

> :warning: 💥 Breaking Changes

This release removes support for the `armv7`, `i386`, and `armhf` architectures which were
announced as deprecated in the Home Assistant architecture decisions, back in April 2025.

Please see [official thread about deprecations of these architectures](https://community.home-assistant.io/t/feedback-requested-deprecating-core-supervised-i386-armhf-armv7/880968) for further information.


Please note that beside such deprecation, there are no other breaking changes.
The configuration you have for dnsmasq-dhcp version `3.x.x` remains valid in version `4.0.0`.
