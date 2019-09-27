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
	"fmt"

	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/errdefs"
	"github.com/urfave/cli"
)

var tagCommand = cli.Command{
	Name:        "tag",
	Usage:       "tag an image",
	ArgsUsage:   "[flags] <source_ref> <target_ref> [<target_ref>, ...]",
	Description: `Tag an image for use in containerd.`,
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "force",
			Usage: "force target_ref to be created, regardless if it already exists",
		},
	},
	Action: func(context *cli.Context) error {
		var (
			ref = context.Args().First()
		)
		if ref == "" {
			return fmt.Errorf("please provide an image reference to tag from")
		}
		if context.NArg() <= 1 {
			return fmt.Errorf("please provide an image reference to tag to")
		}

		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()

		ctx, done, err := client.WithLease(ctx)
		if err != nil {
			return err
		}
		defer done(ctx)

		imageService := client.ImageService()
		image, err := imageService.Get(ctx, ref)
		if err != nil {
			return err
		}
		// Support multiple references for one command run
		for _, targetRef := range context.Args()[1:] {
			image.Name = targetRef
			// Attempt to create the image first
			if _, err = imageService.Create(ctx, image); err != nil {
				// If user has specified force and the image already exists then
				// delete the original image and attempt to create the new one
				if errdefs.IsAlreadyExists(err) && context.Bool("force") {
					if err = imageService.Delete(ctx, targetRef); err != nil {
						return err
					}
					if _, err = imageService.Create(ctx, image); err != nil {
						return err
					}
				} else {
					return err
				}
			}
			fmt.Println(targetRef)
		}
		return nil
	},
}
