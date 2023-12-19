#!/bin/bash
ip4_addr=$1
ip6_addr=$2
ip6_addr_gw=$3
os=$4

sysctl -w net.ipv6.conf.all.disable_ipv6=0
sysctl -w net.ipv6.conf.eth1.accept_dad=0
sysctl -w net.ipv6.conf.eth1.accept_ra=0
sysctl -w net.ipv6.conf.eth1.forwarding=0

if [ -z "${os##*ubuntu*}" ]; then
  netplan set ethernets.eth1.accept-ra=false
  netplan set ethernets.eth1.addresses=["$ip4_addr"/24,"$ip6_addr"/64]
  netplan set ethernets.eth1.gateway6="$ip6_addr_gw"
  netplan apply
elif [ -z "${os##*alpine*}" ]; then
  iplink set eth1 down
  iplink set eth1 up
  ip -6 addr add "$ip6_addr"/64 dev eth1
  ip -6 r add default via "$ip6_addr_gw"
else
  ip -6 addr add "$ip6_addr"/64 dev eth1
  ip -6 r add default via "$ip6_addr_gw"
fi
ip addr show dev eth1
ip -6 r
