// Copyright 2015 flannel authors
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
// +build !windows

package vxlan

import (
	"encoding/json"
	"net"
	"sync"
	"syscall"

	"github.com/flannel-io/flannel/backend"
	"github.com/flannel-io/flannel/pkg/ip"
	"github.com/flannel-io/flannel/subnet"
	"github.com/vishvananda/netlink"
	"golang.org/x/net/context"
	log "k8s.io/klog"
)

type network struct {
	backend.SimpleNetwork
	dev       *vxlanDevice
	v6Dev     *vxlanDevice
	subnetMgr subnet.Manager
}

const (
	encapOverhead = 50
)

func newNetwork(subnetMgr subnet.Manager, extIface *backend.ExternalInterface, dev *vxlanDevice, v6Dev *vxlanDevice, _ ip.IP4Net, lease *subnet.Lease) (*network, error) {
	nw := &network{
		SimpleNetwork: backend.SimpleNetwork{
			SubnetLease: lease,
			ExtIface:    extIface,
		},
		subnetMgr: subnetMgr,
		dev:       dev,
		v6Dev:     v6Dev,
	}

	return nw, nil
}

func (nw *network) Run(ctx context.Context) {
	wg := sync.WaitGroup{}

	log.V(0).Info("watching for new subnet leases")
	events := make(chan []subnet.Event)
	wg.Add(1)
	go func() {
		subnet.WatchLeases(ctx, nw.subnetMgr, nw.SubnetLease, events)
		log.V(1).Info("WatchLeases exited")
		wg.Done()
	}()

	defer wg.Wait()

	for {
		select {
		case evtBatch, ok := <-events:
			if !ok {
				log.Infof("evts chan closed")
				return
			}
			nw.handleSubnetEvents(evtBatch)
		}
	}
}

func (nw *network) MTU() int {
	return nw.ExtIface.Iface.MTU - encapOverhead
}

type vxlanLeaseAttrs struct {
	VNI     uint16
	VtepMAC hardwareAddr
}

