# Changelog

For the full changelog please check https://github.com/f18m/ha-addon-dnsmasq-dhcp-server/releases.

This file mainly contains only migration instructions from a major version to the next major version. A new major version is released each time there is a backward-incompatible change in the config format.
Additionally some notable new features are also documented here as well.

## Version 5.1.3

> :warning: 💥 Revert unstable change introduced in 5.1.2

The 5.1.3 release is rolling back the new configuration `bind-dynamic` implemented in previous release 5.1.2.
The reason is that the `bind-dynamic` feature in dnsmasq suffers of a race condition and dnsmasq will randomly disconnect its DNS listener from some interfaces (especially on IPv4).
If you rely on this addon to provide DNS and you use IPv4 in your network, then you should upgrade immediately to this release 5.1.3, which is rolling back to the previous, more tested dnsmasq configuration.


## Older changelog notes

> [Older changelog entries available in git history]


## Star the project

If you like this app, please give it a star on [Github](https://github.com/f18m/ha-addon-dnsmasq-dhcp). Thanks!
