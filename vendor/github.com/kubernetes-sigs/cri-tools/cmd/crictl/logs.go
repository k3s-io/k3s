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
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	timetypes "github.com/docker/docker/api/types/time"
	"github.com/urfave/cli/v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubelet/kuberuntime/logs"
)

var logsCommand = &cli.Command{
	Name:                   "logs",
	Usage:                  "Fetch the logs of a container",
	ArgsUsage:              "CONTAINER-ID",
	UseShortOptionHandling: true,
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:    "follow",
			Aliases: []string{"f"},
			Usage:   "Follow log output",
		},
		&cli.BoolFlag{
			Name:    "previous",
			Aliases: []string{"p"},
			Usage:   "Print the logs for the previous instance of the container in a pod if it exists",
		},
		&cli.Int64Flag{
			Name:  "tail",
			Value: -1,
			Usage: "Number of lines to show from the end of the logs. Defaults to all",
		},
		&cli.Int64Flag{
			Name:  "limit-bytes",
			Value: -1,
			Usage: "Maximum bytes of logs to return. Defaults to no limit",
		},
		&cli.StringFlag{
			Name:  "since",
			Value: "",
			Usage: "Show logs since timestamp (e.g. 2013-01-02T13:23:37) or relative (e.g. 42m for 42 minutes)",
		},
		&cli.BoolFlag{
			Name:    "timestamps",
			Aliases: []string{"t"},
			Usage:   "Show timestamps",
		},
	},
	Action: func(ctx *cli.Context) error {
		runtimeService, err := getRuntimeService(ctx)
		if err != nil {
			return err
		}

		containerID := ctx.Args().First()
		if containerID == "" {
			return fmt.Errorf("ID cannot be empty")
		}
		tailLines := ctx.Int64("tail")
		limitBytes := ctx.Int64("limit-bytes")
		since, err := parseTimestamp(ctx.String("since"))
		if err != nil {
			return err
		}
		timestamp := ctx.Bool("timestamps")
		previous := ctx.Bool("previous")
		logOptions := logs.NewLogOptions(&v1.PodLogOptions{
			Follow:     ctx.Bool("follow"),
			TailLines:  &tailLines,
			LimitBytes: &limitBytes,
			SinceTime:  since,
			Timestamps: timestamp,
		}, time.Now())
		status, err := runtimeService.ContainerStatus(containerID)
		if err != nil {
			return err
		}
		logPath := status.GetLogPath()
		if logPath == "" {
			return fmt.Errorf("The container has not set log path")
		}
		if previous {
			containerAttempt := status.GetMetadata().Attempt
			if containerAttempt == uint32(0) {
				return fmt.Errorf("Previous terminated container %s not found", status.GetMetadata().Name)
			}
			logPath = fmt.Sprintf("%s%s%s", logPath[:strings.LastIndex(logPath, "/")+1], fmt.Sprint(containerAttempt-1),
				logPath[strings.LastIndex(logPath, "."):])
		}
		return logs.ReadLogs(context.Background(), logPath, status.GetId(), logOptions, runtimeService, os.Stdout, os.Stderr)
	},
}

// parseTimestamp parses timestamp string as golang duration,
// then RFC3339 time and finally as a Unix timestamp.
func parseTimestamp(value string) (*metav1.Time, error) {
	if value == "" {
		return nil, nil
	}
	str, err := timetypes.GetTimestamp(value, time.Now())
	if err != nil {
		return nil, err
	}
	s, ns, err := timetypes.ParseTimestamps(str, 0)
	if err != nil {
		return nil, err
	}
	t := metav1.NewTime(time.Unix(s, ns))
	return &t, nil
}
