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

package v2

import (
	"bytes"
	"context"
	"io"
	"os"
	gruntime "runtime"
	"strings"

	eventstypes "github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/events/exchange"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/runtime"
	client "github.com/containerd/containerd/runtime/v2/shim"
	"github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/ttrpc"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func shimBinary(ctx context.Context, bundle *Bundle, runtime, containerdAddress string, events *exchange.Exchange, rt *runtime.TaskList) *binary {
	return &binary{
		bundle:            bundle,
		runtime:           runtime,
		containerdAddress: containerdAddress,
		events:            events,
		rtTasks:           rt,
	}
}

type binary struct {
	runtime           string
	containerdAddress string
	bundle            *Bundle
	events            *exchange.Exchange
	rtTasks           *runtime.TaskList
}

func (b *binary) Start(ctx context.Context) (_ *shim, err error) {
	args := []string{"-id", b.bundle.ID}
	if logrus.GetLevel() == logrus.DebugLevel {
		args = append(args, "-debug")
	}
	args = append(args, "start")

	cmd, err := client.Command(
		ctx,
		b.runtime,
		b.containerdAddress,
		b.bundle.Path,
		args...,
	)
	if err != nil {
		return nil, err
	}
	f, err := openShimLog(ctx, b.bundle)
	if err != nil {
		return nil, errors.Wrap(err, "open shim log pipe")
	}
	defer func() {
		if err != nil {
			f.Close()
		}
	}()
	// open the log pipe and block until the writer is ready
	// this helps with synchronization of the shim
	// copy the shim's logs to containerd's output
	go func() {
		defer f.Close()
		if _, err := io.Copy(os.Stderr, f); err != nil {
			log.G(ctx).WithError(err).Error("copy shim log")
		}
	}()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, errors.Wrapf(err, "%s", out)
	}
	address := strings.TrimSpace(string(out))
	conn, err := client.Connect(address, client.AnonDialer)
	if err != nil {
		return nil, err
	}
	client := ttrpc.NewClient(conn)
	client.OnClose(func() { conn.Close() })
	return &shim{
		bundle:  b.bundle,
		client:  client,
		task:    task.NewTaskClient(client),
		events:  b.events,
		rtTasks: b.rtTasks,
	}, nil
}

func (b *binary) Delete(ctx context.Context) (*runtime.Exit, error) {
	log.G(ctx).Info("cleaning up dead shim")

	// Windows cannot delete the current working directory while an
	// executable is in use with it. For the cleanup case we invoke with the
	// default work dir and forward the bundle path on the cmdline.
	var bundlePath string
	if gruntime.GOOS != "windows" {
		bundlePath = b.bundle.Path
	}

	cmd, err := client.Command(ctx,
		b.runtime,
		b.containerdAddress,
		bundlePath,
		"-id", b.bundle.ID,
		"-bundle", b.bundle.Path,
		"delete")
	if err != nil {
		return nil, err
	}
	var (
		out  = bytes.NewBuffer(nil)
		errb = bytes.NewBuffer(nil)
	)
	cmd.Stdout = out
	cmd.Stderr = errb
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrapf(err, "%s", errb.String())
	}
	s := errb.String()
	if s != "" {
		log.G(ctx).Warnf("cleanup warnings %s", s)
	}
	var response task.DeleteResponse
	if err := response.Unmarshal(out.Bytes()); err != nil {
		return nil, err
	}
	if err := b.bundle.Delete(); err != nil {
		return nil, err
	}
	// remove self from the runtime task list
	// this seems dirty but it cleans up the API across runtimes, tasks, and the service
	b.rtTasks.Delete(ctx, b.bundle.ID)
	// shim will send the exit event
	b.events.Publish(ctx, runtime.TaskDeleteEventTopic, &eventstypes.TaskDelete{
		ContainerID: b.bundle.ID,
		ExitStatus:  response.ExitStatus,
		ExitedAt:    response.ExitedAt,
		Pid:         response.Pid,
	})
	return &runtime.Exit{
		Status:    response.ExitStatus,
		Timestamp: response.ExitedAt,
		Pid:       response.Pid,
	}, nil
}
