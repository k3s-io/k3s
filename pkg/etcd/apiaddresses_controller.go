package etcd

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	toolscache "k8s.io/client-go/tools/cache"
	toolswatch "k8s.io/client-go/tools/watch"
)

func registerEndpointsHandlers(ctx context.Context, etcd *ETCD) {
	labelSelector := labels.Set{discoveryv1.LabelServiceName: "kubernetes"}.String()
	lw := toolscache.NewFilteredListWatchFromClient(etcd.config.Runtime.K8s.DiscoveryV1().RESTClient(), "endpointslices", metav1.NamespaceDefault, func(options *metav1.ListOptions) { options.LabelSelector = labelSelector })
	_, _, watch, done := toolswatch.NewIndexerInformerWatcher(lw, &discoveryv1.EndpointSlice{})

	go func() {
		<-ctx.Done()
		watch.Stop()
		<-done
	}()

	h := &handler{
		etcd:  etcd,
		watch: watch,
	}

	logrus.Infof("Starting managed etcd apiserver addresses controller")
	go h.watchEndpointSlice(ctx)
}

type handler struct {
	etcd  *ETCD
	watch watch.Interface
}

// This controller will update the version.program/apiaddresses etcd key with a list of
// api addresses endpoint slices found in the kubernetes service in the default namespace
func (h *handler) watchEndpointSlice(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-h.watch.ResultChan():
			slice, ok := ev.Object.(*discoveryv1.EndpointSlice)
			if !ok {
				logrus.Fatalf("Failed to watch apiserver addresses: could not convert event object to endpointslice: %v", ev)
			}

			w := &bytes.Buffer{}
			if err := json.NewEncoder(w).Encode(util.GetAddressesFromSlices(*slice)); err != nil {
				logrus.Warnf("Failed to encode apiserver addresses: %v", err)
				continue
			}

			_, err := h.etcd.client.Put(ctx, AddressKey, w.String())
			if err != nil {
				logrus.Warnf("Failed to store apiserver addresses in etcd: %v", err)
			}
		}
	}
}
