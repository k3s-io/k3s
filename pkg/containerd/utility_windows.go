//go:build windows
// +build windows

package containerd

import (
	util2 "github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
)

func OverlaySupported(root string) error {
	return errors.Wrapf(util2.ErrUnsupportedPlatform, "overlayfs is not supported")
}

func FuseoverlayfsSupported(root string) error {
	return errors.Wrapf(util2.ErrUnsupportedPlatform, "fuse-overlayfs is not supported")
}

func StargzSupported(root string) error {
	return errors.Wrapf(util2.ErrUnsupportedPlatform, "stargz is not supported")
}

func NixSupported(root string) error {
	return errors.Wrapf(util2.ErrUnsupportedPlatform, "nix is not supported")
}
