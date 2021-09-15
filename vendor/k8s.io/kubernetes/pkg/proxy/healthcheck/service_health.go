/*
Copyright 2016 The Kubernetes Authors.

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

package healthcheck

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/lithammer/dedent"
	v1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	api "k8s.io/kubernetes/pkg/apis/core"
)

// ServiceHealthServer serves HTTP endpoints for each service name, with results
// based on the endpoints.  If there are 0 endpoints for a service, it returns a
// 503 "Service Unavailable" error (telling LBs not to use this node).  If there
// are 1 or more endpoints, it returns a 200 "OK".
type ServiceHealthServer interface {
	// Make the new set of services be active.  Services that were open before
	// will be closed.  Services that are new will be opened.  Service that
	// existed and are in the new set will be left alone.  The value of the map
	// is the healthcheck-port to listen on.
	SyncServices(newServices map[types.NamespacedName]uint16) error
	// Make the new set of endpoints be active.  Endpoints for services that do
	// not exist will be dropped.  The value of the map is the number of
	// endpoints the service has on this node.
	SyncEndpoints(newEndpoints map[types.NamespacedName]int) error
}

func newServiceHealthServer(hostname string, recorder events.EventRecorder, listener listener, factory httpServerFactory) ServiceHealthServer {
	return &server{
		hostname:    hostname,
		recorder:    recorder,
		listener:    listener,
		httpFactory: factory,
		services:    map[types.NamespacedName]*hcInstance{},
	}
}

// NewServiceHealthServer allocates a new service healthcheck server manager
func NewServiceHealthServer(hostname string, recorder events.EventRecorder) ServiceHealthServer {
	return newServiceHealthServer(hostname, recorder, stdNetListener{}, stdHTTPServerFactory{})
}

type server struct {
	hostname    string
	recorder    events.EventRecorder // can be nil
	listener    listener
	httpFactory httpServerFactory

	lock     sync.RWMutex
	services map[types.NamespacedName]*hcInstance
}

func (hcs *server) SyncServices(newServices map[types.NamespacedName]uint16) error {
	hcs.lock.Lock()
	defer hcs.lock.Unlock()

	// Remove any that are not needed any more.
	for nsn, svc := range hcs.services {
		if port, found := newServices[nsn]; !found || port != svc.port {
			klog.V(2).Infof("Closing healthcheck %q on port %d", nsn.String(), svc.port)
			if err := svc.listener.Close(); err != nil {
				klog.Errorf("Close(%v): %v", svc.listener.Addr(), err)
			}
			delete(hcs.services, nsn)
		}
	}

	// Add any that are needed.
	for nsn, port := range newServices {
		if hcs.services[nsn] != nil {
			klog.V(3).Infof("Existing healthcheck %q on port %d", nsn.String(), port)
			continue
		}

		klog.V(2).Infof("Opening healthcheck %q on port %d", nsn.String(), port)
		svc := &hcInstance{port: port}
		addr := fmt.Sprintf(":%d", port)
		svc.server = hcs.httpFactory.New(addr, hcHandler{name: nsn, hcs: hcs})
		var err error
		svc.listener, err = hcs.listener.Listen(addr)
		if err != nil {
			msg := fmt.Sprintf("node %s failed to start healthcheck %q on port %d: %v", hcs.hostname, nsn.String(), port, err)

			if hcs.recorder != nil {
				hcs.recorder.Eventf(
					&v1.ObjectReference{
						Kind:      "Service",
						Namespace: nsn.Namespace,
						Name:      nsn.Name,
						UID:       types.UID(nsn.String()),
					}, nil, api.EventTypeWarning, "FailedToStartServiceHealthcheck", "Listen", msg)
			}
			klog.Error(msg)
			continue
		}
		hcs.services[nsn] = svc

		go func(nsn types.NamespacedName, svc *hcInstance) {
			// Serve() will exit when the listener is closed.
			klog.V(3).Infof("Starting goroutine for healthcheck %q on port %d", nsn.String(), svc.port)
			if err := svc.server.Serve(svc.listener); err != nil {
				klog.V(3).Infof("Healthcheck %q closed: %v", nsn.String(), err)
				return
			}
			klog.V(3).Infof("Healthcheck %q closed", nsn.String())
		}(nsn, svc)
	}
	return nil
}

type hcInstance struct {
	port      uint16
	listener  net.Listener
	server    httpServer
	endpoints int // number of local endpoints for a service
}

type hcHandler struct {
	name types.NamespacedName
	hcs  *server
}

var _ http.Handler = hcHandler{}

func (h hcHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	h.hcs.lock.RLock()
	svc, ok := h.hcs.services[h.name]
	if !ok || svc == nil {
		h.hcs.lock.RUnlock()
		klog.Errorf("Received request for closed healthcheck %q", h.name.String())
		return
	}
	count := svc.endpoints
	h.hcs.lock.RUnlock()

	resp.Header().Set("Content-Type", "application/json")
	resp.Header().Set("X-Content-Type-Options", "nosniff")
	if count == 0 {
		resp.WriteHeader(http.StatusServiceUnavailable)
	} else {
		resp.WriteHeader(http.StatusOK)
	}
	fmt.Fprint(resp, strings.Trim(dedent.Dedent(fmt.Sprintf(`
		{
			"service": {
				"namespace": %q,
				"name": %q
			},
			"localEndpoints": %d
		}
		`, h.name.Namespace, h.name.Name, count)), "\n"))
}

func (hcs *server) SyncEndpoints(newEndpoints map[types.NamespacedName]int) error {
	hcs.lock.Lock()
	defer hcs.lock.Unlock()

	for nsn, count := range newEndpoints {
		if hcs.services[nsn] == nil {
			klog.V(3).Infof("Not saving endpoints for unknown healthcheck %q", nsn.String())
			continue
		}
		klog.V(3).Infof("Reporting %d endpoints for healthcheck %q", count, nsn.String())
		hcs.services[nsn].endpoints = count
	}
	for nsn, hci := range hcs.services {
		if _, found := newEndpoints[nsn]; !found {
			hci.endpoints = 0
		}
	}
	return nil
}

// FakeServiceHealthServer is a fake ServiceHealthServer for test programs
type FakeServiceHealthServer struct{}

// NewFakeServiceHealthServer allocates a new fake service healthcheck server manager
func NewFakeServiceHealthServer() ServiceHealthServer {
	return FakeServiceHealthServer{}
}

// SyncServices is part of ServiceHealthServer
func (fake FakeServiceHealthServer) SyncServices(_ map[types.NamespacedName]uint16) error {
	return nil
}

// SyncEndpoints is part of ServiceHealthServer
func (fake FakeServiceHealthServer) SyncEndpoints(_ map[types.NamespacedName]int) error {
	return nil
}
