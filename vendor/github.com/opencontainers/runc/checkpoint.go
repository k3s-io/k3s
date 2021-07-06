// +build linux

package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	criu "github.com/checkpoint-restore/go-criu/v5/rpc"
	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runc/libcontainer/userns"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/sys/unix"
)

var checkpointCommand = cli.Command{
	Name:  "checkpoint",
	Usage: "checkpoint a running container",
	ArgsUsage: `<container-id>

Where "<container-id>" is the name for the instance of the container to be
checkpointed.`,
	Description: `The checkpoint command saves the state of the container instance.`,
	Flags: []cli.Flag{
		cli.StringFlag{Name: "image-path", Value: "", Usage: "path for saving criu image files"},
		cli.StringFlag{Name: "work-path", Value: "", Usage: "path for saving work files and logs"},
		cli.StringFlag{Name: "parent-path", Value: "", Usage: "path for previous criu image files in pre-dump"},
		cli.BoolFlag{Name: "leave-running", Usage: "leave the process running after checkpointing"},
		cli.BoolFlag{Name: "tcp-established", Usage: "allow open tcp connections"},
		cli.BoolFlag{Name: "ext-unix-sk", Usage: "allow external unix sockets"},
		cli.BoolFlag{Name: "shell-job", Usage: "allow shell jobs"},
		cli.BoolFlag{Name: "lazy-pages", Usage: "use userfaultfd to lazily restore memory pages"},
		cli.IntFlag{Name: "status-fd", Value: -1, Usage: "criu writes \\0 to this FD once lazy-pages is ready"},
		cli.StringFlag{Name: "page-server", Value: "", Usage: "ADDRESS:PORT of the page server"},
		cli.BoolFlag{Name: "file-locks", Usage: "handle file locks, for safety"},
		cli.BoolFlag{Name: "pre-dump", Usage: "dump container's memory information only, leave the container running after this"},
		cli.StringFlag{Name: "manage-cgroups-mode", Value: "", Usage: "cgroups mode: 'soft' (default), 'full' and 'strict'"},
		cli.StringSliceFlag{Name: "empty-ns", Usage: "create a namespace, but don't restore its properties"},
		cli.BoolFlag{Name: "auto-dedup", Usage: "enable auto deduplication of memory images"},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 1, exactArgs); err != nil {
			return err
		}
		// XXX: Currently this is untested with rootless containers.
		if os.Geteuid() != 0 || userns.RunningInUserNS() {
			logrus.Warn("runc checkpoint is untested with rootless containers")
		}

		container, err := getContainer(context)
		if err != nil {
			return err
		}
		status, err := container.Status()
		if err != nil {
			return err
		}
		if status == libcontainer.Created || status == libcontainer.Stopped {
			fatal(fmt.Errorf("Container cannot be checkpointed in %s state", status.String()))
		}
		options := criuOptions(context)
		if !(options.LeaveRunning || options.PreDump) {
			// destroy container unless we tell CRIU to keep it
			defer destroy(container)
		}
		// these are the mandatory criu options for a container
		setPageServer(context, options)
		setManageCgroupsMode(context, options)
		if err := setEmptyNsMask(context, options); err != nil {
			return err
		}
		return container.Checkpoint(options)
	},
}

func prepareImagePaths(context *cli.Context) (string, string, error) {
	imagePath := context.String("image-path")
	if imagePath == "" {
		imagePath = getDefaultImagePath(context)
	}

	if err := os.MkdirAll(imagePath, 0600); err != nil {
		return "", "", err
	}

	parentPath := context.String("parent-path")
	if parentPath == "" {
		return imagePath, parentPath, nil
	}

	if filepath.IsAbs(parentPath) {
		return "", "", errors.New("--parent-path must be relative")
	}

	realParent := filepath.Join(imagePath, parentPath)
	fi, err := os.Stat(realParent)
	if err == nil && !fi.IsDir() {
		err = &os.PathError{Path: realParent, Err: unix.ENOTDIR}
	}

	if err != nil {
		return "", "", fmt.Errorf("invalid --parent-path: %w", err)
	}

	return imagePath, parentPath, nil

}

func setPageServer(context *cli.Context, options *libcontainer.CriuOpts) {
	// xxx following criu opts are optional
	// The dump image can be sent to a criu page server
	if psOpt := context.String("page-server"); psOpt != "" {
		address, port, err := net.SplitHostPort(psOpt)

		if err != nil || address == "" || port == "" {
			fatal(errors.New("Use --page-server ADDRESS:PORT to specify page server"))
		}
		portInt, err := strconv.Atoi(port)
		if err != nil {
			fatal(errors.New("Invalid port number"))
		}
		options.PageServer = libcontainer.CriuPageServerInfo{
			Address: address,
			Port:    int32(portInt),
		}
	}
}

func setManageCgroupsMode(context *cli.Context, options *libcontainer.CriuOpts) {
	if cgOpt := context.String("manage-cgroups-mode"); cgOpt != "" {
		switch cgOpt {
		case "soft":
			options.ManageCgroupsMode = criu.CriuCgMode_SOFT
		case "full":
			options.ManageCgroupsMode = criu.CriuCgMode_FULL
		case "strict":
			options.ManageCgroupsMode = criu.CriuCgMode_STRICT
		default:
			fatal(errors.New("Invalid manage cgroups mode"))
		}
	}
}

var namespaceMapping = map[specs.LinuxNamespaceType]int{
	specs.NetworkNamespace: unix.CLONE_NEWNET,
}

func setEmptyNsMask(context *cli.Context, options *libcontainer.CriuOpts) error {
	/* Runc doesn't manage network devices and their configuration */
	nsmask := unix.CLONE_NEWNET

	for _, ns := range context.StringSlice("empty-ns") {
		f, exists := namespaceMapping[specs.LinuxNamespaceType(ns)]
		if !exists {
			return fmt.Errorf("namespace %q is not supported", ns)
		}
		nsmask |= f
	}

	options.EmptyNs = uint32(nsmask)
	return nil
}
