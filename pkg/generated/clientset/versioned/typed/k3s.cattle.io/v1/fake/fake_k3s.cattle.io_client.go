/*
Copyright The Kubernetes Authors.

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

// Code generated by main. DO NOT EDIT.

package fake

import (
	v1 "github.com/k3s-io/k3s/pkg/generated/clientset/versioned/typed/k3s.cattle.io/v1"
	rest "k8s.io/client-go/rest"
	testing "k8s.io/client-go/testing"
)

type FakeK3sV1 struct {
	*testing.Fake
}

func (c *FakeK3sV1) Addons(namespace string) v1.AddonInterface {
	return &FakeAddons{c, namespace}
}

func (c *FakeK3sV1) ETCDSnapshotFiles() v1.ETCDSnapshotFileInterface {
	return &FakeETCDSnapshotFiles{c}
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *FakeK3sV1) RESTClient() rest.Interface {
	var ret *rest.RESTClient
	return ret
}
