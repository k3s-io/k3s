//go:build !windows
// +build !windows

package rootless

import (
	"io"
	"path"
	"strings"

	"github.com/rootless-containers/rootlesskit/pkg/port"
	portbuiltin "github.com/rootless-containers/rootlesskit/pkg/port/builtin"
	portslirp4netns "github.com/rootless-containers/rootlesskit/pkg/port/slirp4netns"
	"github.com/sirupsen/logrus"
)

type logrusDebugWriter struct {
}

func (w *logrusDebugWriter) Write(p []byte) (int, error) {
	s := strings.TrimSuffix(string(p), "\n")
	logrus.Debug(s)
	return len(p), nil
}

type portDriver interface {
	NewParentDriver() (port.ParentDriver, error)
	NewChildDriver() port.ChildDriver
	LogWriter() io.Writer
	SetStateDir(string)
	APISocketPath() string
}

type builtinDriver struct {
	logWriter io.Writer
	stateDir  string
}

func (b *builtinDriver) NewParentDriver() (port.ParentDriver, error) {
	return portbuiltin.NewParentDriver(b.logWriter, b.stateDir)
}

func (b *builtinDriver) NewChildDriver() port.ChildDriver {
	return portbuiltin.NewChildDriver(b.logWriter)
}

func (b *builtinDriver) LogWriter() io.Writer {
	return b.logWriter
}

func (b *builtinDriver) SetStateDir(stateDir string) {
	b.stateDir = stateDir
}

func (b *builtinDriver) APISocketPath() string {
	return ""
}

type slirp4netnsDriver struct {
	logWriter io.Writer
	stateDir  string
}

func (s *slirp4netnsDriver) NewParentDriver() (port.ParentDriver, error) {
	return portslirp4netns.NewParentDriver(s.logWriter, s.APISocketPath())
}

func (s *slirp4netnsDriver) NewChildDriver() port.ChildDriver {
	return portslirp4netns.NewChildDriver()
}

func (s *slirp4netnsDriver) LogWriter() io.Writer {
	return s.logWriter
}

func (s *slirp4netnsDriver) SetStateDir(stateDir string) {
	s.stateDir = stateDir
}

func (s *slirp4netnsDriver) APISocketPath() string {
	if s.stateDir != "" {
		return path.Join(s.stateDir, ".s4nn.sock")
	}
	return ""
}

func getDriver(driverName string) portDriver {
	logWriter := &logrusDebugWriter{}

	if driverName == "slirp4netns" {
		return &slirp4netnsDriver{logWriter: logWriter}
	}

	if driverName != "" && driverName != "builtin" {
		logrus.Warnf("Unsupported port driver %s, using default builtin", driverName)
	}

	return &builtinDriver{logWriter: logWriter}
}
