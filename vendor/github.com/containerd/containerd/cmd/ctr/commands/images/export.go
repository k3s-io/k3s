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

package images

import (
	"io"
	"os"

	"github.com/containerd/containerd/cmd/ctr/commands"
	oci "github.com/containerd/containerd/images/oci"
	"github.com/containerd/containerd/reference"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var exportCommand = cli.Command{
	Name:      "export",
	Usage:     "export an image",
	ArgsUsage: "[flags] <out> <image>",
	Description: `Export an image to a tar stream.
Currently, only OCI format is supported.
`,
	Flags: []cli.Flag{
		// TODO(AkihiroSuda): make this map[string]string as in moby/moby#33355?
		cli.StringFlag{
			Name:  "oci-ref-name",
			Value: "",
			Usage: "override org.opencontainers.image.ref.name annotation",
		},
		cli.StringFlag{
			Name:  "manifest",
			Usage: "digest of manifest",
		},
		cli.StringFlag{
			Name:  "manifest-type",
			Usage: "media type of manifest digest",
			Value: ocispec.MediaTypeImageManifest,
		},
	},
	Action: func(context *cli.Context) error {
		var (
			out   = context.Args().First()
			local = context.Args().Get(1)
			desc  ocispec.Descriptor
		)
		if out == "" || local == "" {
			return errors.New("please provide both an output filename and an image reference to export")
		}
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		if manifest := context.String("manifest"); manifest != "" {
			desc.Digest, err = digest.Parse(manifest)
			if err != nil {
				return errors.Wrap(err, "invalid manifest digest")
			}
			desc.MediaType = context.String("manifest-type")
		} else {
			img, err := client.ImageService().Get(ctx, local)
			if err != nil {
				return errors.Wrap(err, "unable to resolve image to manifest")
			}
			desc = img.Target
		}

		if desc.Annotations == nil {
			desc.Annotations = make(map[string]string)
		}
		if s, ok := desc.Annotations[ocispec.AnnotationRefName]; !ok || s == "" {
			if ociRefName := determineOCIRefName(local); ociRefName != "" {
				desc.Annotations[ocispec.AnnotationRefName] = ociRefName
			}
			if ociRefName := context.String("oci-ref-name"); ociRefName != "" {
				desc.Annotations[ocispec.AnnotationRefName] = ociRefName
			}
		}
		var w io.WriteCloser
		if out == "-" {
			w = os.Stdout
		} else {
			w, err = os.Create(out)
			if err != nil {
				return nil
			}
		}
		r, err := client.Export(ctx, &oci.V1Exporter{}, desc)
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, r); err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
		return r.Close()
	},
}

func determineOCIRefName(local string) string {
	refspec, err := reference.Parse(local)
	if err != nil {
		return ""
	}
	tag, _ := reference.SplitObject(refspec.Object)
	return tag
}
