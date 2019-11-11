package pipe

import (
	"io"
	"net"

	"github.com/lxc/lxd/shared/eagain"
	"github.com/sirupsen/logrus"
)

func UnixPiper(srcs <-chan net.Conn, bindAddress string) {
	for src := range srcs {
		go Unix(src, bindAddress)
	}
}

func Unix(src net.Conn, target string) error {
	dst, err := net.Dial("unix", target)
	if err != nil {
		src.Close()
		return err
	}

	Connect(src, dst)
	return nil
}

func Connect(src net.Conn, dst net.Conn) {
	go func() {
		_, err := io.Copy(eagain.Writer{Writer: dst}, eagain.Reader{Reader: src})
		if err != nil && err != io.EOF {
			logrus.Debugf("copy pipe src->dst closed: %v", err)
		}
		src.Close()
		dst.Close()
	}()

	go func() {
		_, err := io.Copy(eagain.Writer{Writer: src}, eagain.Reader{Reader: dst})
		if err != nil {
			logrus.Debugf("copy pipe dst->src closed: %v", err)
		}
		src.Close()
		dst.Close()
	}()
}
