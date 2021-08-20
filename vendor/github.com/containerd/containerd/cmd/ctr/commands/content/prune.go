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

package content

import (
	"strings"
	"time"
	"unicode"

	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/leases"
	"github.com/containerd/containerd/log"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

const (
	layerPrefix   = "containerd.io/gc.ref.content.l."
	contentPrefix = "containerd.io/gc.ref.content."
)

var pruneFlags = []cli.Flag{
	cli.BoolFlag{
		Name:  "async",
		Usage: "allow garbage collection to cleanup asynchronously",
	},
	cli.BoolFlag{
		Name:  "dry",
		Usage: "just show updates without applying (enables debug logging)",
	},
}

var pruneCommand = cli.Command{
	Name:  "prune",
	Usage: "prunes content from the content store",
	Subcommands: cli.Commands{
		pruneReferencesCommand,
	},
}

var pruneReferencesCommand = cli.Command{
	Name:  "references",
	Usage: "prunes preference labels from the content store (layers only by default)",
	Flags: pruneFlags,
	Action: func(clicontext *cli.Context) error {
		client, ctx, cancel, err := commands.NewClient(clicontext)
		if err != nil {
			return err
		}
		defer cancel()

		dryRun := clicontext.Bool("dry")
		if dryRun {
			log.G(ctx).Logger.SetLevel(logrus.DebugLevel)
			log.G(ctx).Debug("dry run, no changes will be applied")
		}

		var deleteOpts []leases.DeleteOpt
		if !clicontext.Bool("async") {
			deleteOpts = append(deleteOpts, leases.SynchronousDelete)
		}

		cs := client.ContentStore()
		if err := cs.Walk(ctx, func(info content.Info) error {
			var fields []string

			for k := range info.Labels {
				if isLayerLabel(k) {
					log.G(ctx).WithFields(logrus.Fields{
						"digest": info.Digest,
						"label":  k,
					}).Debug("Removing label")
					if dryRun {
						continue
					}
					fields = append(fields, "labels."+k)
					delete(info.Labels, k)
				}
			}

			if len(fields) == 0 {
				return nil
			}

			_, err := cs.Update(ctx, info, fields...)
			return err
		}); err != nil {
			return err
		}

		ls := client.LeasesService()
		l, err := ls.Create(ctx, leases.WithRandomID(), leases.WithExpiration(time.Hour))
		if err != nil {
			return err
		}
		return ls.Delete(ctx, l, deleteOpts...)
	},
}

func isLayerLabel(key string) bool {
	if strings.HasPrefix(key, layerPrefix) {
		return true
	}
	if !strings.HasPrefix(key, contentPrefix) {
		return false
	}

	// handle legacy labels which used content prefix and index (0 always for config)
	key = key[len(contentPrefix):]
	if isInteger(key) && key != "0" {
		return true
	}

	return false
}

func isInteger(key string) bool {
	for _, r := range key {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
