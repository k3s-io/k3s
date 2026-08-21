// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package untar untars a tarball to disk.
package untar

import (
	"io"

	"github.com/k3s-io/k3s/pkg/util/errors"
	"github.com/klauspost/compress/zstd"
	"github.com/nlepage/go-tarfs"
	"github.com/otiai10/copy"
)

const MaxDecoderMemory = uint64(1 << 25)

// Untar reads the zstd-compressed tar file from r and writes it into dir.
func Untar(r io.Reader, dir string) error {
	return untar(r, dir)
}

func untar(r io.Reader, dir string) (err error) {
	zr, err := zstd.NewReader(r, zstd.WithDecoderMaxMemory(MaxDecoderMemory))
	if err != nil {
		return errors.WithMessage(err, "failed to extract zstd-compressed body")
	}
	defer zr.Close()
	tfs, err := tarfs.New(zr)
	if err != nil {
		return errors.WithMessage(err, "failed to open tar reader")
	}
	err = copy.Copy(".", dir, copy.Options{
		PermissionControl: copy.AddPermission(0755),
		PreserveOwner:     false,
		PreserveTimes:     false,
		NumOfWorkers:      0,
		Sync:              true,
		FS:                tfs,
	})
	return errors.WithMessage(err, "failed to copy files")
}
