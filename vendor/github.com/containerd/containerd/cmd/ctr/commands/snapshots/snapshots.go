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

package snapshots

import (
	gocontext "context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/containerd/containerd/cmd/ctr/commands"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/diff"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/mount"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/containerd/rootfs"
	"github.com/containerd/containerd/snapshots"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// Command is the cli command for managing snapshots
var Command = cli.Command{
	Name:    "snapshots",
	Aliases: []string{"snapshot"},
	Usage:   "manage snapshots",
	Flags:   commands.SnapshotterFlags,
	Subcommands: cli.Commands{
		commitCommand,
		diffCommand,
		infoCommand,
		listCommand,
		mountCommand,
		prepareCommand,
		removeCommand,
		setLabelCommand,
		treeCommand,
		unpackCommand,
		usageCommand,
		viewCommand,
	},
}

var listCommand = cli.Command{
	Name:    "list",
	Aliases: []string{"ls"},
	Usage:   "list snapshots",
	Action: func(context *cli.Context) error {
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		var (
			snapshotter = client.SnapshotService(context.GlobalString("snapshotter"))
			tw          = tabwriter.NewWriter(os.Stdout, 1, 8, 1, ' ', 0)
		)
		fmt.Fprintln(tw, "KEY\tPARENT\tKIND\t")
		if err := snapshotter.Walk(ctx, func(ctx gocontext.Context, info snapshots.Info) error {
			fmt.Fprintf(tw, "%v\t%v\t%v\t\n",
				info.Name,
				info.Parent,
				info.Kind)
			return nil
		}); err != nil {
			return err
		}

		return tw.Flush()
	},
}

var diffCommand = cli.Command{
	Name:      "diff",
	Usage:     "get the diff of two snapshots. the default second snapshot is the first snapshot's parent.",
	ArgsUsage: "[flags] <idA> [<idB>]",
	Flags: append([]cli.Flag{
		cli.StringFlag{
			Name:  "media-type",
			Usage: "media type to use for creating diff",
			Value: ocispec.MediaTypeImageLayerGzip,
		},
		cli.StringFlag{
			Name:  "ref",
			Usage: "content upload reference to use",
		},
		cli.BoolFlag{
			Name:  "keep",
			Usage: "keep diff content. up to creator to delete it.",
		},
	}, commands.LabelFlag),
	Action: func(context *cli.Context) error {
		var (
			idA = context.Args().First()
			idB = context.Args().Get(1)
		)
		if idA == "" {
			return errors.New("snapshot id must be provided")
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

		var desc ocispec.Descriptor
		labels := commands.LabelArgs(context.StringSlice("label"))
		snapshotter := client.SnapshotService(context.GlobalString("snapshotter"))

		fmt.Println(context.String("media-type"))

		if context.Bool("keep") {
			labels["containerd.io/gc.root"] = time.Now().UTC().Format(time.RFC3339)
		}
		opts := []diff.Opt{
			diff.WithMediaType(context.String("media-type")),
			diff.WithReference(context.String("ref")),
			diff.WithLabels(labels),
		}

		if idB == "" {
			desc, err = rootfs.CreateDiff(ctx, idA, snapshotter, client.DiffService(), opts...)
			if err != nil {
				return err
			}
		} else {
			var a, b []mount.Mount
			ds := client.DiffService()

			a, err = getMounts(ctx, idA, snapshotter)
			if err != nil {
				return err
			}
			b, err = getMounts(ctx, idB, snapshotter)
			if err != nil {
				return err
			}
			desc, err = ds.Compare(ctx, a, b, opts...)
			if err != nil {
				return err
			}
		}

		ra, err := client.ContentStore().ReaderAt(ctx, desc)
		if err != nil {
			return err
		}
		_, err = io.Copy(os.Stdout, content.NewReader(ra))

		return err
	},
}

func getMounts(ctx gocontext.Context, id string, sn snapshots.Snapshotter) ([]mount.Mount, error) {
	var mounts []mount.Mount
	info, err := sn.Stat(ctx, id)
	if err != nil {
		return nil, err
	}
	if info.Kind == snapshots.KindActive {
		mounts, err = sn.Mounts(ctx, id)
		if err != nil {
			return nil, err
		}
	} else {
		key := fmt.Sprintf("%s-view-key", id)
		mounts, err = sn.View(ctx, key, id)
		if err != nil {
			return nil, err
		}
		defer sn.Remove(ctx, key)
	}
	return mounts, nil
}

var usageCommand = cli.Command{
	Name:      "usage",
	Usage:     "usage snapshots",
	ArgsUsage: "[flags] [<key>, ...]",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "b",
			Usage: "display size in bytes",
		},
	},
	Action: func(context *cli.Context) error {
		var displaySize func(int64) string
		if context.Bool("b") {
			displaySize = func(s int64) string {
				return fmt.Sprintf("%d", s)
			}
		} else {
			displaySize = func(s int64) string {
				return progress.Bytes(s).String()
			}
		}
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		var (
			snapshotter = client.SnapshotService(context.GlobalString("snapshotter"))
			tw          = tabwriter.NewWriter(os.Stdout, 1, 8, 1, ' ', 0)
		)
		fmt.Fprintln(tw, "KEY\tSIZE\tINODES\t")
		if context.NArg() == 0 {
			if err := snapshotter.Walk(ctx, func(ctx gocontext.Context, info snapshots.Info) error {
				usage, err := snapshotter.Usage(ctx, info.Name)
				if err != nil {
					return err
				}
				fmt.Fprintf(tw, "%v\t%s\t%d\t\n", info.Name, displaySize(usage.Size), usage.Inodes)
				return nil
			}); err != nil {
				return err
			}
		} else {
			for _, id := range context.Args() {
				usage, err := snapshotter.Usage(ctx, id)
				if err != nil {
					return err
				}
				fmt.Fprintf(tw, "%v\t%s\t%d\t\n", id, displaySize(usage.Size), usage.Inodes)
			}
		}

		return tw.Flush()
	},
}

