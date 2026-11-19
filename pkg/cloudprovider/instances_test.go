package cloudprovider

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/wrangler/v3/pkg/generic"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	cloudprovider "k8s.io/cloud-provider"
)

func Test_UnitK3sInstanceMetadata(t *testing.T) {
	nodeName := "test-node"
	nodeInternalIP := "10.0.0.1"
	nodeExternalIP := "1.2.3.4"

	tests := []struct {
		name    string
		node    *corev1.Node
		want    *cloudprovider.InstanceMetadata
		wantErr bool
	}{
		{
			name:    "No Annotations",
			node:    &corev1.Node{},
			wantErr: true,
		},
		{
			name: "Internal IP",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					Annotations: map[string]string{
						InternalIPKey: nodeInternalIP,
					},
				},
			},
			want: &cloudprovider.InstanceMetadata{
				InstanceType: version.Program,
				ProviderID:   version.Program + "://" + nodeName,
				NodeAddresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: nodeInternalIP},
				},
			},
		},
		{
			name: "Internal IP, External IP",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					Annotations: map[string]string{
						InternalIPKey: nodeInternalIP,
						ExternalIPKey: nodeExternalIP,
					},
				},
			},
			want: &cloudprovider.InstanceMetadata{
				InstanceType: version.Program,
				ProviderID:   version.Program + "://" + nodeName,
				NodeAddresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: nodeInternalIP},
					{Type: corev1.NodeExternalIP, Address: nodeExternalIP},
				},
			},
		},
		{
			name: "Internal IP, External IP, Hostname",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					Annotations: map[string]string{
						InternalIPKey: nodeInternalIP,
						ExternalIPKey: nodeExternalIP,
						HostnameKey:   nodeName + ".example.com",
					},
				},
			},
			want: &cloudprovider.InstanceMetadata{
				InstanceType: version.Program,
				ProviderID:   version.Program + "://" + nodeName,
				NodeAddresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: nodeInternalIP},
					{Type: corev1.NodeExternalIP, Address: nodeExternalIP},
					{Type: corev1.NodeHostName, Address: nodeName + ".example.com"},
				},
			},
		},
		{
			name: "Custom Metadata",
			node: &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: nodeName,
					Annotations: map[string]string{
						InternalIPKey: nodeInternalIP,
					},
					Labels: map[string]string{
						corev1.LabelInstanceTypeStable: "test.t1",
						corev1.LabelTopologyRegion:     "region",
						corev1.LabelTopologyZone:       "zone",
					},
				},
				Spec: corev1.NodeSpec{
					ProviderID: "test://i-abc",
				},
			},
			want: &cloudprovider.InstanceMetadata{
				InstanceType: "test.t1",
				ProviderID:   "test://i-abc",
				NodeAddresses: []corev1.NodeAddress{
					{Type: corev1.NodeInternalIP, Address: nodeInternalIP},
				},
				Region: "region",
				Zone:   "zone",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k := &k3s{}
			got, err := k.InstanceMetadata(context.Background(), tt.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("k3s.InstanceMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("k3s.InstanceMetadata() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitK3sInstanceMetadataRejectDuplicateAddresses(t *testing.T) {
	existing := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-1", CreationTimestamp: metav1.NewTime(time.Now().Add(-time.Hour))},
		Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{
			{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
		}},
	}
	nodeIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if err := nodeIndexer.Add(existing); err != nil {
		t.Fatalf("failed to seed node cache: %v", err)
	}
	k := &k3s{
		Config:    Config{NodeEnabled: true, RejectDuplicateNodeAddresses: true},
		nodeCache: generic.NewNonNamespacedCache[*corev1.Node](nodeIndexer, corev1.Resource("nodes")),
	}

	newNode := func(internalIP string) *corev1.Node {
		return &corev1.Node{ObjectMeta: metav1.ObjectMeta{
			Name:              "agent-2",
			CreationTimestamp: metav1.Now(),
			Annotations:       map[string]string{InternalIPKey: internalIP},
		}}
	}

	// a new node asking for an address another node already has is refused
	if _, err := k.InstanceMetadata(context.Background(), newNode("10.0.0.1")); err == nil {
		t.Errorf("k3s.InstanceMetadata() with a duplicate address: expected an error")
	}

	// a unique address is set as usual
	got, err := k.InstanceMetadata(context.Background(), newNode("10.0.0.2"))
	if err != nil {
		t.Fatalf("k3s.InstanceMetadata() with a unique address: %v", err)
	}
	want := []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.2"}}
	if !reflect.DeepEqual(got.NodeAddresses, want) {
		t.Errorf("k3s.InstanceMetadata() addresses = %+v, want %+v", got.NodeAddresses, want)
	}

	// the node that had the address first keeps getting it
	if _, err := k.InstanceMetadata(context.Background(), &corev1.Node{ObjectMeta: metav1.ObjectMeta{
		Name:              existing.Name,
		CreationTimestamp: existing.CreationTimestamp,
		Annotations:       map[string]string{InternalIPKey: "10.0.0.1"},
	}}); err != nil {
		t.Errorf("k3s.InstanceMetadata() for the original owner: %v", err)
	}
}
