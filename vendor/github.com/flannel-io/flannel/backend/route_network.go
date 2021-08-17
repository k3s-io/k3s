// +build !windows

// Copyright 2017 flannel authors
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
	"bytes"
	"net"
	"sync"
	"time"

	"github.com/flannel-io/flannel/subnet"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"
	log "k8s.io/klog"
)

const (
	routeCheckRetries = 10
)

type RouteNetwork struct {
	SimpleNetwork
	BackendType string
	routes      []netlink.Route
	v6Routes    []netlink.Route
	SM          subnet.Manager
	GetRoute    func(lease *subnet.Lease) *netlink.Route
	GetV6Route  func(lease *subnet.Lease) *netlink.Route
	Mtu         int
	LinkIndex   int
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

	n.routes = make([]netlink.Route, 0, 10)
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
	for _, evt := range batch {
		switch evt.Type {
		case subnet.EventAdded:
			if evt.Lease.Attrs.BackendType != n.BackendType {
				log.Warningf("Ignoring non-%v subnet: type=%v", n.BackendType, evt.Lease.Attrs.BackendType)
				continue
			}

			if evt.Lease.EnableIPv4 {
				log.Infof("Subnet added: %v via %v", evt.Lease.Subnet, evt.Lease.Attrs.PublicIP)

				route := n.GetRoute(&evt.Lease)
				routeAdd(route, netlink.FAMILY_V4, n.addToRouteList, n.removeFromV4RouteList)
			}

			if evt.Lease.EnableIPv6 {
				log.Infof("Subnet added: %v via %v", evt.Lease.IPv6Subnet, evt.Lease.Attrs.PublicIPv6)

				route := n.GetV6Route(&evt.Lease)
				routeAdd(route, netlink.FAMILY_V6, n.addToV6RouteList, n.removeFromV6RouteList)
			}

		case subnet.EventRemoved:
			if evt.Lease.Attrs.BackendType != n.BackendType {
				log.Warningf("Ignoring non-%v subnet: type=%v", n.BackendType, evt.Lease.Attrs.BackendType)
				continue
			}

			if evt.Lease.EnableIPv4 {
				log.Info("Subnet removed: ", evt.Lease.Subnet)

				route := n.GetRoute(&evt.Lease)
				// Always remove the route from the route list.
				n.removeFromV4RouteList(*route)

				if err := netlink.RouteDel(route); err != nil {
					log.Errorf("Error deleting route to %v: %v", evt.Lease.Subnet, err)
				}
			}

			if evt.Lease.EnableIPv6 {
				log.Info("Subnet removed: ", evt.Lease.IPv6Subnet)

				route := n.GetV6Route(&evt.Lease)
				// Always remove the route from the route list.
				n.removeFromV6RouteList(*route)

				if err := netlink.RouteDel(route); err != nil {
					log.Errorf("Error deleting route to %v: %v", evt.Lease.IPv6Subnet, err)
				}
			}

		default:
			log.Error("Internal error: unknown event type: ", int(evt.Type))
		}
	}
}

func routeAdd(route *netlink.Route, ipFamily int, addToRouteList, removeFromRouteList func(netlink.Route)) {
	addToRouteList(*route)
	// Check if route exists before attempting to add it
	routeList, err := netlink.RouteListFiltered(ipFamily, &netlink.Route{Dst: route.Dst}, netlink.RT_FILTER_DST)
	if err != nil {
		log.Warningf("Unable to list routes: %v", err)
	}

	if len(routeList) > 0 && !routeEqual(routeList[0], *route) {
		// Same Dst different Gw or different link index. Remove it, correct route will be added below.
		log.Warningf("Replacing existing route to %v with %v", routeList[0], route)
		if err := netlink.RouteDel(&routeList[0]); err != nil {
			log.Errorf("Effor deleteing route to %v: %v", routeList[0].Dst, err)
			return
		}
		removeFromRouteList(routeList[0])
	}
	routeList, err = netlink.RouteListFiltered(ipFamily, &netlink.Route{Dst: route.Dst}, netlink.RT_FILTER_DST)
	if err != nil {
		log.Warningf("Unable to list routes: %v", err)
	}

	if len(routeList) > 0 && routeEqual(routeList[0], *route) {
		// Same Dst and same Gw, keep it and do not attempt to add it.
		log.Infof("Route to %v already exists, skipping.", route)
	} else if err := netlink.RouteAdd(route); err != nil {
		log.Errorf("Error adding route to %v", route)
		return
	}
	routeList, err = netlink.RouteListFiltered(ipFamily, &netlink.Route{Dst: route.Dst}, netlink.RT_FILTER_DST)
	if err != nil {
		log.Warningf("Unable to list routes: %v", err)
	}
}

func (n *RouteNetwork) addToRouteList(route netlink.Route) {
	n.routes = addToRouteList(&route, n.routes)
}

func (n *RouteNetwork) addToV6RouteList(route netlink.Route) {
	n.v6Routes = addToRouteList(&route, n.v6Routes)
}

func addToRouteList(route *netlink.Route, routes []netlink.Route) []netlink.Route {
	for _, r := range routes {
		if routeEqual(r, *route) {
			return routes
		}
	}
	return append(routes, *route)
}

func (n *RouteNetwork) removeFromV4RouteList(route netlink.Route) {
	n.routes = n.removeFromRouteList(&route, n.routes)
}

func (n *RouteNetwork) removeFromV6RouteList(route netlink.Route) {
	n.v6Routes = n.removeFromRouteList(&route, n.v6Routes)
}

func (n *RouteNetwork) removeFromRouteList(route *netlink.Route, routes []netlink.Route) []netlink.Route {
	for index, r := range routes {
		if routeEqual(r, *route) {
			routes = append(routes[:index], routes[index+1:]...)
			return routes
		}
	}
	return routes
}

func (n *RouteNetwork) routeCheck(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(routeCheckRetries * time.Second):
			n.checkSubnetExistInV4Routes()
			n.checkSubnetExistInV6Routes()
		}
	}
}

func (n *RouteNetwork) checkSubnetExistInV4Routes() {
	n.checkSubnetExistInRoutes(n.routes, netlink.FAMILY_V4)
}

func (n *RouteNetwork) checkSubnetExistInV6Routes() {
	n.checkSubnetExistInRoutes(n.v6Routes, netlink.FAMILY_V6)
}

func (n *RouteNetwork) checkSubnetExistInRoutes(routes []netlink.Route, ipFamily int) {
	routeList, err := netlink.RouteList(nil, ipFamily)
	if err == nil {
		for _, route := range routes {
			exist := false
			for _, r := range routeList {
				if r.Dst == nil {
					continue
				}
				if routeEqual(r, route) {
					exist = true
					break
				}
			}

			if !exist {
				if err := netlink.RouteAdd(&route); err != nil {
					if nerr, ok := err.(net.Error); !ok {
						log.Errorf("Error recovering route to %v: %v, %v", route.Dst, route.Gw, nerr)
					}
					continue
				} else {
					log.Infof("Route recovered %v : %v", route.Dst, route.Gw)
				}
			}
		}
	} else {
		log.Errorf("Error fetching route list. Will automatically retry: %v", err)
	}
}

func routeEqual(x, y netlink.Route) bool {
	// For ipip backend, when enabling directrouting, link index of some routes may change
	// For both ipip and host-gw backend, link index may also change if updating ExtIface
	if x.Dst.IP.Equal(y.Dst.IP) && x.Gw.Equal(y.Gw) && bytes.Equal(x.Dst.Mask, y.Dst.Mask) && x.LinkIndex == y.LinkIndex {
		return true
	}
	return false
}
