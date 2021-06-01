// +build linux

package containerd

import (
	"github.com/containerd/containerd/snapshots/overlay"
	fuseoverlayfs "github.com/containerd/fuse-overlayfs-snapshotter"
)

func OverlaySupported(root string) error {
	return overlay.Supported(root)
}

func FuseoverlayfsSupported(root string) error {
	return fuseoverlayfs.Supported(root)
}
