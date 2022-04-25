package etcd

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	toolswatch "k8s.io/client-go/tools/watch"
)

func registerEndpointsHandlers(ctx context.Context, etcd *ETCD) {
	if etcd.config.DisableAPIServer {
		return
	}

	endpoints := etcd.config.Runtime.Core.Core().V1().Endpoints()
	fieldSelector := fields.Set{metav1.ObjectNameField: "kubernetes"}.String()
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (object runtime.Object, e error) {
			options.FieldSelector = fieldSelector
			return endpoints.List(metav1.NamespaceDefault, options)
		},
		WatchFunc: func(options metav1.ListOptions) (i watch.Interface, e error) {
			options.FieldSelector = fieldSelector
			return endpoints.Watch(metav1.NamespaceDefault, options)
		},
	}

	_, _, watch, done := toolswatch.NewIndexerInformerWatcher(lw, &v1.Endpoints{})

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
	go h.watchEndpoints(ctx)
}

type handler struct {
	etcd  *ETCD
	watch watch.Interface
}

// This controller will update the version.program/apiaddresses etcd key with a list of
// api addresses endpoints found in the kubernetes service in the default namespace
func (h *handler) watchEndpoints(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-h.watch.ResultChan():
			endpoint, ok := ev.Object.(*v1.Endpoints)
			if !ok {
				logrus.Fatalf("Failed to watch apiserver addresses: could not convert event object to endpoint: %v", ev)
			}

			w := &bytes.Buffer{}
			if err := json.NewEncoder(w).Encode(util.GetAddresses(endpoint)); err != nil {
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
