package proxy

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

// failConn fails every Read and Write with err.
type failConn struct{ err error }

func (c failConn) Read([]byte) (int, error)  { return 0, c.err }
func (c failConn) Write([]byte) (int, error) { return 0, c.err }
func (failConn) Close() error                { return nil }

// Test_UnitProxyBothDirectionsFail checks that no pipe goroutine stays blocked when both directions fail.
func Test_UnitProxyBothDirectionsFail(t *testing.T) {
	g := gomega.NewWithT(t)
	out := logrus.StandardLogger().Out
	logrus.SetOutput(t.Output())
	t.Cleanup(func() { logrus.SetOutput(out) })

	conn := failConn{errors.New("i/o error")}
	before := runtime.NumGoroutine()
	for range 10000 {
		_ = Proxy(conn, conn)
	}
	g.Eventually(runtime.NumGoroutine, 5*time.Second).Should(gomega.BeNumerically("<=", before))
}
