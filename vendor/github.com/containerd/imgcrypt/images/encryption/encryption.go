/*
   Copyright The containerd Authors.

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

package encryption

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"

	"github.com/containerd/containerd/images"
	"github.com/containers/ocicrypt"
	encconfig "github.com/containers/ocicrypt/config"

	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/platforms"
	encocispec "github.com/containers/ocicrypt/spec"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	"github.com/pkg/errors"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type cryptoOp int

const (
	cryptoOpEncrypt    cryptoOp = iota
	cryptoOpDecrypt             = iota
	cryptoOpUnwrapOnly          = iota
)

// LayerFilter allows to select Layers by certain criteria
type LayerFilter func(desc ocispec.Descriptor) bool

// IsEncryptedDiff returns true if mediaType is a known encrypted media type.
func IsEncryptedDiff(ctx context.Context, mediaType string) bool {
	switch mediaType {
	case encocispec.MediaTypeLayerGzipEnc, encocispec.MediaTypeLayerEnc:
		return true
	}
	return false
}

// HasEncryptedLayer returns true if any LayerInfo indicates that the layer is encrypted
func HasEncryptedLayer(ctx context.Context, layerInfos []ocispec.Descriptor) bool {
	for i := 0; i < len(layerInfos); i++ {
		if IsEncryptedDiff(ctx, layerInfos[i].MediaType) {
			return true
		}
	}
	return false
}

// encryptLayer encrypts the layer using the CryptoConfig and creates a new OCI Descriptor.
// A call to this function may also only manipulate the wrapped keys list.
// The caller is expected to store the returned encrypted data and OCI Descriptor
func encryptLayer(cc *encconfig.CryptoConfig, dataReader content.ReaderAt, desc ocispec.Descriptor) (ocispec.Descriptor, io.Reader, ocicrypt.EncryptLayerFinalizer, error) {
	var (
		size int64
		d    digest.Digest
		err  error
	)

	encLayerReader, encLayerFinalizer, err := ocicrypt.EncryptLayer(cc.EncryptConfig, ocicrypt.ReaderFromReaderAt(dataReader), desc)
	if err != nil {
		return ocispec.Descriptor{}, nil, nil, err
	}

	// were data touched ?
	if encLayerReader != nil {
		size = 0
		d = ""
	} else {
		size = desc.Size
		d = desc.Digest
	}

	newDesc := ocispec.Descriptor{
		Digest:   d,
		Size:     size,
		Platform: desc.Platform,
	}

	switch desc.MediaType {
	case images.MediaTypeDockerSchema2LayerGzip:
		newDesc.MediaType = encocispec.MediaTypeLayerGzipEnc
	case images.MediaTypeDockerSchema2Layer:
		newDesc.MediaType = encocispec.MediaTypeLayerEnc
	case encocispec.MediaTypeLayerGzipEnc:
		newDesc.MediaType = encocispec.MediaTypeLayerGzipEnc
	case encocispec.MediaTypeLayerEnc:
		newDesc.MediaType = encocispec.MediaTypeLayerEnc

	// TODO: Mediatypes to be added in ocispec
	case ocispec.MediaTypeImageLayerGzip:
		newDesc.MediaType = encocispec.MediaTypeLayerGzipEnc
	case ocispec.MediaTypeImageLayer:
		newDesc.MediaType = encocispec.MediaTypeLayerEnc

	default:
		return ocispec.Descriptor{}, nil, nil, errors.Errorf("Encryption: unsupporter layer MediaType: %s\n", desc.MediaType)
	}

	return newDesc, encLayerReader, encLayerFinalizer, nil
}

// DecryptLayer decrypts the layer using the DecryptConfig and creates a new OCI Descriptor.
// The caller is expected to store the returned plain data and OCI Descriptor
func DecryptLayer(dc *encconfig.DecryptConfig, dataReader io.Reader, desc ocispec.Descriptor, unwrapOnly bool) (ocispec.Descriptor, io.Reader, digest.Digest, error) {
	resultReader, layerDigest, err := ocicrypt.DecryptLayer(dc, dataReader, desc, unwrapOnly)
	if err != nil || unwrapOnly {
		return ocispec.Descriptor{}, nil, "", err
	}

	newDesc := ocispec.Descriptor{
		Size:     0,
		Platform: desc.Platform,
	}

	switch desc.MediaType {
	case encocispec.MediaTypeLayerGzipEnc:
		newDesc.MediaType = images.MediaTypeDockerSchema2LayerGzip
	case encocispec.MediaTypeLayerEnc:
		newDesc.MediaType = images.MediaTypeDockerSchema2Layer
	default:
		return ocispec.Descriptor{}, nil, "", errors.Errorf("Decryption: unsupporter layer MediaType: %s\n", desc.MediaType)
	}
	return newDesc, resultReader, layerDigest, nil
}

// decryptLayer decrypts the layer using the CryptoConfig and creates a new OCI Descriptor.
// The caller is expected to store the returned plain data and OCI Descriptor
func decryptLayer(cc *encconfig.CryptoConfig, dataReader content.ReaderAt, desc ocispec.Descriptor, unwrapOnly bool) (ocispec.Descriptor, io.Reader, error) {
	resultReader, d, err := ocicrypt.DecryptLayer(cc.DecryptConfig, ocicrypt.ReaderFromReaderAt(dataReader), desc, unwrapOnly)
	if err != nil || unwrapOnly {
		return ocispec.Descriptor{}, nil, err
	}

	newDesc := ocispec.Descriptor{
		Digest:   d,
		Size:     0,
		Platform: desc.Platform,
	}

	switch desc.MediaType {
	case encocispec.MediaTypeLayerGzipEnc:
		newDesc.MediaType = images.MediaTypeDockerSchema2LayerGzip
	case encocispec.MediaTypeLayerEnc:
		newDesc.MediaType = images.MediaTypeDockerSchema2Layer
	default:
		return ocispec.Descriptor{}, nil, errors.Errorf("Decryption: unsupporter layer MediaType: %s\n", desc.MediaType)
	}
	return newDesc, resultReader, nil
}

// cryptLayer handles the changes due to encryption or decryption of a layer
func cryptLayer(ctx context.Context, cs content.Store, desc ocispec.Descriptor, cc *encconfig.CryptoConfig, cryptoOp cryptoOp) (ocispec.Descriptor, error) {
	var (
		resultReader      io.Reader
		newDesc           ocispec.Descriptor
		encLayerFinalizer ocicrypt.EncryptLayerFinalizer
	)

	dataReader, err := cs.ReaderAt(ctx, desc)
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	defer dataReader.Close()

	if cryptoOp == cryptoOpEncrypt {
		newDesc, resultReader, encLayerFinalizer, err = encryptLayer(cc, dataReader, desc)
	} else {
		newDesc, resultReader, err = decryptLayer(cc, dataReader, desc, cryptoOp == cryptoOpUnwrapOnly)
	}
	if err != nil || cryptoOp == cryptoOpUnwrapOnly {
		return ocispec.Descriptor{}, err
	}

	newDesc.Annotations = ocicrypt.FilterOutAnnotations(desc.Annotations)

	// some operations, such as changing recipients, may not touch the layer at all
	if resultReader != nil {
		var ref string
		// If we have the digest, write blob with checks
		haveDigest := newDesc.Digest.String() != ""
		if haveDigest {
			ref = fmt.Sprintf("layer-%s", newDesc.Digest.String())
		} else {
			ref = fmt.Sprintf("blob-%d-%d", rand.Int(), rand.Int())
		}

		if haveDigest {
			if err := content.WriteBlob(ctx, cs, ref, resultReader, newDesc); err != nil {
				return ocispec.Descriptor{}, errors.Wrap(err, "failed to write config")
			}
		} else {
			newDesc.Digest, newDesc.Size, err = ingestReader(ctx, cs, ref, resultReader)
			if err != nil {
				return ocispec.Descriptor{}, err
			}
		}
	}

	// After performing encryption, call finalizer to get annotations
	if encLayerFinalizer != nil {
		annotations, err := encLayerFinalizer()
		if err != nil {
			return ocispec.Descriptor{}, errors.Wrap(err, "Error getting annotations from encLayer finalizer")
		}
		for k, v := range annotations {
			newDesc.Annotations[k] = v
		}
	}
	return newDesc, err
}

func ingestReader(ctx context.Context, cs content.Ingester, ref string, r io.Reader) (digest.Digest, int64, error) {
	cw, err := content.OpenWriter(ctx, cs, content.WithRef(ref))
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to open writer")
	}
	defer cw.Close()

	if _, err := content.CopyReader(cw, r); err != nil {
		return "", 0, errors.Wrap(err, "copy failed")
	}

	st, err := cw.Status()
	if err != nil {
		return "", 0, errors.Wrap(err, "failed to get state")
	}

	if err := cw.Commit(ctx, st.Offset, ""); err != nil {
		if !errdefs.IsAlreadyExists(err) {
			return "", 0, errors.Wrapf(err, "failed commit on ref %q", ref)
		}
	}

	return cw.Digest(), st.Offset, nil
}

// Encrypt or decrypt all the Children of a given descriptor
func cryptChildren(ctx context.Context, cs content.Store, desc ocispec.Descriptor, cc *encconfig.CryptoConfig, lf LayerFilter, cryptoOp cryptoOp, thisPlatform *ocispec.Platform) (ocispec.Descriptor, bool, error) {
	children, err := images.Children(ctx, cs, desc)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return desc, false, nil
		}
		return ocispec.Descriptor{}, false, err
	}

	var newLayers []ocispec.Descriptor
	var config ocispec.Descriptor
	modified := false

	for _, child := range children {
		// we only encrypt child layers and have to update their parents if encryption happened
		switch child.MediaType {
		case images.MediaTypeDockerSchema2Config, ocispec.MediaTypeImageConfig:
			config = child
		case images.MediaTypeDockerSchema2LayerGzip, images.MediaTypeDockerSchema2Layer,
			ocispec.MediaTypeImageLayerGzip, ocispec.MediaTypeImageLayer:
			if cryptoOp == cryptoOpEncrypt && lf(child) {
				nl, err := cryptLayer(ctx, cs, child, cc, cryptoOp)
				if err != nil {
					return ocispec.Descriptor{}, false, err
				}
				modified = true
				newLayers = append(newLayers, nl)
			} else {
				newLayers = append(newLayers, child)
			}
		case encocispec.MediaTypeLayerGzipEnc, encocispec.MediaTypeLayerEnc:
			// this one can be decrypted but also its recipients list changed
			if lf(child) {
				nl, err := cryptLayer(ctx, cs, child, cc, cryptoOp)
				if err != nil || cryptoOp == cryptoOpUnwrapOnly {
					return ocispec.Descriptor{}, false, err
				}
				modified = true
				newLayers = append(newLayers, nl)
			} else {
				newLayers = append(newLayers, child)
			}
		case images.MediaTypeDockerSchema2LayerForeign, images.MediaTypeDockerSchema2LayerForeignGzip:
			// never encrypt/decrypt
			newLayers = append(newLayers, child)
		default:
			return ocispec.Descriptor{}, false, errors.Errorf("bad/unhandled MediaType %s in encryptChildren\n", child.MediaType)
		}
	}

	if modified && len(newLayers) > 0 {
		newManifest := ocispec.Manifest{
			Versioned: specs.Versioned{
				SchemaVersion: 2,
			},
			Config: config,
			Layers: newLayers,
		}

		mb, err := json.MarshalIndent(newManifest, "", "   ")
		if err != nil {
			return ocispec.Descriptor{}, false, errors.Wrap(err, "failed to marshal image")
		}

		newDesc := ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageManifest,
			Size:      int64(len(mb)),
			Digest:    digest.Canonical.FromBytes(mb),
			Platform:  desc.Platform,
		}

		labels := map[string]string{}
		labels["containerd.io/gc.ref.content.0"] = newManifest.Config.Digest.String()
		for i, ch := range newManifest.Layers {
			labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i+1)] = ch.Digest.String()
		}

		ref := fmt.Sprintf("manifest-%s", newDesc.Digest.String())

		if err := content.WriteBlob(ctx, cs, ref, bytes.NewReader(mb), newDesc, content.WithLabels(labels)); err != nil {
			return ocispec.Descriptor{}, false, errors.Wrap(err, "failed to write config")
		}
		return newDesc, true, nil
	}

	return desc, modified, nil
}

// cryptManifest encrypts or decrypts the children of a top level manifest
func cryptManifest(ctx context.Context, cs content.Store, desc ocispec.Descriptor, cc *encconfig.CryptoConfig, lf LayerFilter, cryptoOp cryptoOp) (ocispec.Descriptor, bool, error) {
	p, err := content.ReadBlob(ctx, cs, desc)
	if err != nil {
		return ocispec.Descriptor{}, false, err
	}
	var manifest ocispec.Manifest
	if err := json.Unmarshal(p, &manifest); err != nil {
		return ocispec.Descriptor{}, false, err
	}
	platform := platforms.DefaultSpec()
	newDesc, modified, err := cryptChildren(ctx, cs, desc, cc, lf, cryptoOp, &platform)
	if err != nil || cryptoOp == cryptoOpUnwrapOnly {
		return ocispec.Descriptor{}, false, err
	}
	return newDesc, modified, nil
}

// cryptManifestList encrypts or decrypts the children of a top level manifest list
func cryptManifestList(ctx context.Context, cs content.Store, desc ocispec.Descriptor, cc *encconfig.CryptoConfig, lf LayerFilter, cryptoOp cryptoOp) (ocispec.Descriptor, bool, error) {
	// read the index; if any layer is encrypted and any manifests change we will need to rewrite it
	b, err := content.ReadBlob(ctx, cs, desc)
	if err != nil {
		return ocispec.Descriptor{}, false, err
	}

	var index ocispec.Index
	if err := json.Unmarshal(b, &index); err != nil {
		return ocispec.Descriptor{}, false, err
	}

	var newManifests []ocispec.Descriptor
	modified := false
	for _, manifest := range index.Manifests {
		newManifest, m, err := cryptChildren(ctx, cs, manifest, cc, lf, cryptoOp, manifest.Platform)
		if err != nil || cryptoOp == cryptoOpUnwrapOnly {
			return ocispec.Descriptor{}, false, err
		}
		if m {
			modified = true
		}
		newManifests = append(newManifests, newManifest)
	}

	if modified {
		// we need to update the index
		newIndex := ocispec.Index{
			Versioned: index.Versioned,
			Manifests: newManifests,
		}

		mb, err := json.MarshalIndent(newIndex, "", "   ")
		if err != nil {
			return ocispec.Descriptor{}, false, errors.Wrap(err, "failed to marshal index")
		}

		newDesc := ocispec.Descriptor{
			MediaType: ocispec.MediaTypeImageIndex,
			Size:      int64(len(mb)),
			Digest:    digest.Canonical.FromBytes(mb),
		}

		labels := map[string]string{}
		for i, m := range newIndex.Manifests {
			labels[fmt.Sprintf("containerd.io/gc.ref.content.%d", i)] = m.Digest.String()
		}

		ref := fmt.Sprintf("index-%s", newDesc.Digest.String())

		if err = content.WriteBlob(ctx, cs, ref, bytes.NewReader(mb), newDesc, content.WithLabels(labels)); err != nil {
			return ocispec.Descriptor{}, false, errors.Wrap(err, "failed to write index")
		}
		return newDesc, true, nil
	}

	return desc, false, nil
}

// cryptImage is the dispatcher to encrypt/decrypt an image; it accepts either an OCI descriptor
// representing a manifest list or a single manifest
func cryptImage(ctx context.Context, cs content.Store, desc ocispec.Descriptor, cc *encconfig.CryptoConfig, lf LayerFilter, cryptoOp cryptoOp) (ocispec.Descriptor, bool, error) {
	if cc == nil {
		return ocispec.Descriptor{}, false, errors.Wrapf(errdefs.ErrInvalidArgument, "CryptoConfig must not be nil")
	}
	switch desc.MediaType {
	case ocispec.MediaTypeImageIndex, images.MediaTypeDockerSchema2ManifestList:
		return cryptManifestList(ctx, cs, desc, cc, lf, cryptoOp)
	case ocispec.MediaTypeImageManifest, images.MediaTypeDockerSchema2Manifest:
		return cryptManifest(ctx, cs, desc, cc, lf, cryptoOp)
	default:
		return ocispec.Descriptor{}, false, errors.Errorf("CryptImage: Unhandled media type: %s", desc.MediaType)
	}
}

// EncryptImage encrypts an image; it accepts either an OCI descriptor representing a manifest list or a single manifest
func EncryptImage(ctx context.Context, cs content.Store, desc ocispec.Descriptor, cc *encconfig.CryptoConfig, lf LayerFilter) (ocispec.Descriptor, bool, error) {
	return cryptImage(ctx, cs, desc, cc, lf, cryptoOpEncrypt)
}

// DecryptImage decrypts an image; it accepts either an OCI descriptor representing a manifest list or a single manifest
func DecryptImage(ctx context.Context, cs content.Store, desc ocispec.Descriptor, cc *encconfig.CryptoConfig, lf LayerFilter) (ocispec.Descriptor, bool, error) {
	return cryptImage(ctx, cs, desc, cc, lf, cryptoOpDecrypt)
}

// CheckAuthorization checks whether a user has the right keys to be allowed to access an image (every layer)
// It takes decrypting of the layers only as far as decrypting the asymmetrically encrypted data
// The decryption is only done for the current platform
func CheckAuthorization(ctx context.Context, cs content.Store, desc ocispec.Descriptor, dc *encconfig.DecryptConfig) error {
	cc := encconfig.InitDecryption(dc.Parameters)

	lf := func(desc ocispec.Descriptor) bool {
		return true
	}

	_, _, err := cryptImage(ctx, cs, desc, &cc, lf, cryptoOpUnwrapOnly)
	if err != nil {
		return errors.Wrapf(err, "you are not authorized to use this image")
	}
	return nil
}
