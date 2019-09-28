#!/usr/bin/env bash

auto-mtu() {
	local mtu=0 endpoint output
	while read -r _ endpoint; do
		[[ $endpoint =~ ^\[?([a-z0-9:.]+)\]?:[0-9]+$ ]] || continue
		output="$(ip route get "${BASH_REMATCH[1]}" || true)"
		[[ ( $output =~ mtu\ ([0-9]+) || ( $output =~ dev\ ([^ ]+) && $(ip link show dev "${BASH_REMATCH[1]}") =~ mtu\ ([0-9]+) ) ) && ${BASH_REMATCH[1]} -gt $mtu ]] && mtu="${BASH_REMATCH[1]}"
	done < <(wg show "$1" endpoints)
	if [[ $mtu -eq 0 ]]; then
		read -r output < <(ip route show default || true) || true
		[[ ( $output =~ mtu\ ([0-9]+) || ( $output =~ dev\ ([^ ]+) && $(ip link show dev "${BASH_REMATCH[1]}") =~ mtu\ ([0-9]+) ) ) && ${BASH_REMATCH[1]} -gt $mtu ]] && mtu="${BASH_REMATCH[1]}"
	fi
	[[ $mtu -gt 0 ]] || mtu=1500
	ip link set mtu $(( mtu - 80 )) up dev "$1"
}

# probe for any modules that may be needed
modprobe wireguard 
modprobe tun

# try wireguard kernel module first
ip link add "$1" type wireguard && exit

# try boringtun and let it drop privileges
boringtun "$1" && auto-mtu "$1" && exit

# try boringtun w/o dropping privileges
WG_SUDO=1 boringtun "$1" && auto-mtu "$1" && exit

# try wireguard-go - p.s. should not use wireguard-go, it leaks memory
WG_I_PREFER_BUGGY_USERSPACE_TO_POLISHED_KMOD=1 wireguard-go "$1" && auto-mtu "$1" && exit

exit 1
