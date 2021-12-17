
# Known Issues

The Known Issues are updated periodically and designed to inform you about any issues that may not be immediately addressed in the next upcoming release.

**Snap Docker**

If you plan to use K3s with docker, Docker installed via a snap package is not recommended as it has been known to cause issues running K3s.

**Iptables**

If you are running iptables in nftables mode instead of legacy you might encounter issues. We recommend utilizing newer iptables (such as 1.6.1+) to avoid issues.

**Rootless Mode**

Running K3s with Rootless mode is experimental and has several [known issues.](advanced.md#known-issues-with-rootless-mode)
