// +build linux

package main

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/opencontainers/runc/libcontainer"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/urfave/cli"
)

type notifySocket struct {
	socket     *net.UnixConn
	host       string
	socketPath string
}

func newNotifySocket(context *cli.Context, notifySocketHost string, id string) *notifySocket {
	if notifySocketHost == "" {
		return nil
	}

	root := filepath.Join(context.GlobalString("root"), id)
	socketPath := filepath.Join(root, "notify", "notify.sock")

	notifySocket := &notifySocket{
		socket:     nil,
		host:       notifySocketHost,
		socketPath: socketPath,
	}

	return notifySocket
}

func (s *notifySocket) Close() error {
	return s.socket.Close()
}

// If systemd is supporting sd_notify protocol, this function will add support
// for sd_notify protocol from within the container.
func (s *notifySocket) setupSpec(context *cli.Context, spec *specs.Spec) error {
	pathInContainer := filepath.Join("/run/notify", path.Base(s.socketPath))
	mount := specs.Mount{
		Destination: path.Dir(pathInContainer),
		Source:      path.Dir(s.socketPath),
		Options:     []string{"bind", "nosuid", "noexec", "nodev", "ro"},
	}
	spec.Mounts = append(spec.Mounts, mount)
	spec.Process.Env = append(spec.Process.Env, fmt.Sprintf("NOTIFY_SOCKET=%s", pathInContainer))
	return nil
}

func (s *notifySocket) bindSocket() error {
	addr := net.UnixAddr{
		Name: s.socketPath,
		Net:  "unixgram",
	}

	socket, err := net.ListenUnixgram("unixgram", &addr)
	if err != nil {
		return err
	}

	err = os.Chmod(s.socketPath, 0777)
	if err != nil {
		socket.Close()
		return err
	}

	s.socket = socket
	return nil
}

func (s *notifySocket) setupSocketDirectory() error {
	return os.Mkdir(path.Dir(s.socketPath), 0755)
}

func notifySocketStart(context *cli.Context, notifySocketHost, id string) (*notifySocket, error) {
	notifySocket := newNotifySocket(context, notifySocketHost, id)
	if notifySocket == nil {
		return nil, nil
	}

	if err := notifySocket.bindSocket(); err != nil {
		return nil, err
	}
	return notifySocket, nil
}

func (n *notifySocket) waitForContainer(container libcontainer.Container) error {
	s, err := container.State()
	if err != nil {
		return err
	}
	return n.run(s.InitProcessPid)
}

func (n *notifySocket) run(pid1 int) error {
	if n.socket == nil {
		return nil
	}
	notifySocketHostAddr := net.UnixAddr{Name: n.host, Net: "unixgram"}
	client, err := net.DialUnix("unixgram", nil, &notifySocketHostAddr)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(time.Millisecond * 100)
	defer ticker.Stop()

	fileChan := make(chan []byte)
	go func() {
		for {
			buf := make([]byte, 4096)
			r, err := n.socket.Read(buf)
			if err != nil {
				return
			}
			got := buf[0:r]
			// systemd-ready sends a single datagram with the state string as payload,
			// so we don't need to worry about partial messages.
			for _, line := range bytes.Split(got, []byte{'\n'}) {
				if bytes.HasPrefix(got, []byte("READY=")) {
					fileChan <- line
					return
				}
			}

		}
	}()

	for {
		select {
		case <-ticker.C:
			_, err := os.Stat(filepath.Join("/proc", strconv.Itoa(pid1)))
			if err != nil {
				return nil
			}
		case b := <-fileChan:
			var out bytes.Buffer
			_, err = out.Write(b)
			if err != nil {
				return err
			}

			_, err = out.Write([]byte{'\n'})
			if err != nil {
				return err
			}

			_, err = client.Write(out.Bytes())
			if err != nil {
				return err
			}

			// now we can inform systemd to use pid1 as the pid to monitor
			newPid := fmt.Sprintf("MAINPID=%d\n", pid1)
			client.Write([]byte(newPid))
			return nil
		}
	}
}
