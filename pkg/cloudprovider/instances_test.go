package cloudprovider

import (
	"context"
	"reflect"
	"testing"

	"github.com/k3s-io/k3s/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
