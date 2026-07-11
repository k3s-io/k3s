//go:build windows

package flannel

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/k3s-io/k3s/pkg/util/errors"
	"golang.org/x/sys/windows"
)

// WriteSubnetFile atomically writes the flannel subnet configuration file.
// Windows does not provide a reliable atomic file replacement syscall, so this
// is a best-effort approximation. The file is written to a temporary location
// on the same directory, synced, then moved into place via MoveFileEx with
// MOVEFILE_WRITE_THROUGH to flush metadata as much as the system allows.
func WriteSubnetFile(path string, nw ip.IP4Net, nwv6 ip.IP6Net, ipMasq bool, bn backend.Network, nm netMode) error {
	dir, name := filepath.Split(path)
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

	// Uses a dotted-prefix pattern (".subnet.env.") that matches
	// what renameio generates internally on Unix.
	f, err := os.CreateTemp(dir, "."+name+".")
	if err != nil {
		return errors.WithMessage(err, "create temp file")
	}
	// On early exit (before the move), remove the temp file.
	// After a successful MoveFileEx the source path no longer exists,
	// so this becomes a harmless no-op.
	defer os.Remove(f.Name())

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

	// 1. Write
	if _, err := fmt.Fprintf(f, "FLANNEL_MTU=%d\n", bn.MTU()); err != nil {
		return errors.WithMessage(err, "failed to write FLANNEL_MTU")
	}
	if _, err := fmt.Fprintf(f, "FLANNEL_IPMASQ=%v\n", ipMasq); err != nil {
		return errors.WithMessage(err, "failed to write FLANNEL_IPMASQ")
	}

	// 2. Flush file contents
	if err := f.Sync(); err != nil {
		f.Close()
		return errors.WithMessage(err, "flush sync has failed")
	}

	// 3. Close the file
	if err := f.Close(); err != nil {
		return errors.WithMessage(err, "file close has failed")
	}

	src, err := windows.UTF16PtrFromString(f.Name())
	if err != nil {
		return errors.WithMessage(err, "UTF16 encoding has failed")
	}

	dst, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return errors.WithMessage(err, "UTF16 encoding has failed")
	}

	// 4. Atomically swap the temp file into place.
	// MOVEFILE_REPLACE_EXISTING overwrites the destination if present.
	// MOVEFILE_WRITE_THROUGH requests the move and its metadata be flushed
	// to persistent storage before returning, as close to fsync(dir) as
	// Windows supports for this operation.
	return windows.MoveFileEx(
		src,
		dst,
		windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH,
	)
}
