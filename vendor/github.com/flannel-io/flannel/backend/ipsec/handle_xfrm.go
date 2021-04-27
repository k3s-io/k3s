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
// +build !windows

package ipsec

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/flannel-io/flannel/subnet"
	"github.com/vishvananda/netlink"
	log "k8s.io/klog"
)

func AddXFRMPolicy(myLease, remoteLease *subnet.Lease, dir netlink.Dir, reqID int) error {
	src := myLease.Subnet.ToIPNet()

	dst := remoteLease.Subnet.ToIPNet()

	policy := &netlink.XfrmPolicy{
		Src: src,
		Dst: dst,
		Dir: dir,
	}

	tunnelLeft := myLease.Attrs.PublicIP.ToIP()
	tunnelRight := remoteLease.Attrs.PublicIP.ToIP()

	tmpl := netlink.XfrmPolicyTmpl{
		Src:   tunnelLeft,
		Dst:   tunnelRight,
		Proto: netlink.XFRM_PROTO_ESP,
		Mode:  netlink.XFRM_MODE_TUNNEL,
		Reqid: reqID,
	}

	policy.Tmpls = append(policy.Tmpls, tmpl)

	if existingPolicy, err := netlink.XfrmPolicyGet(policy); err != nil {
		if errors.Is(err, syscall.ENOENT) {
			log.Infof("Adding ipsec policy: %+v", tmpl)
			if err := netlink.XfrmPolicyAdd(policy); err != nil {
				return fmt.Errorf("error adding policy: %+v err: %v", policy, err)
			}
		} else {
			return fmt.Errorf("error getting policy: %+v err: %v", policy, err)
		}
	} else {
		log.Infof("Updating ipsec policy %+v with %+v", existingPolicy, policy)
		if err := netlink.XfrmPolicyUpdate(policy); err != nil {
			return fmt.Errorf("error updating policy: %+v err: %v", policy, err)
		}
	}
	return nil
}

func DeleteXFRMPolicy(localSubnet, remoteSubnet *net.IPNet, localPublicIP, remotePublicIP net.IP, dir netlink.Dir, reqID int) error {
	src := localSubnet
	dst := remoteSubnet

	policy := netlink.XfrmPolicy{
		Src: src,
		Dst: dst,
		Dir: dir,
	}

	tunnelLeft := localPublicIP
	tunnelRight := remotePublicIP

	tmpl := netlink.XfrmPolicyTmpl{
		Src:   tunnelLeft,
		Dst:   tunnelRight,
		Proto: netlink.XFRM_PROTO_ESP,
		Mode:  netlink.XFRM_MODE_TUNNEL,
		Reqid: reqID,
	}

	log.Infof("Deleting ipsec policy: %+v", tmpl)

	policy.Tmpls = append(policy.Tmpls, tmpl)

	if err := netlink.XfrmPolicyDel(&policy); err != nil {
		return fmt.Errorf("error deleting policy: %+v err: %v", policy, err)
	}

	return nil
}
