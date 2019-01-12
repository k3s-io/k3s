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
	"golang.org/x/net/context"

	"github.com/coreos/flannel/subnet"
)

type SimpleNetwork struct {
	SubnetLease *subnet.Lease
	ExtIface    *ExternalInterface
}

func (n *SimpleNetwork) Lease() *subnet.Lease {
	return n.SubnetLease
}

func (n *SimpleNetwork) MTU() int {
	return n.ExtIface.Iface.MTU
}

func (_ *SimpleNetwork) Run(ctx context.Context) {
	<-ctx.Done()
}
