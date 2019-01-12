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

package importer

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/log"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"

	ctrdutil "github.com/containerd/cri/pkg/containerd/util"
	"github.com/containerd/cri/pkg/util"
)

// This code reuses the docker import code from containerd/containerd#1602.
// It has been simplified a bit and garbage collection support was added.
// If a library/helper is added to containerd in the future, we should switch to it.

// manifestDotJSON is an entry in manifest.json.
type manifestDotJSON struct {
	Config   string
	RepoTags []string
	Layers   []string
	// Parent is unsupported
	Parent string
}

// isLayerTar returns true if name is like "foobar/layer.tar"
func isLayerTar(name string) bool {
	slashes := len(strings.Split(name, "/"))
	return slashes == 2 && strings.HasSuffix(name, "/layer.tar")
}

// followSymlinkLayer returns actual layer name of the symlink layer.
// It returns "foobar/layer.tar" if the name is like
// "../foobar/layer.tar", and returns error if the name
// is not in "../foobar/layer.tar" format.
func followSymlinkLayer(name string) (string, error) {
	parts := strings.Split(name, "/")
	if len(parts) != 3 || parts[0] != ".." {
		return "", errors.New("invalid symlink layer")
	}
	name = strings.TrimPrefix(name, "../")
	if !isLayerTar(name) {
		return "", errors.New("invalid layer tar")
	}
	return name, nil
}

// isDotJSON returns true if name is like "foobar.json"
func isDotJSON(name string) bool {
	slashes := len(strings.Split(name, "/"))
	return slashes == 1 && strings.HasSuffix(name, ".json")
}

type imageConfig struct {
	desc ocispec.Descriptor
	img  ocispec.Image
}

type importConfig struct {
	unpack      bool
	snapshotter string
}

// ImportOption configures import behavior.
type ImportOption func(*importConfig)

// WithUnpack is used to unpack image after import.
func WithUnpack(snapshotter string) ImportOption {
	return func(c *importConfig) {
		c.unpack = true
		c.snapshotter = snapshotter
	}
}

// Import implements Docker Image Spec v1.1.
// An image MUST have `manifest.json`.
// `repositories` file in Docker Image Spec v1.0 is not supported (yet).
// Also, the current implementation assumes the implicit file name convention,
// which is not explicitly documented in the spec. (e.g. foobar/layer.tar)
// It returns a group of image references successfully loaded.
func Import(ctx context.Context, client *containerd.Client, reader io.Reader, opts ...ImportOption) (_ []string, retErr error) {
	c := &importConfig{}
	for _, o := range opts {
		o(c)
	}
	ctx, done, err := client.WithLease(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		deferCtx, deferCancel := ctrdutil.DeferContext()
		defer deferCancel()
		if err := done(deferCtx); err != nil {
			// Get lease id from context still works after context is done.
			leaseID, _ := leases.FromContext(ctx)
			log.G(ctx).WithError(err).Errorf("Failed to release lease %q", leaseID)
		}
	}()

	cs := client.ContentStore()
	is := client.ImageService()

	tr := tar.NewReader(reader)
	var (
		mfsts         []manifestDotJSON
		symlinkLayers = make(map[string]string)             // key: filename (foobar/layer.tar), value: linkname (targetlayerid/layer.tar)
		layers        = make(map[string]ocispec.Descriptor) // key: filename (foobar/layer.tar)
		configs       = make(map[string]imageConfig)        // key: filename (foobar.json)
	)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "get next file")
		}
		if hdr.Typeflag == tar.TypeSymlink && isLayerTar(hdr.Name) {
			linkname, err := followSymlinkLayer(hdr.Linkname)
			if err != nil {
				return nil, errors.Wrapf(err, "follow symlink layer from %q to %q", hdr.Name, hdr.Linkname)
			}
			symlinkLayers[hdr.Name] = linkname
			continue
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		if hdr.Name == "manifest.json" {
			mfsts, err = onUntarManifestJSON(tr)
			if err != nil {
				return nil, errors.Wrapf(err, "untar manifest %q", hdr.Name)
			}
			continue
		}
		if isLayerTar(hdr.Name) {
			desc, err := onUntarLayerTar(ctx, tr, cs, hdr.Name, hdr.Size)
			if err != nil {
				return nil, errors.Wrapf(err, "untar layer %q", hdr.Name)
			}
			layers[hdr.Name] = *desc
			continue
		}
		if isDotJSON(hdr.Name) {
			c, err := onUntarDotJSON(ctx, tr, cs, hdr.Name, hdr.Size)
			if err != nil {
				return nil, errors.Wrapf(err, "untar config %q", hdr.Name)
			}
			configs[hdr.Name] = *c
			continue
		}
	}
	for name, linkname := range symlinkLayers {
		desc, ok := layers[linkname]
		if !ok {
			return nil, errors.Errorf("no target for symlink layer from %q to %q", name, linkname)
		}
		layers[name] = desc
	}
	var refs []string
	defer func() {
		if retErr == nil {
			return
		}
		// TODO(random-liu): Consider whether we should keep images already imported
		// even when there is an error.
		for _, ref := range refs {
			func() {
				deferCtx, deferCancel := ctrdutil.DeferContext()
				defer deferCancel()
				if err := is.Delete(deferCtx, ref); err != nil {
					log.G(ctx).WithError(err).Errorf("Failed to remove image %q", ref)
				}
			}()
		}
	}()
	for _, mfst := range mfsts {
		config, ok := configs[mfst.Config]
		if !ok {
			return refs, errors.Errorf("image config %q not found", mfst.Config)
		}
		schema2Manifest, err := makeDockerSchema2Manifest(mfst, config, layers)
		if err != nil {
			return refs, errors.Wrap(err, "create docker manifest")
		}
		desc, err := writeDockerSchema2Manifest(ctx, cs, *schema2Manifest, config.img.Architecture, config.img.OS)
		if err != nil {
			return refs, errors.Wrap(err, "write docker manifest")
		}

		for _, ref := range mfst.RepoTags {
			normalized, err := util.NormalizeImageRef(ref)
			if err != nil {
				return refs, errors.Wrapf(err, "normalize image ref %q", ref)
			}
			ref = normalized.String()
			imgrec := images.Image{
				Name:   ref,
				Target: *desc,
			}
			if c.unpack {
				img := containerd.NewImage(client, imgrec)
				if err := img.Unpack(ctx, c.snapshotter); err != nil {
					return refs, errors.Wrapf(err, "unpack image %q", ref)
				}
			}
			if _, err := is.Create(ctx, imgrec); err != nil {
				if !errdefs.IsAlreadyExists(err) {
					return refs, errors.Wrapf(err, "create image ref %+v", imgrec)
				}

				_, err := is.Update(ctx, imgrec)
				if err != nil {
					return refs, errors.Wrapf(err, "update image ref %+v", imgrec)
				}
			}
			refs = append(refs, ref)
		}
	}
	return refs, nil
}

