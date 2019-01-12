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
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/errdefs"
	containerdimages "github.com/containerd/containerd/images"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	imagespec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	runtime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"github.com/containerd/cri/pkg/util"
)

// For image management:
// 1) We have an in-memory metadata index to:
//   a. Maintain ImageID -> RepoTags, ImageID -> RepoDigset relationships; ImageID
//   is the digest of image config, which conforms to oci image spec.
//   b. Cache constant and useful information such as image chainID, config etc.
//   c. An image will be added into the in-memory metadata only when it's successfully
//   pulled and unpacked.
//
// 2) We use containerd image metadata store and content store:
//   a. To resolve image reference (digest/tag) locally. During pulling image, we
//   normalize the image reference provided by user, and put it into image metadata
//   store with resolved descriptor. For the other operations, if image id is provided,
//   we'll access the in-memory metadata index directly; if image reference is
//   provided, we'll normalize it, resolve it in containerd image metadata store
//   to get the image id.
//   b. As the backup of in-memory metadata in 1). During startup, the in-memory
//   metadata could be re-constructed from image metadata store + content store.
//
// Several problems with current approach:
// 1) An entry in containerd image metadata store doesn't mean a "READY" (successfully
// pulled and unpacked) image. E.g. during pulling, the client gets killed. In that case,
// if we saw an image without snapshots or with in-complete contents during startup,
// should we re-pull the image? Or should we remove the entry?
//
// yanxuean: We cann't delete image directly, because we don't know if the image
// is pulled by us. There are resource leakage.
//
// 2) Containerd suggests user to add entry before pulling the image. However if
// an error occurrs during the pulling, should we remove the entry from metadata
// store? Or should we leave it there until next startup (resource leakage)?
//
// 3) The cri plugin only exposes "READY" (successfully pulled and unpacked) images
// to the user, which are maintained in the in-memory metadata index. However, it's
// still possible that someone else removes the content or snapshot by-pass the cri plugin,
// how do we detect that and update the in-memory metadata correspondingly? Always
// check whether corresponding snapshot is ready when reporting image status?
//
// 4) Is the content important if we cached necessary information in-memory
// after we pull the image? How to manage the disk usage of contents? If some
// contents are missing but snapshots are ready, is the image still "READY"?

// PullImage pulls an image with authentication config.
func (c *criService) PullImage(ctx context.Context, r *runtime.PullImageRequest) (*runtime.PullImageResponse, error) {
	imageRef := r.GetImage().GetImage()
	namedRef, err := util.NormalizeImageRef(imageRef)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to parse image reference %q", imageRef)
	}
	ref := namedRef.String()
	if ref != imageRef {
		logrus.Debugf("PullImage using normalized image ref: %q", ref)
	}
	resolver, desc, err := c.getResolver(ctx, ref, c.credentials(r.GetAuth()))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to resolve image %q", ref)
	}
	// We have to check schema1 here, because after `Pull`, schema1
	// image has already been converted.
	isSchema1 := desc.MediaType == containerdimages.MediaTypeDockerSchema1Manifest

	image, err := c.client.Pull(ctx, ref,
		containerd.WithSchema1Conversion,
		containerd.WithResolver(resolver),
		containerd.WithPullSnapshotter(c.config.ContainerdConfig.Snapshotter),
		containerd.WithPullUnpack,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to pull and unpack image %q", ref)
	}

	configDesc, err := image.Config(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "get image config descriptor")
	}
	imageID := configDesc.Digest.String()

	repoDigest, repoTag := getRepoDigestAndTag(namedRef, image.Target().Digest, isSchema1)
	for _, r := range []string{imageID, repoTag, repoDigest} {
		if r == "" {
			continue
		}
		if err := c.createImageReference(ctx, r, image.Target()); err != nil {
			return nil, errors.Wrapf(err, "failed to create image reference %q", r)
		}
		// Update image store to reflect the newest state in containerd.
		// No need to use `updateImage`, because the image reference must
		// have been managed by the cri plugin.
		if err := c.imageStore.Update(ctx, r); err != nil {
			return nil, errors.Wrapf(err, "failed to update image store %q", r)
		}
	}

	logrus.Debugf("Pulled image %q with image id %q, repo tag %q, repo digest %q", imageRef, imageID,
		repoTag, repoDigest)
	// NOTE(random-liu): the actual state in containerd is the source of truth, even we maintain
	// in-memory image store, it's only for in-memory indexing. The image could be removed
	// by someone else anytime, before/during/after we create the metadata. We should always
	// check the actual state in containerd before using the image or returning status of the
	// image.
	return &runtime.PullImageResponse{ImageRef: imageID}, nil
}

