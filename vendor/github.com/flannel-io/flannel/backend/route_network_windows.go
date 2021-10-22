// Copyright 2018 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backend

import (
	"strings"
	"sync"
	"time"

	"golang.org/x/net/context"

	"github.com/flannel-io/flannel/pkg/routing"
	"github.com/flannel-io/flannel/subnet"
	log "k8s.io/klog"
)

const (
	routeCheckRetries = 10
)

type RouteNetwork struct {
	SimpleNetwork
	Name        string
	BackendType string
	SM          subnet.Manager
	GetRoute    func(lease *subnet.Lease) *routing.Route
	Mtu         int
	LinkIndex   int
	routes      []routing.Route
}

func (n *RouteNetwork) MTU() int {
	return n.Mtu
}

func (n *RouteNetwork) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	log.Info("Watching for new subnet leases")
	evts := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, n.SM, n.SubnetLease, evts)
		wg.Done()
	}()

	n.routes = make([]routing.Route, 0, 10)
	wg.Add(1)
	go func() {
		n.routeCheck(ctx)
		wg.Done()
	}()

	defer wg.Wait()

	for {
		select {
		case evtBatch, ok := <-evts:
			if !ok {
				log.Infof("evts chan closed")
				return
			}
			n.handleSubnetEvents(evtBatch)
		}
	}
}

func (n *RouteNetwork) handleSubnetEvents(batch []subnet.Event) {
	router := routing.RouterWindows{}

	for _, evt := range batch {
		leaseSubnet := evt.Lease.Subnet
		leaseAttrs := evt.Lease.Attrs
		if !strings.EqualFold(leaseAttrs.BackendType, n.BackendType) {
			log.Warningf("Ignoring non-%v subnet(%v): type=%v", n.BackendType, leaseSubnet, leaseAttrs.BackendType)
			continue
		}

		expectedRoute := n.GetRoute(&evt.Lease)

		switch evt.Type {
		case subnet.EventAdded:
			log.Infof("Subnet added: %v via %v", leaseSubnet, leaseAttrs.PublicIP)

			existingRoutes, _ := router.GetRoutesFromInterfaceToSubnet(expectedRoute.InterfaceIndex, expectedRoute.DestinationSubnet)
			if len(existingRoutes) > 0 {
				existingRoute := existingRoutes[0]
				if existingRoute.Equal(*expectedRoute) {
					continue
				}

				log.Warningf("Replacing existing route %v via %v with %v via %v", leaseSubnet, existingRoute.GatewayAddress, leaseSubnet, leaseAttrs.PublicIP)
				err := router.DeleteRoute(existingRoute.InterfaceIndex, existingRoute.DestinationSubnet, existingRoute.GatewayAddress)
				if err != nil {
					log.Errorf("Error removing route: %v", err)
					continue
				}
			}

			err := router.CreateRoute(expectedRoute.InterfaceIndex, expectedRoute.DestinationSubnet, expectedRoute.GatewayAddress)
			if err != nil {
				log.Errorf("Error creating route: %v", err)
				continue
			}

			n.addToRouteList(expectedRoute)

		case subnet.EventRemoved:
			log.Infof("Subnet removed: %v", leaseSubnet)

			existingRoutes, _ := router.GetRoutesFromInterfaceToSubnet(expectedRoute.InterfaceIndex, expectedRoute.DestinationSubnet)
			if len(existingRoutes) > 0 {
				existingRoute := existingRoutes[0]
				if existingRoute.Equal(*expectedRoute) {
					log.Infof("Removing existing route %v via %v", leaseSubnet, existingRoute.GatewayAddress)

					err := router.DeleteRoute(existingRoute.InterfaceIndex, existingRoute.DestinationSubnet, existingRoute.GatewayAddress)
					if err != nil {
						log.Warningf("Error removing route: %v", err)
					}
				}
			}

			n.removeFromRouteList(expectedRoute)

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}

func (n *RouteNetwork) addToRouteList(newRoute *routing.Route) {
	for _, route := range n.routes {
		if route.Equal(*newRoute) {
			return
		}
	}

	n.routes = append(n.routes, *newRoute)
}

func (n *RouteNetwork) removeFromRouteList(oldRoute *routing.Route) {
	for index, route := range n.routes {
		if route.Equal(*oldRoute) {
			n.routes = append(n.routes[:index], n.routes[index+1:]...)
			return
		}
	}
}

func (n *RouteNetwork) routeCheck(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(routeCheckRetries * time.Second):
			n.checkSubnetExistInRoutes()
		}
	}
}

func (n *RouteNetwork) checkSubnetExistInRoutes() {
	router := routing.RouterWindows{}

	existingRoutes, err := router.GetAllRoutes()
	if err != nil {
		log.Errorf("Error enumerating routes: %v", err)
		return
	}
	for _, expectedRoute := range n.routes {
		exist := false
		for _, existingRoute := range existingRoutes {
			if expectedRoute.Equal(existingRoute) {
				exist = true
				break
			}
		}

		if !exist {
			err := router.CreateRoute(expectedRoute.InterfaceIndex, expectedRoute.DestinationSubnet, expectedRoute.GatewayAddress)
			if err != nil {
				log.Warningf("Error recovering route to %v via %v on %v (%v).", expectedRoute.DestinationSubnet, expectedRoute.GatewayAddress, expectedRoute.InterfaceIndex, err)
				continue
			}
			log.Infof("Recovered route to %v via %v on %v.", expectedRoute.DestinationSubnet, expectedRoute.GatewayAddress, expectedRoute.InterfaceIndex)
		}
	}
}