func makeDockerSchema2Manifest(mfst manifestDotJSON, config imageConfig, layers map[string]ocispec.Descriptor) (*ocispec.Manifest, error) {
	manifest := ocispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		Config: config.desc,
	}
	for _, f := range mfst.Layers {
		desc, ok := layers[f]
		if !ok {
			return nil, errors.Errorf("layer %q not found", f)
		}
		manifest.Layers = append(manifest.Layers, desc)
	}
	return &manifest, nil
}

func writeDockerSchema2Manifest(ctx context.Context, cs content.Ingester, manifest ocispec.Manifest, arch, os string) (*ocispec.Descriptor, error) {
	manifestBytes, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	manifestBytesR := bytes.NewReader(manifestBytes)
	manifestDigest := digest.FromBytes(manifestBytes)
	labels := map[string]string{}
	labels["containerd.io/gc.ref.content.0"] = manifest.Config.Digest.String()
	for i, ch := range manifest.Layers {
		labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i+1)] = ch.Digest.String()
	}

	desc := ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2Manifest,
		Digest:    manifestDigest,
		Size:      int64(len(manifestBytes)),
	}
	if err := content.WriteBlob(ctx, cs, "manifest-"+manifestDigest.String(), manifestBytesR,
		desc, content.WithLabels(labels)); err != nil {
		return nil, err
	}

	if arch != "" || os != "" {
		desc.Platform = &ocispec.Platform{
			Architecture: arch,
			OS:           os,
		}
	}
	return &desc, nil
}

func onUntarManifestJSON(r io.Reader) ([]manifestDotJSON, error) {
	// name: "manifest.json"
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var mfsts []manifestDotJSON
	if err := json.Unmarshal(b, &mfsts); err != nil {
		return nil, err
	}
	return mfsts, nil
}

func onUntarLayerTar(ctx context.Context, r io.Reader, cs content.Ingester, name string, size int64) (*ocispec.Descriptor, error) {
	// name is like "foobar/layer.tar" ( guaranteed by isLayerTar() )
	split := strings.Split(name, "/")
	// note: split[0] is not expected digest here
	cw, err := cs.Writer(ctx, content.WithRef("layer-"+split[0]), content.WithDescriptor(ocispec.Descriptor{Size: size}))
	if err != nil {
		return nil, err
	}
	defer cw.Close()
	if err := content.Copy(ctx, cw, r, size, ""); err != nil {
		return nil, err
	}
	return &ocispec.Descriptor{
		MediaType: images.MediaTypeDockerSchema2Layer,
		Size:      size,
		Digest:    cw.Digest(),
	}, nil
}

func onUntarDotJSON(ctx context.Context, r io.Reader, cs content.Ingester, name string, size int64) (*imageConfig, error) {
	config := imageConfig{}
	config.desc.MediaType = images.MediaTypeDockerSchema2Config
	config.desc.Size = size
	// name is like "foobar.json" ( guaranteed by is DotJSON() )
	split := strings.Split(name, ".")
	cw, err := cs.Writer(ctx, content.WithRef("config-"+split[0]), content.WithDescriptor(ocispec.Descriptor{Size: size}))
	if err != nil {
		return nil, err
	}
	defer cw.Close()
	var buf bytes.Buffer
	tr := io.TeeReader(r, &buf)
	if err := content.Copy(ctx, cw, tr, size, ""); err != nil {
		return nil, err
	}
	config.desc.Digest = cw.Digest()
	if err := json.Unmarshal(buf.Bytes(), &config.img); err != nil {
		return nil, err
	}
	return &config, nil
}
