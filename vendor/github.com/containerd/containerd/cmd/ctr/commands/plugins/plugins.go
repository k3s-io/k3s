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

package plugins

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/platforms"
	pluginutils "github.com/containerd/containerd/plugin"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
	"google.golang.org/grpc/codes"
)

// Command is a cli command that outputs plugin information
var Command = cli.Command{
	Name:    "plugins",
	Aliases: []string{"plugin"},
	Usage:   "provides information about containerd plugins",
	Subcommands: []cli.Command{
		listCommand,
	},
}

var listCommand = cli.Command{
	Name:    "list",
	Aliases: []string{"ls"},
	Usage:   "lists containerd plugins",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "quiet,q",
			Usage: "print only the plugin ids",
		},
		cli.BoolFlag{
			Name:  "detailed,d",
			Usage: "print detailed information about each plugin",
		},
	},
	Action: func(context *cli.Context) error {
		var (
			quiet    = context.Bool("quiet")
			detailed = context.Bool("detailed")
		)
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		ps := client.IntrospectionService()
		response, err := ps.Plugins(ctx, context.Args())
		if err != nil {
			return err
		}
		if quiet {
			for _, plugin := range response.Plugins {
				fmt.Println(plugin.ID)
			}
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 4, 8, 4, ' ', 0)
		if detailed {
			first := true
			for _, plugin := range response.Plugins {
				if !first {
					fmt.Fprintln(w, "\t\t\t")
				}
				first = false
				fmt.Fprintln(w, "Type:\t", plugin.Type)
				fmt.Fprintln(w, "ID:\t", plugin.ID)
				if len(plugin.Requires) > 0 {
					fmt.Fprintln(w, "Requires:\t")
					for _, r := range plugin.Requires {
						fmt.Fprintln(w, "\t", r)
					}
				}
				if len(plugin.Platforms) > 0 {
					fmt.Fprintln(w, "Platforms:\t", prettyPlatforms(plugin.Platforms))
				}

				if len(plugin.Exports) > 0 {
					fmt.Fprintln(w, "Exports:\t")
					for k, v := range plugin.Exports {
						fmt.Fprintln(w, "\t", k, "\t", v)
					}
				}

				if len(plugin.Capabilities) > 0 {
					fmt.Fprintln(w, "Capabilities:\t", strings.Join(plugin.Capabilities, ","))
				}

				if plugin.InitErr != nil {
					fmt.Fprintln(w, "Error:\t")
					fmt.Fprintln(w, "\t Code:\t", codes.Code(plugin.InitErr.Code))
					fmt.Fprintln(w, "\t Message:\t", plugin.InitErr.Message)
				}
			}
			return w.Flush()
		}

		fmt.Fprintln(w, "TYPE\tID\tPLATFORMS\tSTATUS\t")
		for _, plugin := range response.Plugins {
			status := "ok"

			if plugin.InitErr != nil {
				if strings.Contains(plugin.InitErr.Message, pluginutils.ErrSkipPlugin.Error()) {
					status = "skip"
				} else {
					status = "error"
				}
			}

			var platformColumn = "-"
			if len(plugin.Platforms) > 0 {
				platformColumn = prettyPlatforms(plugin.Platforms)
			}
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\t%s\t\n",
				plugin.Type,
				plugin.ID,
				platformColumn,
				status,
			); err != nil {
				return err
			}
		}
		return w.Flush()
	},
}

func prettyPlatforms(pspb []types.Platform) string {
	psm := map[string]struct{}{}
	for _, p := range pspb {
		psm[platforms.Format(v1.Platform{
			OS:           p.OS,
			Architecture: p.Architecture,
			Variant:      p.Variant,
		})] = struct{}{}
	}
	var ps []string
	for p := range psm {
		ps = append(ps, p)
	}
	sort.Stable(sort.StringSlice(ps))
	return strings.Join(ps, ",")
}
