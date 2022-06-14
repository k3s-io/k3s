#!/bin/bash
ip4_addr=$1
ip6_addr=$2
os=$3

sysctl -w net.ipv6.conf.all.disable_ipv6=0
sysctl -w net.ipv6.conf.eth1.accept_dad=0

if [ -z "${os##*ubuntu*}" ]; then
  netplan set ethernets.eth1.accept-ra=false
  netplan set ethernets.eth1.addresses=["$ip4_addr"/24,"$ip6_addr"/64]
  netplan apply
else
  ip -6 addr add "$ip6_addr"/64 dev eth1
fi
ip addr show dev eth1