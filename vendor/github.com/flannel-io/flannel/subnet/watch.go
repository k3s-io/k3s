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

package subnet

import (
	"time"

	"github.com/flannel-io/flannel/pkg/ip"
	"golang.org/x/net/context"
	log "k8s.io/klog"
)

// WatchLeases performs a long term watch of the given network's subnet leases
// and communicates addition/deletion events on receiver channel. It takes care
// of handling "fall-behind" logic where the history window has advanced too far
// and it needs to diff the latest snapshot with its saved state and generate events
func WatchLeases(ctx context.Context, sm Manager, ownLease *Lease, receiver chan []Event) {
	lw := &leaseWatcher{
		ownLease: ownLease,
	}
	var cursor interface{}

	for {
		res, err := sm.WatchLeases(ctx, cursor)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				log.Infof("%v, close receiver chan", err)
				close(receiver)
				return
			}

			if res.Cursor != nil {
				cursor = res.Cursor
			}

			log.Errorf("Watch subnets: %v", err)
			time.Sleep(time.Second)
			continue
		}

		cursor = res.Cursor

		var batch []Event

		if len(res.Events) > 0 {
			batch = lw.update(res.Events)
		} else {
			batch = lw.reset(res.Snapshot)
		}

		if len(batch) > 0 {
			receiver <- batch
		}
	}
}

type leaseWatcher struct {
	ownLease *Lease
	leases   []Lease
}

func (lw *leaseWatcher) reset(leases []Lease) []Event {
	batch := []Event{}

	for _, nl := range leases {
		if lw.ownLease != nil && nl.EnableIPv4 && !nl.EnableIPv6 &&
			nl.Subnet.Equal(lw.ownLease.Subnet) {
			continue
		} else if lw.ownLease != nil && !nl.EnableIPv4 && nl.EnableIPv6 &&
			nl.IPv6Subnet.Equal(lw.ownLease.IPv6Subnet) {
			continue
		} else if lw.ownLease != nil && nl.EnableIPv4 && nl.EnableIPv6 &&
			nl.Subnet.Equal(lw.ownLease.Subnet) &&
			nl.IPv6Subnet.Equal(lw.ownLease.IPv6Subnet) {
			continue
		} else if lw.ownLease != nil && !nl.EnableIPv4 && !nl.EnableIPv6 &&
			nl.Subnet.Equal(lw.ownLease.Subnet) {
			//TODO - dual-stack temporarily only compatible with kube subnet manager
			continue
		}

		found := false
		for i, ol := range lw.leases {
			if ol.EnableIPv4 && !ol.EnableIPv6 && ol.Subnet.Equal(nl.Subnet) {
				lw.leases = deleteLease(lw.leases, i)
				found = true
				break
			} else if ol.EnableIPv4 && !ol.EnableIPv6 && ol.IPv6Subnet.Equal(nl.IPv6Subnet) {
				lw.leases = deleteLease(lw.leases, i)
				found = true
				break
			} else if ol.EnableIPv4 && ol.EnableIPv6 && ol.Subnet.Equal(nl.Subnet) &&
				ol.IPv6Subnet.Equal(nl.IPv6Subnet) {
				lw.leases = deleteLease(lw.leases, i)
				found = true
				break
			} else if !ol.EnableIPv4 && !ol.EnableIPv6 && ol.Subnet.Equal(nl.Subnet) {
				//TODO - dual-stack temporarily only compatible with kube subnet manager
				lw.leases = deleteLease(lw.leases, i)
				found = true
				break
			}
		}

		if !found {
			// new lease
			batch = append(batch, Event{EventAdded, nl})
		}
	}

	// everything left in sm.leases has been deleted
	for _, l := range lw.leases {
		if lw.ownLease != nil && l.EnableIPv4 && !l.EnableIPv6 &&
			l.Subnet.Equal(lw.ownLease.Subnet) {
			continue
		} else if lw.ownLease != nil && !l.EnableIPv4 && l.EnableIPv6 &&
			l.IPv6Subnet.Equal(lw.ownLease.IPv6Subnet) {
			continue
		} else if lw.ownLease != nil && l.EnableIPv4 && l.EnableIPv6 &&
			l.Subnet.Equal(lw.ownLease.Subnet) &&
			l.IPv6Subnet.Equal(lw.ownLease.IPv6Subnet) {
			continue
		} else if lw.ownLease != nil && !l.EnableIPv4 && !l.EnableIPv6 &&
			l.Subnet.Equal(lw.ownLease.Subnet) {
			//TODO - dual-stack temporarily only compatible with kube subnet manager
			continue
		}
		batch = append(batch, Event{EventRemoved, l})
	}

	// copy the leases over (caution: don't just assign a slice)
	lw.leases = make([]Lease, len(leases))
	copy(lw.leases, leases)

	return batch
}

