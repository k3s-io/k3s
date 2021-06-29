// +build !windows

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

package server

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/containerd/containerd/log"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/pkg/errors"
	"golang.org/x/net/context"

	runtime "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// portForward uses netns to enter the sandbox namespace, and forwards a stream inside the
// the namespace to a specific port. It keeps forwarding until it exits or client disconnect.
func (c *criService) portForward(ctx context.Context, id string, port int32, stream io.ReadWriteCloser) error {
	s, err := c.sandboxStore.Get(id)
	if err != nil {
		return errors.Wrapf(err, "failed to find sandbox %q in store", id)
	}

	var netNSDo func(func(ns.NetNS) error) error
	// netNSPath is the network namespace path for logging.
	var netNSPath string
	securityContext := s.Config.GetLinux().GetSecurityContext()
	hostNet := securityContext.GetNamespaceOptions().GetNetwork() == runtime.NamespaceMode_NODE
	if !hostNet {
		if closed, err := s.NetNS.Closed(); err != nil {
			return errors.Wrapf(err, "failed to check netwok namespace closed for sandbox %q", id)
		} else if closed {
			return errors.Errorf("network namespace for sandbox %q is closed", id)
		}
		netNSDo = s.NetNS.Do
		netNSPath = s.NetNS.GetPath()
	} else {
		// Run the function directly for host network.
		netNSDo = func(do func(_ ns.NetNS) error) error {
			return do(nil)
		}
		netNSPath = "host"
	}

	log.G(ctx).Infof("Executing port forwarding in network namespace %q", netNSPath)
	err = netNSDo(func(_ ns.NetNS) error {
		defer stream.Close()
		// TODO: hardcoded to tcp4 because localhost resolves to ::1 by default if the system has IPv6 enabled.
		// Theoretically happy eyeballs will try IPv6 first and fallback to IPv4
		// but resolving localhost doesn't seem to return and IPv4 address, thus failing the connection.
		conn, err := net.Dial("tcp4", fmt.Sprintf("localhost:%d", port))
		if err != nil {
			return errors.Wrapf(err, "failed to dial %d", port)
		}
		defer conn.Close()

		errCh := make(chan error, 2)
		// Copy from the the namespace port connection to the client stream
		go func() {
			log.G(ctx).Debugf("PortForward copying data from namespace %q port %d to the client stream", id, port)
			_, err := io.Copy(stream, conn)
			errCh <- err
		}()

		// Copy from the client stream to the namespace port connection
		go func() {
			log.G(ctx).Debugf("PortForward copying data from client stream to namespace %q port %d", id, port)
			_, err := io.Copy(conn, stream)
			errCh <- err
		}()

		// Wait until the first error is returned by one of the connections
		// we use errFwd to store the result of the port forwarding operation
		// if the context is cancelled close everything and return
		var errFwd error
		select {
		case errFwd = <-errCh:
			log.G(ctx).Debugf("PortForward stop forwarding in one direction in network namespace %q port %d: %v", id, port, errFwd)
		case <-ctx.Done():
			log.G(ctx).Debugf("PortForward cancelled in network namespace %q port %d: %v", id, port, ctx.Err())
			return ctx.Err()
		}
		// give a chance to terminate gracefully or timeout
		// after 1s
		// https://linux.die.net/man/1/socat
		const timeout = time.Second
		select {
		case e := <-errCh:
			if errFwd == nil {
				errFwd = e
			}
			log.G(ctx).Debugf("PortForward stopped forwarding in both directions in network namespace %q port %d: %v", id, port, e)
		case <-time.After(timeout):
			log.G(ctx).Debugf("PortForward timed out waiting to close the connection in network namespace %q port %d", id, port)
		case <-ctx.Done():
			log.G(ctx).Debugf("PortForward cancelled in network namespace %q port %d: %v", id, port, ctx.Err())
			errFwd = ctx.Err()
		}

		return errFwd
	})

	if err != nil {
		return errors.Wrapf(err, "failed to execute portforward in network namespace %q", netNSPath)
	}
	log.G(ctx).Infof("Finish port forwarding for %q port %d", id, port)

	return nil
}
