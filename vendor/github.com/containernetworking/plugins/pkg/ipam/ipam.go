// Copyright 2015 CNI authors
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

package ipam

import (
	"context"
	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/types"
)

func ExecAdd(plugin string, netconf []byte) (types.Result, error) {
	return invoke.DelegateAdd(context.TODO(), plugin, netconf, nil)
}

func ExecCheck(plugin string, netconf []byte) error {
	return invoke.DelegateCheck(context.TODO(), plugin, netconf, nil)
}

func ExecDel(plugin string, netconf []byte) error {
	return invoke.DelegateDel(context.TODO(), plugin, netconf, nil)
}
