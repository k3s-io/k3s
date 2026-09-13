package cloudprovider

import (
	"context"
	"math/rand"
	"reflect"
	"testing"

	"github.com/rancher/wrangler/v3/pkg/generic"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

const (
	addrv4   = "1.2.3.4"
	addrv4_2 = "2.3.4.5"
	addrv6   = "2001:db8::1"
	addrv6_2 = "3001:db8::1"
)

func Test_UnitFilterByIPFamily(t *testing.T) {
	type args struct {
		ips []string
		svc *core.Service
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "No IPFamily",
			args: args{
				ips: []string{addrv4, addrv6},
				svc: &core.Service{
					Spec: core.ServiceSpec{
						IPFamilies: []core.IPFamily{},
					},
				},
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "IPv4 Only",
			args: args{
				ips: []string{addrv4, addrv6},
				svc: &core.Service{
					Spec: core.ServiceSpec{
						IPFamilies: []core.IPFamily{core.IPv4Protocol},
					},
				},
			},
			want:    []string{addrv4},
			wantErr: false,
		},
		{
			name: "IPv6 Only",
			args: args{
				ips: []string{addrv4, addrv6},
				svc: &core.Service{
					Spec: core.ServiceSpec{
						IPFamilies: []core.IPFamily{core.IPv6Protocol},
					},
				},
			},
			want:    []string{addrv6},
			wantErr: false,
		},
		{
			name: "Dual-Stack",
			args: args{
				ips: []string{addrv4, addrv6},
				svc: &core.Service{
					Spec: core.ServiceSpec{
						IPFamilies: []core.IPFamily{core.IPv4Protocol, core.IPv6Protocol},
					},
				},
			},
			want:    []string{addrv4, addrv6},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := filterByIPFamily(tt.args.ips, tt.args.svc)
			if (err != nil) != tt.wantErr {
				t.Errorf("filterByIPFamily() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterByIPFamily() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitFilterByIPFamily_Ordering(t *testing.T) {
	want := []string{addrv4, addrv4_2, addrv6, addrv6_2}
	ips := []string{addrv4, addrv4_2, addrv6, addrv6_2}
	rand.Shuffle(len(ips), func(i, j int) {
		ips[i], ips[j] = ips[j], ips[i]
	})
	svc := &core.Service{
		Spec: core.ServiceSpec{
			IPFamilies: []core.IPFamily{core.IPv4Protocol, core.IPv6Protocol},
		},
	}

	got, _ := filterByIPFamily(ips, svc)

	if !reflect.DeepEqual(got, want) {
		t.Errorf("filterByIPFamily() = %+v\nWant = %+v", got, want)
	}
}

func Test_UnitNodePoolAffinity(t *testing.T) {
	tests := []struct {
		name string
		pool string
		want *core.Affinity
	}{
		{
			name: "No pool",
			pool: "",
			want: nil,
		},
		{
			name: "Named pool",
			pool: "pool-a",
			want: &core.Affinity{
				NodeAffinity: &core.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &core.NodeSelector{
						NodeSelectorTerms: []core.NodeSelectorTerm{
							{
								MatchExpressions: []core.NodeSelectorRequirement{{
									Key:      "svccontroller.k3s.cattle.io/lbpool",
									Operator: core.NodeSelectorOpIn,
									Values:   []string{"pool-a"},
								}},
							},
							{
								MatchExpressions: []core.NodeSelectorRequirement{{
									Key:      "lbpool.svccontroller.k3s.cattle.io/pool-a",
									Operator: core.NodeSelectorOpIn,
									Values:   []string{"true"},
								}},
							},
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nodePoolAffinity(tt.pool); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("nodePoolAffinity() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitGenerateName(t *testing.T) {
	uid := types.UID("35a5ccb3-4a82-40b7-8d83-cda9582e4251")
	tests := []struct {
		name string
		svc  *core.Service
		want string
	}{
		{
			name: "Short name",
			svc: &core.Service{
				ObjectMeta: meta.ObjectMeta{
					Name: "a-service",
					UID:  uid,
				},
			},
			want: "svclb-a-service-35a5ccb3",
		},
		{
			name: "Long name",
			svc: &core.Service{
				ObjectMeta: meta.ObjectMeta{
					Name: "a-service-with-a-very-veeeeeery-long-yet-valid-name",
					UID:  uid,
				},
			},
			want: "svclb-a-service-with-a-very-veeeeeery-long-yet-valid-n-35a5ccb3",
		},
		{
			name: "Long hypenated name",
			svc: &core.Service{
				ObjectMeta: meta.ObjectMeta{
					Name: "a-service-with-a-name-with-inconvenient------------hypens",
					UID:  uid,
				},
			},
			want: "svclb-a-service-with-a-name-with-inconvenient-35a5ccb3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateName(tt.svc); got != tt.want {
				t.Errorf("generateName() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

// Test_UnitK3s_DaemonSetNodeSelectorFollowsNodeLabel confirms that the ServiceLB DaemonSet
// NodeSelector is added when a node is labeled, and removed again when the label goes away.
func Test_UnitK3s_DaemonSetNodeSelectorFollowsNodeLabel(t *testing.T) {
	node := &core.Node{ObjectMeta: meta.ObjectMeta{Name: "test-node"}}
	nodeIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if err := nodeIndexer.Add(node); err != nil {
		t.Fatalf("failed to seed node cache: %v", err)
	}

	ds := &apps.DaemonSet{
		ObjectMeta: meta.ObjectMeta{
			Name:      "svclb-test-service",
			Namespace: DefaultLBNS,
			Labels:    map[string]string{nodeSelectorLabel: "false"},
		},
	}
	dsIndexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	if err := dsIndexer.Add(ds); err != nil {
		t.Fatalf("failed to seed daemonset cache: %v", err)
	}

	k := &k3s{
		Config:               Config{LBEnabled: true, LBNamespace: DefaultLBNS},
		client:               fake.NewClientset(ds),
		nodeCache:            generic.NewNonNamespacedCache[*core.Node](nodeIndexer, core.Resource("nodes")),
		daemonsetCache:       generic.NewCache[*apps.DaemonSet](dsIndexer, apps.Resource("daemonsets")),
		nodeHadSelectorLabel: map[string]bool{},
	}

	// nodeSelector returns the NodeSelector currently set on the DaemonSet, syncing the cache
	// from the API as the shared informer would after an update.
	nodeSelector := func(t *testing.T) map[string]string {
		t.Helper()
		updated, err := k.client.AppsV1().DaemonSets(DefaultLBNS).Get(context.TODO(), ds.Name, meta.GetOptions{})
		if err != nil {
			t.Fatalf("failed to get daemonset: %v", err)
		}
		if err := dsIndexer.Update(updated); err != nil {
			t.Fatalf("failed to sync daemonset cache: %v", err)
		}
		return updated.Spec.Template.Spec.NodeSelector
	}

	// Labeling a node should restrict the DaemonSet to labeled nodes.
	node.Labels = map[string]string{daemonsetNodeLabel: "true"}
	if err := nodeIndexer.Update(node); err != nil {
		t.Fatalf("failed to update node cache: %v", err)
	}
	if _, err := k.onChangeNode(node.Name, node); err != nil {
		t.Fatalf("onChangeNode() error = %v", err)
	}
	if got, want := nodeSelector(t), map[string]string{daemonsetNodeLabel: "true"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("NodeSelector after labeling node = %+v\nWant = %+v", got, want)
	}

	// Removing the label from the only labeled node should return the DaemonSet to all nodes.
	node.Labels = map[string]string{}
	if err := nodeIndexer.Update(node); err != nil {
		t.Fatalf("failed to update node cache: %v", err)
	}
	if _, err := k.onChangeNode(node.Name, node); err != nil {
		t.Fatalf("onChangeNode() error = %v", err)
	}
	if got := nodeSelector(t); len(got) != 0 {
		t.Fatalf("NodeSelector after removing label = %+v\nWant = empty", got)
	}

	// A routine update that does not change the node's label presence (e.g. a kubelet heartbeat)
	// must not trigger reconciliation. Corrupt the DaemonSet out-of-band, fire an unchanged
	// update for the still-unlabeled node, and confirm updateDaemonSets did not run (the corrupt
	// value survives).
	ds.Labels[nodeSelectorLabel] = "true"
	ds.Spec.Template.Spec.NodeSelector = map[string]string{daemonsetNodeLabel: "true"}
	if _, err := k.client.AppsV1().DaemonSets(DefaultLBNS).Update(context.TODO(), ds, meta.UpdateOptions{}); err != nil {
		t.Fatalf("failed to seed out-of-band daemonset state: %v", err)
	}
	if err := dsIndexer.Update(ds); err != nil {
		t.Fatalf("failed to sync daemonset cache: %v", err)
	}
	if _, err := k.onChangeNode(node.Name, node); err != nil {
		t.Fatalf("onChangeNode() error = %v", err)
	}
	if got, want := nodeSelector(t), map[string]string{daemonsetNodeLabel: "true"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("no-op node update should not reconcile; NodeSelector = %+v\nWant unchanged = %+v", got, want)
	}
}
