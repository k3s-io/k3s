// Copyright 2017 CNI authors
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

package ip

import (
	"fmt"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
)

const SETTLE_INTERVAL = 50 * time.Millisecond

// SettleAddresses waits for all addresses on a link to leave tentative state.
// This is particularly useful for ipv6, where all addresses need to do DAD.
// There is no easy way to wait for this as an event, so just loop until the
// addresses are no longer tentative.
// If any addresses are still tentative after timeout seconds, then error.
func SettleAddresses(ifName string, timeout int) error {
	link, err := netlink.LinkByName(ifName)
	if err != nil {
		return fmt.Errorf("failed to retrieve link: %v", err)
	}

	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for {
		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		if err != nil {
			return fmt.Errorf("could not list addresses: %v", err)
		}

		if len(addrs) == 0 {
			return nil
		}

		ok := true
		for _, addr := range addrs {
			if addr.Flags&(syscall.IFA_F_TENTATIVE|syscall.IFA_F_DADFAILED) > 0 {
				ok = false
				break // Break out of the `range addrs`, not the `for`
			}
		}

		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("link %s still has tentative addresses after %d seconds",
				ifName,
				timeout)
		}

		time.Sleep(SETTLE_INTERVAL)
	}
}
