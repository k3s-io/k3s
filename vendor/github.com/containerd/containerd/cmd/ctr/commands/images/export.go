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
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var exportCommand = cli.Command{
	Name:      "export",
	Usage:     "export images",
	ArgsUsage: "[flags] <out> <image> ...",
	Description: `Export images to an OCI tar archive.

Tar output is formatted as an OCI archive, a Docker manifest is provided for the platform.
Use '--skip-manifest-json' to avoid including the Docker manifest.json file.
Use '--platform' to define the output platform.
When '--all-platforms' is given all images in a manifest list must be available.
`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "skip-manifest-json",
			Usage: "do not add Docker compatible manifest.json to archive",
		},
		cli.BoolFlag{
			Name:  "skip-non-distributable",
			Usage: "do not add non-distributable blobs such as Windows layers to archive",
		},
		cli.StringSliceFlag{
			Name:  "platform",
			Usage: "Pull content from a specific platform",
			Value: &cli.StringSlice{},
		},
		cli.BoolFlag{
			Name:  "all-platforms",
			Usage: "exports content from all platforms",
		},
	},
	Action: func(context *cli.Context) error {
		var (
			out        = context.Args().First()
			images     = context.Args().Tail()
			exportOpts = []archive.ExportOpt{}
		)
		if out == "" || len(images) == 0 {
			return errors.New("please provide both an output filename and an image reference to export")
		}

		if pss := context.StringSlice("platform"); len(pss) > 0 {
			var all []ocispec.Platform
			for _, ps := range pss {
				p, err := platforms.Parse(ps)
				if err != nil {
					return errors.Wrapf(err, "invalid platform %q", ps)
				}
				all = append(all, p)
			}
			exportOpts = append(exportOpts, archive.WithPlatform(platforms.Ordered(all...)))
		} else {
			exportOpts = append(exportOpts, archive.WithPlatform(platforms.Default()))
		}

		if context.Bool("all-platforms") {
			exportOpts = append(exportOpts, archive.WithAllPlatforms())
		}

		if context.Bool("skip-manifest-json") {
			exportOpts = append(exportOpts, archive.WithSkipDockerManifest())
		}

		if context.Bool("skip-non-distributable") {
			exportOpts = append(exportOpts, archive.WithSkipNonDistributableBlobs())
		}

		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()

		is := client.ImageService()
		for _, img := range images {
			exportOpts = append(exportOpts, archive.WithImage(is, img))
		}

		var w io.WriteCloser
		if out == "-" {
			w = os.Stdout
		} else {
			w, err = os.Create(out)
			if err != nil {
				return err
			}
		}
		defer w.Close()

		return client.Export(ctx, w, exportOpts...)
	},
}
