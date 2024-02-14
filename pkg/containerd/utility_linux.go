//go:build linux

package containerd

import (
	"github.com/containerd/containerd/v2/plugins/snapshots/overlay/overlayutils"
	fuseoverlayfs "github.com/containerd/fuse-overlayfs-snapshotter/v2"
	stargz "github.com/containerd/stargz-snapshotter/service"
	"github.com/pdtpartners/nix-snapshotter/pkg/nix"
)

func OverlaySupported(root string) error {
	return overlayutils.Supported(root)
}

func FuseoverlayfsSupported(root string) error {
	return fuseoverlayfs.Supported(root)
}

func StargzSupported(root string) error {
	return stargz.Supported(root)
}

func NixSupported(root string) error {
	return nix.Supported(root)
}
