/*
Copyright 2018 The containerd Authors.

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

package opts

import (
	"context"

	"github.com/containerd/containerd/containers"
	"github.com/containerd/containerd/oci"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"
)

// WithAdditionalGIDs adds any additional groups listed for a particular user in the
// /etc/groups file of the image's root filesystem to the OCI spec's additionalGids array.
func WithAdditionalGIDs(userstr string) oci.SpecOpts {
	return func(ctx context.Context, client oci.Client, c *containers.Container, s *runtimespec.Spec) (err error) {
		gids := s.Process.User.AdditionalGids
		if err := oci.WithAdditionalGIDs(userstr)(ctx, client, c, s); err != nil {
			return err
		}
		// Merge existing gids and new gids.
		s.Process.User.AdditionalGids = mergeGids(s.Process.User.AdditionalGids, gids)
		return nil
	}
}

func mergeGids(gids1, gids2 []uint32) []uint32 {
	for _, gid1 := range gids1 {
		for i, gid2 := range gids2 {
			if gid1 == gid2 {
				gids2 = append(gids2[:i], gids2[i+1:]...)
				break
			}
		}
	}
	return append(gids1, gids2...)
}
