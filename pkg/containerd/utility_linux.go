//go:build linux

package containerd

import (
	"github.com/containerd/containerd/snapshots/overlay/overlayutils"
	fuseoverlayfs "github.com/containerd/fuse-overlayfs-snapshotter"
	stargz "github.com/containerd/stargz-snapshotter/service"
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
