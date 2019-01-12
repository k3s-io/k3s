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

package shim

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/containerd/containerd/events"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/namespaces"
	shimapi "github.com/containerd/containerd/runtime/v2/task"
	"github.com/containerd/ttrpc"
	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Client for a shim server
type Client struct {
	service shimapi.TaskService
	context context.Context
	signals chan os.Signal
}

// Init func for the creation of a shim server
type Init func(context.Context, string, events.Publisher) (Shim, error)

// Shim server interface
type Shim interface {
	shimapi.TaskService
	Cleanup(ctx context.Context) (*shimapi.DeleteResponse, error)
	StartShim(ctx context.Context, id, containerdBinary, containerdAddress string) (string, error)
}

// OptsKey is the context key for the Opts value.
type OptsKey struct{}

// Opts are context options associated with the shim invocation.
type Opts struct {
	BundlePath string
	Debug      bool
}

var (
	debugFlag            bool
	idFlag               string
	namespaceFlag        string
	socketFlag           string
	bundlePath           string
	addressFlag          string
	containerdBinaryFlag string
	action               string
)

func parseFlags() {
	flag.BoolVar(&debugFlag, "debug", false, "enable debug output in logs")
	flag.StringVar(&namespaceFlag, "namespace", "", "namespace that owns the shim")
	flag.StringVar(&idFlag, "id", "", "id of the task")
	flag.StringVar(&socketFlag, "socket", "", "abstract socket path to serve")
	flag.StringVar(&bundlePath, "bundle", "", "path to the bundle if not workdir")

	flag.StringVar(&addressFlag, "address", "", "grpc address back to main containerd")
	flag.StringVar(&containerdBinaryFlag, "publish-binary", "containerd", "path to publish binary (used for publishing events)")

	flag.Parse()
	action = flag.Arg(0)
}

func setRuntime() {
	debug.SetGCPercent(40)
	go func() {
		for range time.Tick(30 * time.Second) {
			debug.FreeOSMemory()
		}
	}()
	if os.Getenv("GOMAXPROCS") == "" {
		// If GOMAXPROCS hasn't been set, we default to a value of 2 to reduce
		// the number of Go stacks present in the shim.
		runtime.GOMAXPROCS(2)
	}
}

func setLogger(ctx context.Context, id string) error {
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: log.RFC3339NanoFixed,
		FullTimestamp:   true,
	})
	if debugFlag {
		logrus.SetLevel(logrus.DebugLevel)
	}
	f, err := openLog(ctx, id)
	if err != nil {
		return err
	}
	logrus.SetOutput(f)
	return nil
}

// Run initializes and runs a shim server
func Run(id string, initFunc Init) {
	if err := run(id, initFunc); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", id, err)
		os.Exit(1)
	}
}

func run(id string, initFunc Init) error {
	parseFlags()
	setRuntime()

	signals, err := setupSignals()
	if err != nil {
		return err
	}
	if err := subreaper(); err != nil {
		return err
	}
	publisher := &remoteEventsPublisher{
		address:              addressFlag,
		containerdBinaryPath: containerdBinaryFlag,
	}
	if namespaceFlag == "" {
		return fmt.Errorf("shim namespace cannot be empty")
	}
	ctx := namespaces.WithNamespace(context.Background(), namespaceFlag)
	ctx = context.WithValue(ctx, OptsKey{}, Opts{BundlePath: bundlePath, Debug: debugFlag})
	ctx = log.WithLogger(ctx, log.G(ctx).WithField("runtime", id))

	service, err := initFunc(ctx, idFlag, publisher)
	if err != nil {
		return err
	}
	switch action {
	case "delete":
		logger := logrus.WithFields(logrus.Fields{
			"pid":       os.Getpid(),
			"namespace": namespaceFlag,
		})
		go handleSignals(logger, signals)
		response, err := service.Cleanup(ctx)
		if err != nil {
			return err
		}
		data, err := proto.Marshal(response)
		if err != nil {
			return err
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return err
		}
		return nil
	case "start":
		address, err := service.StartShim(ctx, idFlag, containerdBinaryFlag, addressFlag)
		if err != nil {
			return err
		}
		if _, err := os.Stdout.WriteString(address); err != nil {
			return err
		}
		return nil
	default:
		if err := setLogger(ctx, idFlag); err != nil {
			return err
		}
		client := NewShimClient(ctx, service, signals)
		return client.Serve()
	}
}

// NewShimClient creates a new shim server client
func NewShimClient(ctx context.Context, svc shimapi.TaskService, signals chan os.Signal) *Client {
	s := &Client{
		service: svc,
		context: ctx,
		signals: signals,
	}
	return s
}

// Serve the shim server
func (s *Client) Serve() error {
	dump := make(chan os.Signal, 32)
	setupDumpStacks(dump)

	path, err := os.Getwd()
	if err != nil {
		return err
	}
	server, err := newServer()
	if err != nil {
		return errors.Wrap(err, "failed creating server")
	}

	logrus.Debug("registering ttrpc server")
	shimapi.RegisterTaskService(server, s.service)

	if err := serve(s.context, server, socketFlag); err != nil {
		return err
	}
	logger := logrus.WithFields(logrus.Fields{
		"pid":       os.Getpid(),
		"path":      path,
		"namespace": namespaceFlag,
	})
	go func() {
		for range dump {
			dumpStacks(logger)
		}
	}()
	return handleSignals(logger, s.signals)
}

// serve serves the ttrpc API over a unix socket at the provided path
// this function does not block
func serve(ctx context.Context, server *ttrpc.Server, path string) error {
	l, err := serveListener(path)
	if err != nil {
		return err
	}
	go func() {
		defer l.Close()
		if err := server.Serve(ctx, l); err != nil &&
			!strings.Contains(err.Error(), "use of closed network connection") {
			logrus.WithError(err).Fatal("containerd-shim: ttrpc server failure")
		}
	}()
	return nil
}

func dumpStacks(logger *logrus.Entry) {
	var (
		buf       []byte
		stackSize int
	)
	bufferLen := 16384
	for stackSize == len(buf) {
		buf = make([]byte, bufferLen)
		stackSize = runtime.Stack(buf, true)
		bufferLen *= 2
	}
	buf = buf[:stackSize]
	logger.Infof("=== BEGIN goroutine stack dump ===\n%s\n=== END goroutine stack dump ===", buf)
}

type remoteEventsPublisher struct {
	address              string
	containerdBinaryPath string
}
