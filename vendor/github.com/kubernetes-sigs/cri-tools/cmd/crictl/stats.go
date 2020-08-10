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

package crictl

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"time"

	"github.com/docker/go-units"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"golang.org/x/net/context"
	pb "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

type statsOptions struct {
	// all containers
	all bool
	// id of container
	id string
	// podID of container
	podID string
	// sample is the duration for sampling cpu usage.
	sample time.Duration
	// labels are selectors for the sandbox
	labels map[string]string
	// output format
	output string
	// live watch
	watch bool
}

var statsCommand = &cli.Command{
	Name:                   "stats",
	Usage:                  "List container(s) resource usage statistics",
	UseShortOptionHandling: true,
	ArgsUsage:              "[ID]",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "all",
			Aliases: []string{"a"},
			Usage:   "Show all containers (default shows just running)",
		},
		&cli.StringFlag{
			Name:  "id",
			Value: "",
			Usage: "Filter by container id",
		},
		&cli.StringFlag{
			Name:    "pod",
			Aliases: []string{"p"},
			Value:   "",
			Usage:   "Filter by pod id",
		},
		&cli.StringSliceFlag{
			Name:  "label",
			Usage: "Filter by key=value label",
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "Output format, One of: json|yaml|table",
		},
		&cli.IntFlag{
			Name:    "seconds",
			Aliases: []string{"s"},
			Value:   1,
			Usage:   "Sample duration for CPU usage in seconds",
		},
		&cli.BoolFlag{
			Name:    "watch",
			Aliases: []string{"w"},
			Usage:   "Watch pod resources",
		},
	},
	Action: func(context *cli.Context) error {
		runtimeClient, runtimeConn, err := getRuntimeClient(context)
		if err != nil {
			return err
		}
		defer closeConnection(context, runtimeConn)

		id := context.String("id")
		if id == "" && context.NArg() > 0 {
			id = context.Args().Get(0)
		}

		opts := statsOptions{
			all:    context.Bool("all"),
			id:     id,
			podID:  context.String("pod"),
			sample: time.Duration(context.Int("seconds")) * time.Second,
			output: context.String("output"),
			watch:  context.Bool("watch"),
		}
		opts.labels, err = parseLabelStringSlice(context.StringSlice("label"))
		if err != nil {
			return err
		}

		if err = ContainerStats(runtimeClient, opts); err != nil {
			return errors.Wrap(err, "get container stats")
		}
		return nil
	},
}

type containerStatsByID []*pb.ContainerStats

func (c containerStatsByID) Len() int      { return len(c) }
func (c containerStatsByID) Swap(i, j int) { c[i], c[j] = c[j], c[i] }
func (c containerStatsByID) Less(i, j int) bool {
	return c[i].Attributes.Id < c[j].Attributes.Id
}

// ContainerStats sends a ListContainerStatsRequest to the server, and
// parses the returned ListContainerStatsResponse.
func ContainerStats(client pb.RuntimeServiceClient, opts statsOptions) error {
	filter := &pb.ContainerStatsFilter{}
	if opts.id != "" {
		filter.Id = opts.id
	}
	if opts.podID != "" {
		filter.PodSandboxId = opts.podID
	}
	if opts.labels != nil {
		filter.LabelSelector = opts.labels
	}
	request := &pb.ListContainerStatsRequest{
		Filter: filter,
	}

	display := newTableDisplay(20, 1, 3, ' ', 0)
	if !opts.watch {
		if err := displayStats(client, request, display, opts); err != nil {
			return err
		}
	} else {
		s := make(chan os.Signal)
		signal.Notify(s, os.Interrupt)
		go func() {
			<-s
			os.Exit(0)
		}()
		for range time.Tick(500 * time.Millisecond) {
			if err := displayStats(client, request, display, opts); err != nil {
				return err
			}
		}
	}

	return nil
}

func getContainerStats(client pb.RuntimeServiceClient, request *pb.ListContainerStatsRequest) (*pb.ListContainerStatsResponse, error) {
	logrus.Debugf("ListContainerStatsRequest: %v", request)
	r, err := client.ListContainerStats(context.Background(), request)
	logrus.Debugf("ListContainerResponse: %v", r)
	if err != nil {
		return nil, err
	}
	sort.Sort(containerStatsByID(r.Stats))
	return r, nil
}

func displayStats(client pb.RuntimeServiceClient, request *pb.ListContainerStatsRequest, display *display, opts statsOptions) error {
	r, err := getContainerStats(client, request)
	if err != nil {
		return err
	}
	switch opts.output {
	case "json":
		return outputProtobufObjAsJSON(r)
	case "yaml":
		return outputProtobufObjAsYAML(r)
	}
	oldStats := make(map[string]*pb.ContainerStats)
	for _, s := range r.GetStats() {
		oldStats[s.Attributes.Id] = s
	}

	time.Sleep(opts.sample)

	r, err = getContainerStats(client, request)
	if err != nil {
		return err
	}

	display.AddRow([]string{columnContainer, columnCPU, columnMemory, columnDisk, columnInodes})
	for _, s := range r.GetStats() {
		id := getTruncatedID(s.Attributes.Id, "")
		cpu := s.GetCpu().GetUsageCoreNanoSeconds().GetValue()
		mem := s.GetMemory().GetWorkingSetBytes().GetValue()
		disk := s.GetWritableLayer().GetUsedBytes().GetValue()
		inodes := s.GetWritableLayer().GetInodesUsed().GetValue()
		if !opts.all && cpu == 0 && mem == 0 {
			// Skip non-running container
			continue
		}
		old, ok := oldStats[s.Attributes.Id]
		if !ok {
			// Skip new container
			continue
		}
		var cpuPerc float64
		if cpu != 0 {
			// Only generate cpuPerc for running container
			duration := s.GetCpu().GetTimestamp() - old.GetCpu().GetTimestamp()
			if duration == 0 {
				return fmt.Errorf("cpu stat is not updated during sample")
			}
			cpuPerc = float64(cpu-old.GetCpu().GetUsageCoreNanoSeconds().GetValue()) / float64(duration) * 100
		}
		display.AddRow([]string{id, fmt.Sprintf("%.2f", cpuPerc), units.HumanSize(float64(mem)),
			units.HumanSize(float64(disk)), fmt.Sprintf("%d", inodes)})

	}
	display.ClearScreen()
	display.Flush()

	return nil
}
