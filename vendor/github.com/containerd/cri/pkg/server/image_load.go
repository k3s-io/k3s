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
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	api "github.com/containerd/cri/pkg/api/v1"
	"github.com/containerd/cri/pkg/containerd/importer"
)

// LoadImage loads a image into containerd.
func (c *criService) LoadImage(ctx context.Context, r *api.LoadImageRequest) (*api.LoadImageResponse, error) {
	path := r.GetFilePath()
	if !filepath.IsAbs(path) {
		return nil, errors.Errorf("path %q is not an absolute path", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file")
	}
	repoTags, err := importer.Import(ctx, c.client, f, importer.WithUnpack(c.config.ContainerdConfig.Snapshotter))
	if err != nil {
		return nil, errors.Wrap(err, "failed to import image")
	}
	for _, repoTag := range repoTags {
		// Update image store to reflect the newest state in containerd.
		// Image imported by importer.Import is not treated as managed
		// by the cri plugin, call `updateImage` to make it managed.
		// TODO(random-liu): Replace this with the containerd library (issue #909).
		if err := c.updateImage(ctx, repoTag); err != nil {
			return nil, errors.Wrapf(err, "update image store %q", repoTag)
		}
		logrus.Debugf("Imported image %q", repoTag)
	}
	return &api.LoadImageResponse{Images: repoTags}, nil
}
