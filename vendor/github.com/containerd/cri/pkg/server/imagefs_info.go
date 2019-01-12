/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"time"

	"golang.org/x/net/context"

	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// ImageFsInfo returns information of the filesystem that is used to store images.
func (c *criService) ImageFsInfo(ctx context.Context, r *runtime.ImageFsInfoRequest) (*runtime.ImageFsInfoResponse, error) {
	snapshots := c.snapshotStore.List()
	timestamp := time.Now().UnixNano()
	var usedBytes, inodesUsed uint64
	for _, sn := range snapshots {
		// Use the oldest timestamp as the timestamp of imagefs info.
		if sn.Timestamp < timestamp {
			timestamp = sn.Timestamp
		}
		usedBytes += sn.Size
		inodesUsed += sn.Inodes
	}
	// TODO(random-liu): Handle content store
	return &runtime.ImageFsInfoResponse{
		ImageFilesystems: []*runtime.FilesystemUsage{
			{
				Timestamp:  timestamp,
				FsId:       &runtime.FilesystemIdentifier{Mountpoint: c.imageFSPath},
				UsedBytes:  &runtime.UInt64Value{Value: usedBytes},
				InodesUsed: &runtime.UInt64Value{Value: inodesUsed},
			},
		},
	}, nil
}
