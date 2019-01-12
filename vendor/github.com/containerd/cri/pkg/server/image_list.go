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
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

// ListImages lists existing images.
// TODO(random-liu): Add image list filters after CRI defines this more clear, and kubelet
// actually needs it.
func (c *criService) ListImages(ctx context.Context, r *runtime.ListImagesRequest) (*runtime.ListImagesResponse, error) {
	imagesInStore := c.imageStore.List()

	var images []*runtime.Image
	for _, image := range imagesInStore {
		// TODO(random-liu): [P0] Make sure corresponding snapshot exists. What if snapshot
		// doesn't exist?
		images = append(images, toCRIImage(image))
	}

	return &runtime.ListImagesResponse{Images: images}, nil
}
