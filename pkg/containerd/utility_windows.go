//go:build windows
// +build windows

package containerd

import (
	util2 "github.com/k3s-io/k3s/pkg/util"
	pkgerrors "github.com/pkg/errors"
)

func OverlaySupported(root string) error {
	return pkgerrors.WithMessagef(util2.ErrUnsupportedPlatform, "overlayfs is not supported")
}

func FuseoverlayfsSupported(root string) error {
	return pkgerrors.WithMessagef(util2.ErrUnsupportedPlatform, "fuse-overlayfs is not supported")
}

func StargzSupported(root string) error {
	return pkgerrors.WithMessagef(util2.ErrUnsupportedPlatform, "stargz is not supported")
}