var removeCommand = cli.Command{
	Name:      "remove",
	Aliases:   []string{"rm"},
	ArgsUsage: "<key> [<key>, ...]",
	Usage:     "remove snapshots",
	Action: func(context *cli.Context) error {
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		snapshotter := client.SnapshotService(context.GlobalString("snapshotter"))
		for _, key := range context.Args() {
			err = snapshotter.Remove(ctx, key)
			if err != nil {
				return errors.Wrapf(err, "failed to remove %q", key)
			}
		}

		return nil
	},
}

var prepareCommand = cli.Command{
	Name:      "prepare",
	Usage:     "prepare a snapshot from a committed snapshot",
	ArgsUsage: "[flags] <key> [<parent>]",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "target, t",
			Usage: "mount target path, will print mount, if provided",
		},
	},
	Action: func(context *cli.Context) error {
		if narg := context.NArg(); narg < 1 || narg > 2 {
			return cli.ShowSubcommandHelp(context)
		}
		var (
			target = context.String("target")
			key    = context.Args().Get(0)
			parent = context.Args().Get(1)
		)
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()

		snapshotter := client.SnapshotService(context.GlobalString("snapshotter"))
		labels := map[string]string{
			"containerd.io/gc.root": time.Now().UTC().Format(time.RFC3339),
		}

		mounts, err := snapshotter.Prepare(ctx, key, parent, snapshots.WithLabels(labels))
		if err != nil {
			return err
		}

		if target != "" {
			printMounts(target, mounts)
		}

		return nil
	},
}

var viewCommand = cli.Command{
	Name:      "view",
	Usage:     "create a read-only snapshot from a committed snapshot",
	ArgsUsage: "[flags] <key> [<parent>]",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "target, t",
			Usage: "mount target path, will print mount, if provided",
		},
	},
	Action: func(context *cli.Context) error {
		if narg := context.NArg(); narg < 1 || narg > 2 {
			return cli.ShowSubcommandHelp(context)
		}
		var (
			target = context.String("target")
			key    = context.Args().Get(0)
			parent = context.Args().Get(1)
		)
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()

		snapshotter := client.SnapshotService(context.GlobalString("snapshotter"))
		mounts, err := snapshotter.View(ctx, key, parent)
		if err != nil {
			return err
		}

		if target != "" {
			printMounts(target, mounts)
		}

		return nil
	},
}

var mountCommand = cli.Command{
	Name:      "mounts",
	Aliases:   []string{"m", "mount"},
	Usage:     "mount gets mount commands for the snapshots",
	ArgsUsage: "<target> <key>",
	Action: func(context *cli.Context) error {
		if context.NArg() != 2 {
			return cli.ShowSubcommandHelp(context)
		}
		var (
			target = context.Args().Get(0)
			key    = context.Args().Get(1)
		)
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		snapshotter := client.SnapshotService(context.GlobalString("snapshotter"))
		mounts, err := snapshotter.Mounts(ctx, key)
		if err != nil {
			return err
		}

		printMounts(target, mounts)

		return nil
	},
}

var commitCommand = cli.Command{
	Name:      "commit",
	Usage:     "commit an active snapshot into the provided name",
	ArgsUsage: "<key> <active>",
	Action: func(context *cli.Context) error {
		if context.NArg() != 2 {
			return cli.ShowSubcommandHelp(context)
		}
		var (
			key    = context.Args().Get(0)
			active = context.Args().Get(1)
		)
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		snapshotter := client.SnapshotService(context.GlobalString("snapshotter"))
		labels := map[string]string{
			"containerd.io/gc.root": time.Now().UTC().Format(time.RFC3339),
		}
		return snapshotter.Commit(ctx, key, active, snapshots.WithLabels(labels))
	},
}

var treeCommand = cli.Command{
	Name:  "tree",
	Usage: "display tree view of snapshot branches",
	Action: func(context *cli.Context) error {
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		var (
			snapshotter = client.SnapshotService(context.GlobalString("snapshotter"))
			tree        = newSnapshotTree()
		)

		if err := snapshotter.Walk(ctx, func(ctx gocontext.Context, info snapshots.Info) error {
			// Get or create node and add node details
			tree.add(info)
			return nil
		}); err != nil {
			return err
		}

		printTree(tree)

		return nil
	},
}

