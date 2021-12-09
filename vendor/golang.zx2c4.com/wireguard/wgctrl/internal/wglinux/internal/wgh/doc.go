// Package wgh is an auto-generated package which contains constants and
// types used to access WireGuard information using generic netlink.
package wgh

// Pull the latest wireguard.h from GitHub for code generation.
//go:generate wget https://raw.githubusercontent.com/torvalds/linux/master/include/uapi/linux/wireguard.h

// Generate Go source from C constants.
//go:generate c-for-go -out ../ -nocgo wgh.yml

// Clean up build artifacts.
//go:generate rm -rf wireguard.h _obj/