// ParseAuth parses AuthConfig and returns username and password/secret required by containerd.
func ParseAuth(auth *runtime.AuthConfig) (string, string, error) {
	if auth == nil {
		return "", "", nil
	}
	if auth.Username != "" {
		return auth.Username, auth.Password, nil
	}
	if auth.IdentityToken != "" {
		return "", auth.IdentityToken, nil
	}
	if auth.Auth != "" {
		decLen := base64.StdEncoding.DecodedLen(len(auth.Auth))
		decoded := make([]byte, decLen)
		_, err := base64.StdEncoding.Decode(decoded, []byte(auth.Auth))
		if err != nil {
			return "", "", err
		}
		fields := strings.SplitN(string(decoded), ":", 2)
		if len(fields) != 2 {
			return "", "", errors.Errorf("invalid decoded auth: %q", decoded)
		}
		user, passwd := fields[0], fields[1]
		return user, strings.Trim(passwd, "\x00"), nil
	}
	// TODO(random-liu): Support RegistryToken.
	return "", "", errors.New("invalid auth config")
}

// createImageReference creates image reference inside containerd image store.
// Note that because create and update are not finished in one transaction, there could be race. E.g.
// the image reference is deleted by someone else after create returns already exists, but before update
// happens.
func (c *criService) createImageReference(ctx context.Context, name string, desc imagespec.Descriptor) error {
	img := containerdimages.Image{
		Name:   name,
		Target: desc,
		// Add a label to indicate that the image is managed by the cri plugin.
		Labels: map[string]string{imageLabelKey: imageLabelValue},
	}
	// TODO(random-liu): Figure out which is the more performant sequence create then update or
	// update then create.
	oldImg, err := c.client.ImageService().Create(ctx, img)
	if err == nil || !errdefs.IsAlreadyExists(err) {
		return err
	}
	if oldImg.Target.Digest == img.Target.Digest && oldImg.Labels[imageLabelKey] == imageLabelValue {
		return nil
	}
	_, err = c.client.ImageService().Update(ctx, img, "target", "labels")
	return err
}

// updateImage updates image store to reflect the newest state of an image reference
// in containerd. If the reference is not managed by the cri plugin, the function also
// generates necessary metadata for the image and make it managed.
func (c *criService) updateImage(ctx context.Context, r string) error {
	img, err := c.client.GetImage(ctx, r)
	if err != nil && !errdefs.IsNotFound(err) {
		return errors.Wrap(err, "get image by reference")
	}
	if err == nil && img.Labels()[imageLabelKey] != imageLabelValue {
		// Make sure the image has the image id as its unique
		// identifier that references the image in its lifetime.
		configDesc, err := img.Config(ctx)
		if err != nil {
			return errors.Wrap(err, "get image id")
		}
		id := configDesc.Digest.String()
		if err := c.createImageReference(ctx, id, img.Target()); err != nil {
			return errors.Wrapf(err, "create image id reference %q", id)
		}
		if err := c.imageStore.Update(ctx, id); err != nil {
			return errors.Wrapf(err, "update image store for %q", id)
		}
		// The image id is ready, add the label to mark the image as managed.
		if err := c.createImageReference(ctx, r, img.Target()); err != nil {
			return errors.Wrap(err, "create managed label")
		}
	}
	// If the image is not found, we should continue updating the cache,
	// so that the image can be removed from the cache.
	if err := c.imageStore.Update(ctx, r); err != nil {
		return errors.Wrapf(err, "update image store for %q", r)
	}
	return nil
}

// credentials returns a credential function for docker resolver to use.
func (c *criService) credentials(auth *runtime.AuthConfig) func(string) (string, string, error) {
	return func(host string) (string, string, error) {
		if auth == nil {
			// Get default auth from config.
			for h, ac := range c.config.Registry.Auths {
				u, err := url.Parse(h)
				if err != nil {
					return "", "", errors.Wrapf(err, "parse auth host %q", h)
				}
				if u.Host == host {
					auth = toRuntimeAuthConfig(ac)
					break
				}
			}
		}
		return ParseAuth(auth)
	}
}

// getResolver tries registry mirrors and the default registry, and returns the resolver and descriptor
// from the first working registry.
func (c *criService) getResolver(ctx context.Context, ref string, cred func(string) (string, string, error)) (remotes.Resolver, imagespec.Descriptor, error) {
	refspec, err := reference.Parse(ref)
	if err != nil {
		return nil, imagespec.Descriptor{}, errors.Wrap(err, "parse image reference")
	}
	// Try mirrors in order first, and then try default host name.
	for _, e := range c.config.Registry.Mirrors[refspec.Hostname()].Endpoints {
		u, err := url.Parse(e)
		if err != nil {
			return nil, imagespec.Descriptor{}, errors.Wrapf(err, "parse registry endpoint %q", e)
		}
		resolver := docker.NewResolver(docker.ResolverOptions{
			Authorizer: docker.NewAuthorizer(http.DefaultClient, cred),
			Client:     http.DefaultClient,
			Host:       func(string) (string, error) { return u.Host, nil },
			// By default use "https".
			PlainHTTP: u.Scheme == "http",
		})
		_, desc, err := resolver.Resolve(ctx, ref)
		if err == nil {
			return resolver, desc, nil
		}
		// Continue to try next endpoint
	}
	resolver := docker.NewResolver(docker.ResolverOptions{
		Credentials: cred,
		Client:      http.DefaultClient,
	})
	_, desc, err := resolver.Resolve(ctx, ref)
	if err != nil {
		return nil, imagespec.Descriptor{}, errors.Wrap(err, "no available registry endpoint")
	}
	return resolver, desc, nil
}
