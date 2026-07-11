//go:build !windows

package flannel

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/google/renameio/v2"
	"github.com/k3s-io/k3s/pkg/util/errors"
)

// WriteSubnetFile atomically writes the flannel subnet configuration file.
// Uses google/renameio for safe atomic write semantics, which handles creating
// a temporary file, syncing, closing, renaming, and directory fsync.
func WriteSubnetFile(path string, nw ip.IP4Net, nwv6 ip.IP6Net, ipMasq bool, bn backend.Network, nm netMode) error {
	dir, _ := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.WithMessage(err, "mkdir subnet directory")
	}

	// Preserve original file permissions if the file already exists
	perm := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	f, err := renameio.TempFile(dir, path)
	if err != nil {
		return errors.WithMessage(err, "create temp file")
	}
	defer f.Cleanup()

	if err := f.Chmod(perm); err != nil {
		return errors.WithMessage(err, "chmod temp file")
	}

	// We lease a subnet for the node from the cluster state (etcd)
	sn := bn.Lease().Subnet
	// Increment from network address to the first usable host address
	sn.IP++
	if nm.IPv4Enabled() {
		// Save the CIDR assigned to flannel
		if _, err := fmt.Fprintf(f, "FLANNEL_NETWORK=%s\n", nw); err != nil {
			return errors.WithMessage(err, "failed to write FLANNEL_NETWORK")
		}
		// Save the first usable address in the node's subnet
		if _, err := fmt.Fprintf(f, "FLANNEL_SUBNET=%s\n", sn); err != nil {
			return errors.WithMessage(err, "failed to write FLANNEL_SUBNET")
		}
	}

	if nwv6.String() != emptyIPv6Network {
		// We lease a subnet for the node from the cluster state (etcd)
		snv6 := bn.Lease().IPv6Subnet
		// Increment from network address to the first usable host address
		snv6.IncrementIP()
		// Save the CIDR assigned to flannel
		if _, err := fmt.Fprintf(f, "FLANNEL_IPV6_NETWORK=%s\n", nwv6); err != nil {
			return errors.WithMessage(err, "failed to write FLANNEL_IPV6_NETWORK")
		}
		// Save the first usable address in the node's subnet
		if _, err := fmt.Fprintf(f, "FLANNEL_IPV6_SUBNET=%s\n", snv6); err != nil {
			return errors.WithMessage(err, "failed to write FLANNEL_IPV6_SUBNET")
		}
	}

	if _, err := fmt.Fprintf(f, "FLANNEL_MTU=%d\n", bn.MTU()); err != nil {
		return errors.WithMessage(err, "failed to write FLANNEL_MTU")
	}
	if _, err := fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq); err != nil {
		return errors.WithMessage(err, "failed to write FLANNEL_IPMASQ")
	}

	return f.CloseAtomicallyReplace()
}
