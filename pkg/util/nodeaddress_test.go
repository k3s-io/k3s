package util

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_UnitDuplicateNodeAddress(t *testing.T) {
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	node := func(name string, age time.Duration, addresses ...v1.NodeAddress) *v1.Node {
		return &v1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: name, CreationTimestamp: metav1.NewTime(created.Add(-age))},
			Status:     v1.NodeStatus{Addresses: addresses},
		}
	}
	nodes := []*v1.Node{
		node("server-1", 3*time.Hour,
			v1.NodeAddress{Type: v1.NodeInternalIP, Address: "10.0.0.1"},
			v1.NodeAddress{Type: v1.NodeExternalIP, Address: "2001:db8::1"},
			v1.NodeAddress{Type: v1.NodeHostName, Address: "server-1"},
		),
		node("agent-1", 2*time.Hour, v1.NodeAddress{Type: v1.NodeInternalIP, Address: "10.0.0.2"}),
		// a newer node that was registered with agent-1's address
		node("agent-3", time.Hour, v1.NodeAddress{Type: v1.NodeInternalIP, Address: "10.0.0.2"}),
	}

	tests := []struct {
		name      string
		nodeName  string
		addresses []string
		wantAddr  string
		wantOwner string
	}{
		{name: "unique address", nodeName: "agent-2", addresses: []string{"10.0.0.3"}},
		{name: "internal IP of another node", nodeName: "agent-2", addresses: []string{"10.0.0.3", "10.0.0.2"}, wantAddr: "10.0.0.2", wantOwner: "agent-1"},
		{name: "external IP in another form", nodeName: "agent-2", addresses: []string{"2001:db8:0::1"}, wantAddr: "2001:db8:0::1", wantOwner: "server-1"},
		{name: "older node keeps its address", nodeName: "agent-1", addresses: []string{"10.0.0.2"}},
		{name: "newer node is refused", nodeName: "agent-3", addresses: []string{"10.0.0.2"}, wantAddr: "10.0.0.2", wantOwner: "agent-1"},
		{name: "loopback is ignored", nodeName: "agent-2", addresses: []string{"127.0.0.1", "::1"}},
		{name: "hostname addresses are ignored", nodeName: "agent-2", addresses: []string{"server-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addr, owner := DuplicateNodeAddress(nodes, tt.nodeName, tt.addresses)
			if addr != tt.wantAddr || owner != tt.wantOwner {
				t.Errorf("DuplicateNodeAddress() = %q, %q, want %q, %q", addr, owner, tt.wantAddr, tt.wantOwner)
			}
		})
	}
}
