package etcd_test

import (
	"context"
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rancher/k3s/pkg/cluster"
	"github.com/rancher/k3s/pkg/daemons/config"
)

var ctx context.Context
var cnf *config.Control
var testCluster *cluster.Cluster
var httpHandler http.Handler
var _ = BeforeSuite(func() {
})

var _ = Describe("etcd", func() {
	Context("when a new etcd is created", func() {
		BeforeEach(func() {
		})
		It("starts up with no problems", func() {

		})
		It("reset with no problems", func() {

		})
	})
})

func TestEtcd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Etcd Suite")
}