func (nw *network) handleSubnetEvents(batch []subnet.Event) {
	for _, event := range batch {
		sn := event.Lease.Subnet
		v6Sn := event.Lease.IPv6Subnet
		attrs := event.Lease.Attrs
		if attrs.BackendType != "vxlan" {
			log.Warningf("ignoring non-vxlan v4Subnet(%s) v6Subnet(%s): type=%v", sn, v6Sn, attrs.BackendType)
			continue
		}

		var (
			vxlanAttrs, v6VxlanAttrs           vxlanLeaseAttrs
			directRoutingOK, v6DirectRoutingOK bool
			directRoute, v6DirectRoute         netlink.Route
			vxlanRoute, v6VxlanRoute           netlink.Route
		)

		if event.Lease.EnableIPv4 && nw.dev != nil {
			if err := json.Unmarshal(attrs.BackendData, &vxlanAttrs); err != nil {
				log.Error("error decoding subnet lease JSON: ", err)
				continue
			}

			// This route is used when traffic should be vxlan encapsulated
			vxlanRoute = netlink.Route{
				LinkIndex: nw.dev.link.Attrs().Index,
				Scope:     netlink.SCOPE_UNIVERSE,
				Dst:       sn.ToIPNet(),
				Gw:        sn.IP.ToIP(),
			}
			vxlanRoute.SetFlag(syscall.RTNH_F_ONLINK)

			// directRouting is where the remote host is on the same subnet so vxlan isn't required.
			directRoute = netlink.Route{
				Dst: sn.ToIPNet(),
				Gw:  attrs.PublicIP.ToIP(),
			}
			if nw.dev.directRouting {
				if dr, err := ip.DirectRouting(attrs.PublicIP.ToIP()); err != nil {
					log.Error(err)
				} else {
					directRoutingOK = dr
				}
			}
		}

		if event.Lease.EnableIPv6 && nw.v6Dev != nil {
			if err := json.Unmarshal(attrs.BackendV6Data, &v6VxlanAttrs); err != nil {
				log.Error("error decoding v6 subnet lease JSON: ", err)
				continue
			}
			if v6Sn.IP != nil && nw.v6Dev != nil {
				v6VxlanRoute = netlink.Route{
					LinkIndex: nw.v6Dev.link.Attrs().Index,
					Scope:     netlink.SCOPE_UNIVERSE,
					Dst:       v6Sn.ToIPNet(),
					Gw:        v6Sn.IP.ToIP(),
				}
				v6VxlanRoute.SetFlag(syscall.RTNH_F_ONLINK)

				// directRouting is where the remote host is on the same subnet so vxlan isn't required.
				v6DirectRoute = netlink.Route{
					Dst: v6Sn.ToIPNet(),
					Gw:  attrs.PublicIPv6.ToIP(),
				}

				if nw.v6Dev.directRouting {
					if v6Dr, err := ip.DirectRouting(attrs.PublicIPv6.ToIP()); err != nil {
						log.Error(err)
					} else {
						v6DirectRoutingOK = v6Dr
					}
				}
			}
		}

		switch event.Type {
		case subnet.EventAdded:
			if event.Lease.EnableIPv4 {
				if directRoutingOK {
					log.V(2).Infof("Adding direct route to subnet: %s PublicIP: %s", sn, attrs.PublicIP)

					if err := netlink.RouteReplace(&directRoute); err != nil {
						log.Errorf("Error adding route to %v via %v: %v", sn, attrs.PublicIP, err)
						continue
					}
				} else {
					log.V(2).Infof("adding subnet: %s PublicIP: %s VtepMAC: %s", sn, attrs.PublicIP, net.HardwareAddr(vxlanAttrs.VtepMAC))
					if err := nw.dev.AddARP(neighbor{IP: sn.IP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)}); err != nil {
						log.Error("AddARP failed: ", err)
						continue
					}

					if err := nw.dev.AddFDB(neighbor{IP: attrs.PublicIP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)}); err != nil {
						log.Error("AddFDB failed: ", err)

						// Try to clean up the ARP entry then continue
						if err := nw.dev.DelARP(neighbor{IP: event.Lease.Subnet.IP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)}); err != nil {
							log.Error("DelARP failed: ", err)
						}

						continue
					}

					// Set the route - the kernel would ARP for the Gw IP address if it hadn't already been set above so make sure
					// this is done last.
					if err := netlink.RouteReplace(&vxlanRoute); err != nil {
						log.Errorf("failed to add vxlanRoute (%s -> %s): %v", vxlanRoute.Dst, vxlanRoute.Gw, err)

						// Try to clean up both the ARP and FDB entries then continue
						if err := nw.dev.DelARP(neighbor{IP: event.Lease.Subnet.IP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)}); err != nil {
							log.Error("DelARP failed: ", err)
						}

						if err := nw.dev.DelFDB(neighbor{IP: event.Lease.Attrs.PublicIP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)}); err != nil {
							log.Error("DelFDB failed: ", err)
						}

						continue
					}
				}
			}
			if event.Lease.EnableIPv6 {
				if v6DirectRoutingOK {
					log.V(2).Infof("Adding v6 direct route to v6 subnet: %s PublicIPv6: %s", v6Sn, attrs.PublicIPv6)

					if err := netlink.RouteReplace(&v6DirectRoute); err != nil {
						log.Errorf("Error adding v6 route to %v via %v: %v", v6Sn, attrs.PublicIPv6, err)
						continue
					}
				} else {
					log.V(2).Infof("adding v6 subnet: %s PublicIPv6: %s VtepMAC: %s", v6Sn, attrs.PublicIPv6, net.HardwareAddr(v6VxlanAttrs.VtepMAC))
					if err := nw.v6Dev.AddV6ARP(neighbor{IP6: v6Sn.IP, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)}); err != nil {
						log.Error("AddV6ARP failed: ", err)
						continue
					}

					if err := nw.v6Dev.AddV6FDB(neighbor{IP6: attrs.PublicIPv6, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)}); err != nil {
						log.Error("AddV6FDB failed: ", err)

						// Try to clean up the ARP entry then continue
						if err := nw.v6Dev.DelV6ARP(neighbor{IP6: event.Lease.IPv6Subnet.IP, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)}); err != nil {
							log.Error("DelV6ARP failed: ", err)
						}

						continue
					}

					// Set the route - the kernel would ARP for the Gw IP address if it hadn't already been set above so make sure
					// this is done last.
					if err := netlink.RouteReplace(&v6VxlanRoute); err != nil {
						log.Errorf("failed to add v6 vxlanRoute (%s -> %s): %v", v6VxlanRoute.Dst, v6VxlanRoute.Gw, err)

						// Try to clean up both the ARP and FDB entries then continue
						if err := nw.v6Dev.DelV6ARP(neighbor{IP6: event.Lease.IPv6Subnet.IP, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)}); err != nil {
							log.Error("DelV6ARP failed: ", err)
						}

						if err := nw.v6Dev.DelV6FDB(neighbor{IP6: event.Lease.Attrs.PublicIPv6, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)}); err != nil {
							log.Error("DelV6FDB failed: ", err)
						}

						continue
					}
				}
			}
		case subnet.EventRemoved:
			if event.Lease.EnableIPv4 {
				if directRoutingOK {
					log.V(2).Infof("Removing direct route to subnet: %s PublicIP: %s", sn, attrs.PublicIP)
					if err := netlink.RouteDel(&directRoute); err != nil {
						log.Errorf("Error deleting route to %v via %v: %v", sn, attrs.PublicIP, err)
					}
				} else {
					log.V(2).Infof("removing subnet: %s PublicIP: %s VtepMAC: %s", sn, attrs.PublicIP, net.HardwareAddr(vxlanAttrs.VtepMAC))

					// Try to remove all entries - don't bail out if one of them fails.
					if err := nw.dev.DelARP(neighbor{IP: sn.IP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)}); err != nil {
						log.Error("DelARP failed: ", err)
					}

					if err := nw.dev.DelFDB(neighbor{IP: attrs.PublicIP, MAC: net.HardwareAddr(vxlanAttrs.VtepMAC)}); err != nil {
						log.Error("DelFDB failed: ", err)
					}

					if err := netlink.RouteDel(&vxlanRoute); err != nil {
						log.Errorf("failed to delete vxlanRoute (%s -> %s): %v", vxlanRoute.Dst, vxlanRoute.Gw, err)
					}
				}
			}
			if event.Lease.EnableIPv6 {
				if v6DirectRoutingOK {
					log.V(2).Infof("Removing v6 direct route to subnet: %s PublicIP: %s", sn, attrs.PublicIPv6)
					if err := netlink.RouteDel(&directRoute); err != nil {
						log.Errorf("Error deleting v6 route to %v via %v: %v", v6Sn, attrs.PublicIPv6, err)
					}
				} else {
					log.V(2).Infof("removing v6subnet: %s PublicIPv6: %s VtepMAC: %s", v6Sn, attrs.PublicIPv6, net.HardwareAddr(v6VxlanAttrs.VtepMAC))

					// Try to remove all entries - don't bail out if one of them fails.
					if err := nw.v6Dev.DelV6ARP(neighbor{IP6: v6Sn.IP, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)}); err != nil {
						log.Error("DelV6ARP failed: ", err)
					}

					if err := nw.v6Dev.DelV6FDB(neighbor{IP6: attrs.PublicIPv6, MAC: net.HardwareAddr(v6VxlanAttrs.VtepMAC)}); err != nil {
						log.Error("DelV6FDB failed: ", err)
					}

					if err := netlink.RouteDel(&v6VxlanRoute); err != nil {
						log.Errorf("failed to delete v6 vxlanRoute (%s -> %s): %v", v6VxlanRoute.Dst, v6VxlanRoute.Gw, err)
					}
				}
			}
		default:
			log.Error("internal error: unknown event type: ", int(event.Type))
		}
	}
}
