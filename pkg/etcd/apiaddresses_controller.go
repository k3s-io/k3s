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
	indexer, informer, watch, done := toolswatch.NewIndexerInformerWatcher(lw, &discoveryv1.EndpointSlice{})

	go func() {
		<-ctx.Done()
		watch.Stop()
		<-done
	}()

	h := &handler{
		etcd:     etcd,
		indexer:  indexer,
		informer: informer,
		watch:    watch,
	}

	logrus.Infof("Starting managed etcd apiserver addresses controller")
	go h.informer.RunWithContext(ctx)
	go h.watchEndpointSlice(ctx)
}

type handler struct {
	etcd     *ETCD
	indexer  toolscache.Indexer
	informer toolscache.Controller
	watch    watch.Interface
}

// This controller will update the version.program/apiaddresses etcd key with a list of
// api addresses endpoint slices found in the kubernetes service in the default namespace
func (h *handler) watchEndpointSlice(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.watch.ResultChan():
			if !h.informer.HasSynced() {
				continue
			}
			objs := h.indexer.List()
			eps := make([]discoveryv1.EndpointSlice, 0, len(objs))
			for _, obj := range objs {
				if ep, ok := obj.(*discoveryv1.EndpointSlice); ok {
					eps = append(eps, *ep)
				} else {
					logrus.Warnf("Watch apiserver addresses: expected *discoveryv1.EndpointSlice, got %T", obj)
				}
			}

			w := &bytes.Buffer{}
			if err := json.NewEncoder(w).Encode(util.GetAddressesFromSlices(eps...)); err != nil {
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