var infoCommand = cli.Command{
	Name:      "info",
	Usage:     "get info about a snapshot",
	ArgsUsage: "<key>",
	Action: func(context *cli.Context) error {
		if context.NArg() != 1 {
			return cli.ShowSubcommandHelp(context)
		}

		key := context.Args().Get(0)
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		snapshotter := client.SnapshotService(context.GlobalString("snapshotter"))
		info, err := snapshotter.Stat(ctx, key)
		if err != nil {
			return err
		}

		commands.PrintAsJSON(info)

		return nil
	},
}

var setLabelCommand = cli.Command{
	Name:        "label",
	Usage:       "add labels to content",
	ArgsUsage:   "<name> [<label>=<value> ...]",
	Description: "labels snapshots in the snapshotter",
	Action: func(context *cli.Context) error {
		key, labels := commands.ObjectWithLabelArgs(context)
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()

		snapshotter := client.SnapshotService(context.GlobalString("snapshotter"))

		info := snapshots.Info{
			Name:   key,
			Labels: map[string]string{},
		}

		var paths []string
		for k, v := range labels {
			paths = append(paths, fmt.Sprintf("labels.%s", k))
			if v != "" {
				info.Labels[k] = v
			}
		}

		// Nothing updated, do no clear
		if len(paths) == 0 {
			info, err = snapshotter.Stat(ctx, info.Name)
		} else {
			info, err = snapshotter.Update(ctx, info, paths...)
		}
		if err != nil {
			return err
		}

		var labelStrings []string
		for k, v := range info.Labels {
			labelStrings = append(labelStrings, fmt.Sprintf("%s=%s", k, v))
		}

		fmt.Println(strings.Join(labelStrings, ","))

		return nil
	},
}

var unpackCommand = cli.Command{
	Name:      "unpack",
	Usage:     "unpack applies layers from a manifest to a snapshot",
	ArgsUsage: "[flags] <digest>",
	Flags:     commands.SnapshotterFlags,
	Action: func(context *cli.Context) error {
		dgst, err := digest.Parse(context.Args().First())
		if err != nil {
			return err
		}
		client, ctx, cancel, err := commands.NewClient(context)
		if err != nil {
			return err
		}
		defer cancel()
		log.G(ctx).Debugf("unpacking layers from manifest %s", dgst.String())
		// TODO: Support unpack by name
		images, err := client.ListImages(ctx)
		if err != nil {
			return err
		}
		var unpacked bool
		for _, image := range images {
			if image.Target().Digest == dgst {
				fmt.Printf("unpacking %s (%s)...", dgst, image.Target().MediaType)
				if err := image.Unpack(ctx, context.String("snapshotter")); err != nil {
					fmt.Println()
					return err
				}
				fmt.Println("done")
				unpacked = true
				break
			}
		}
		if !unpacked {
			return errors.New("manifest not found")
		}
		// TODO: Get rootfs from Image
		//log.G(ctx).Infof("chain ID: %s", chainID.String())
		return nil
	},
}

type snapshotTree struct {
	nodes []*snapshotTreeNode
	index map[string]*snapshotTreeNode
}

func newSnapshotTree() *snapshotTree {
	return &snapshotTree{
		index: make(map[string]*snapshotTreeNode),
	}
}

type snapshotTreeNode struct {
	info     snapshots.Info
	children []string
}

func (st *snapshotTree) add(info snapshots.Info) *snapshotTreeNode {
	entry, ok := st.index[info.Name]
	if !ok {
		entry = &snapshotTreeNode{info: info}
		st.nodes = append(st.nodes, entry)
		st.index[info.Name] = entry
	} else {
		entry.info = info // update info if we created placeholder
	}

	if info.Parent != "" {
		pn := st.get(info.Parent)
		if pn == nil {
			// create a placeholder
			pn = st.add(snapshots.Info{Name: info.Parent})
		}

		pn.children = append(pn.children, info.Name)
	}
	return entry
}

func (st *snapshotTree) get(name string) *snapshotTreeNode {
	return st.index[name]
}

func printTree(st *snapshotTree) {
	for _, node := range st.nodes {
		// Print for root(parent-less) nodes only
		if node.info.Parent == "" {
			printNode(node.info.Name, st, 0)
		}
	}
}

func printNode(name string, tree *snapshotTree, level int) {
	node := tree.index[name]
	prefix := strings.Repeat("  ", level)

	if level > 0 {
		prefix += "\\_"
	}

	fmt.Printf(prefix+" %s\n", node.info.Name)
	level++
	for _, child := range node.children {
		printNode(child, tree, level)
	}
}

func printMounts(target string, mounts []mount.Mount) {
	// FIXME: This is specific to Unix
	for _, m := range mounts {
		fmt.Printf("mount -t %s %s %s -o %s\n", m.Type, m.Source, target, strings.Join(m.Options, ","))
	}
}