func (lw *leaseWatcher) update(events []Event) []Event {
	batch := []Event{}

	for _, e := range events {
		if lw.ownLease != nil && e.Lease.EnableIPv4 && !e.Lease.EnableIPv6 &&
			e.Lease.Subnet.Equal(lw.ownLease.Subnet) {
			continue
		} else if lw.ownLease != nil && !e.Lease.EnableIPv4 && e.Lease.EnableIPv6 &&
			e.Lease.IPv6Subnet.Equal(lw.ownLease.IPv6Subnet) {
			continue
		} else if lw.ownLease != nil && e.Lease.EnableIPv4 && e.Lease.EnableIPv6 &&
			e.Lease.Subnet.Equal(lw.ownLease.Subnet) &&
			e.Lease.IPv6Subnet.Equal(lw.ownLease.IPv6Subnet) {
			continue
		} else if lw.ownLease != nil && !e.Lease.EnableIPv4 && !e.Lease.EnableIPv6 &&
			e.Lease.Subnet.Equal(lw.ownLease.Subnet) {
			//TODO - dual-stack temporarily only compatible with kube subnet manager
			continue
		}

		switch e.Type {
		case EventAdded:
			batch = append(batch, lw.add(&e.Lease))

		case EventRemoved:
			batch = append(batch, lw.remove(&e.Lease))
		}
	}

	return batch
}

func (lw *leaseWatcher) add(lease *Lease) Event {
	for i, l := range lw.leases {
		if l.EnableIPv4 && !l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) {
			lw.leases[i] = *lease
			return Event{EventAdded, lw.leases[i]}
		} else if !l.EnableIPv4 && l.EnableIPv6 && l.IPv6Subnet.Equal(lease.IPv6Subnet) {
			lw.leases[i] = *lease
			return Event{EventAdded, lw.leases[i]}
		} else if l.EnableIPv4 && l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) &&
			l.IPv6Subnet.Equal(lease.IPv6Subnet) {
			lw.leases[i] = *lease
			return Event{EventAdded, lw.leases[i]}
		} else if !l.EnableIPv4 && !l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) {
			//TODO - dual-stack temporarily only compatible with kube subnet manager
			lw.leases[i] = *lease
			return Event{EventAdded, lw.leases[i]}
		}
	}
	lw.leases = append(lw.leases, *lease)

	return Event{EventAdded, lw.leases[len(lw.leases)-1]}
}

func (lw *leaseWatcher) remove(lease *Lease) Event {
	for i, l := range lw.leases {
		if l.EnableIPv4 && !l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) {
			lw.leases = deleteLease(lw.leases, i)
			return Event{EventRemoved, l}
		} else if !l.EnableIPv4 && l.EnableIPv6 && l.IPv6Subnet.Equal(lease.IPv6Subnet) {
			lw.leases = deleteLease(lw.leases, i)
			return Event{EventRemoved, l}
		} else if l.EnableIPv4 && l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) &&
			l.IPv6Subnet.Equal(lease.IPv6Subnet) {
			lw.leases = deleteLease(lw.leases, i)
			return Event{EventRemoved, l}
		} else if !l.EnableIPv4 && !l.EnableIPv6 && l.Subnet.Equal(lease.Subnet) {
			//TODO - dual-stack temporarily only compatible with kube subnet manager
			lw.leases = deleteLease(lw.leases, i)
			return Event{EventRemoved, l}
		}
	}

	log.Errorf("Removed subnet (%s) and ipv6 subnet (%s) were not found", lease.Subnet, lease.IPv6Subnet)
	return Event{EventRemoved, *lease}
}

func deleteLease(l []Lease, i int) []Lease {
	l = append(l[:i], l[i+1:]...)
	return l
}

// WatchLease performs a long term watch of the given network's subnet lease
// and communicates addition/deletion events on receiver channel. It takes care
// of handling "fall-behind" logic where the history window has advanced too far
// and it needs to diff the latest snapshot with its saved state and generate events
func WatchLease(ctx context.Context, sm Manager, sn ip.IP4Net, receiver chan Event) {
	var cursor interface{}

	for {
		wr, err := sm.WatchLease(ctx, sn, cursor)
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				log.Infof("%v, close receiver chan", err)
				close(receiver)
				return
			}

			log.Errorf("Subnet watch failed: %v", err)
			time.Sleep(time.Second)
			continue
		}

		if len(wr.Snapshot) > 0 {
			receiver <- Event{
				Type:  EventAdded,
				Lease: wr.Snapshot[0],
			}
		} else {
			receiver <- wr.Events[0]
		}

		cursor = wr.Cursor
	}
}
