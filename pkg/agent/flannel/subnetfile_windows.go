//go:build windows

package flannel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/flannel-io/flannel/pkg/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/k3s-io/k3s/pkg/util/errors"
)

// WriteSubnetFile writes the flannel subnet configuration file.
// renameio does not support Windows, and maintaining a full atomic write
// implementation for a platform without reliable atomic rename is not
// worth the complexity. Use a simple write instead.
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

	var buf strings.Builder

	// We lease a subnet for the node from the cluster state (etcd)
	sn := bn.Lease().Subnet
	// Increment from network address to the first usable host address
	sn.IP++
	if nm.IPv4Enabled() {
		fmt.Fprintf(&buf, "FLANNEL_NETWORK=%s\n", nw)
		fmt.Fprintf(&buf, "FLANNEL_SUBNET=%s\n", sn)
	}

	if nwv6.String() != emptyIPv6Network {
		// We lease a subnet for the node from the cluster state (etcd)
		snv6 := bn.Lease().IPv6Subnet
		// Increment from network address to the first usable host address
		snv6.IncrementIP()
		fmt.Fprintf(&buf, "FLANNEL_IPV6_NETWORK=%s\n", nwv6)
		fmt.Fprintf(&buf, "FLANNEL_IPV6_SUBNET=%s\n", snv6)
	}

	fmt.Fprintf(&buf, "FLANNEL_MTU=%d\n", bn.MTU())
	fmt.Fprintf(&buf, "FLANNEL_IPMASQ=%v\n", ipMasq)

	return errors.WithMessage(os.WriteFile(path, []byte(buf.String()), perm), "write subnet file")
}
