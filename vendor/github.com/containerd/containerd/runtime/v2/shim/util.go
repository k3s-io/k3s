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
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/namespaces"
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"
	"github.com/pkg/errors"
)

var runtimePaths sync.Map

// Command returns the shim command with the provided args and configuration
func Command(ctx context.Context, runtime, containerdAddress, containerdTTRPCAddress, path string, opts *types.Any, cmdArgs ...string) (*exec.Cmd, error) {
	ns, err := namespaces.NamespaceRequired(ctx)
	if err != nil {
		return nil, err
	}
	self, err := os.Executable()
	if err != nil {
		return nil, err
	}
	args := []string{
		"-namespace", ns,
		"-address", containerdAddress,
		"-publish-binary", self,
	}
	args = append(args, cmdArgs...)
	name := BinaryName(runtime)
	if name == "" {
		return nil, fmt.Errorf("invalid runtime name %s, correct runtime name should format like io.containerd.runc.v1", runtime)
	}

	var cmdPath string
	cmdPathI, cmdPathFound := runtimePaths.Load(name)
	if cmdPathFound {
		cmdPath = cmdPathI.(string)
	} else {
		var lerr error
		if cmdPath, lerr = exec.LookPath(name); lerr != nil {
			if eerr, ok := lerr.(*exec.Error); ok {
				if eerr.Err == exec.ErrNotFound {
					// LookPath only finds current directory matches based on
					// the callers current directory but the caller is not
					// likely in the same directory as the containerd
					// executables. Instead match the calling binaries path
					// (containerd) and see if they are side by side. If so
					// execute the shim found there.
					testPath := filepath.Join(filepath.Dir(self), name)
					if _, serr := os.Stat(testPath); serr == nil {
						cmdPath = testPath
					}
					if cmdPath == "" {
						return nil, errors.Wrapf(os.ErrNotExist, "runtime %q binary not installed %q", runtime, name)
					}
				}
			}
		}
		cmdPath, err = filepath.Abs(cmdPath)
		if err != nil {
			return nil, err
		}
		if cmdPathI, cmdPathFound = runtimePaths.LoadOrStore(name, cmdPath); cmdPathFound {
			// We didn't store cmdPath we loaded an already cached value. Use it.
			cmdPath = cmdPathI.(string)
		}
	}

	cmd := exec.Command(cmdPath, args...)
	cmd.Dir = path
	cmd.Env = append(
		os.Environ(),
		"GOMAXPROCS=2",
		fmt.Sprintf("%s=%s", ttrpcAddressEnv, containerdTTRPCAddress),
	)
	cmd.SysProcAttr = getSysProcAttr()
	if opts != nil {
		d, err := proto.Marshal(opts)
		if err != nil {
			return nil, err
		}
		cmd.Stdin = bytes.NewReader(d)
	}
	return cmd, nil
}

// BinaryName returns the shim binary name from the runtime name,
// empty string returns means runtime name is invalid
func BinaryName(runtime string) string {
	// runtime name should format like $prefix.name.version
	parts := strings.Split(runtime, ".")
	if len(parts) < 2 {
		return ""
	}

	return fmt.Sprintf(shimBinaryFormat, parts[len(parts)-2], parts[len(parts)-1])
}

// Connect to the provided address
func Connect(address string, d func(string, time.Duration) (net.Conn, error)) (net.Conn, error) {
	return d(address, 100*time.Second)
}

// WritePidFile writes a pid file atomically
func WritePidFile(path string, pid int) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	tempPath := filepath.Join(filepath.Dir(path), fmt.Sprintf(".%s", filepath.Base(path)))
	f, err := os.OpenFile(tempPath, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0666)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(f, "%d", pid)
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

// WriteAddress writes a address file atomically
func WriteAddress(path, address string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	tempPath := filepath.Join(filepath.Dir(path), fmt.Sprintf(".%s", filepath.Base(path)))
	f, err := os.OpenFile(tempPath, os.O_RDWR|os.O_CREATE|os.O_EXCL|os.O_SYNC, 0666)
	if err != nil {
		return err
	}
	_, err = f.WriteString(address)
	f.Close()
	if err != nil {
		return err
	}
	return os.Rename(tempPath, path)
}

// ErrNoAddress is returned when the address file has no content
var ErrNoAddress = errors.New("no shim address")

// ReadAddress returns the shim's abstract socket address from the path
func ReadAddress(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "", ErrNoAddress
	}
	return string(data), nil
}
