package nodeconfig

import (
	"os"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var FakeNodeWithNoAnnotation = &corev1.Node{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Node",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "fakeNode-no-annotation",
	},
}

var TestEnvName = version.ProgramUpper + "_NODE_NAME"
var FakeNodeConfig = &config.Node{}
var FakeNodeWithAnnotation = &corev1.Node{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Node",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "fakeNode-with-annotation",
		Annotations: map[string]string{
			NodeArgsAnnotation:       `["server","--flannel-backend=none"]`,
			NodeEnvAnnotation:        `{"` + TestEnvName + `":"fakeNode-with-annotation"}`,
			NodeConfigHashAnnotation: "5E6GSWFRVCOEB3BFFVXKWVD7IQEVJFJAALHPOTCLV7SL33N6SIYA====",
		},
	},
}

func Test_UnitSetExistingNodeConfigAnnotations(t *testing.T) {
	// adding same config
	patch := util.NewPatchList()
	os.Args = []string{version.Program, "server", "--flannel-backend=none"}
	os.Setenv(version.ProgramUpper+"_NODE_NAME", "fakeNode-with-annotation")
	err := SetNodeConfigAnnotations(FakeNodeConfig, patch, FakeNodeWithAnnotation)
	if err != nil {
		t.Fatalf("Failed to set node config annotation: %v", err)
	}
	if j, _ := patch.ToJSON(); j != "[]" {
		t.Errorf("Test_UnitSetExistingNodeConfigAnnotations() expected empty patch, got %v", j)
	}
}

func Test_UnitSetNodeConfigAnnotations(t *testing.T) {
	type args struct {
		config *config.Node
		node   *corev1.Node
		osArgs []string
	}
	setup := func(osArgs []string) error {
		os.Args = osArgs
		return os.Setenv(TestEnvName, "fakeNode-with-no-annotation")
	}
	teardown := func() error {
		return os.Unsetenv(TestEnvName)
	}
	tests := []struct {
		name     string
		args     args
		wantErr  bool
		wantJSON string
	}{
		{
			name: "Set empty NodeConfigAnnotations",
			args: args{
				config: FakeNodeConfig,
				node:   FakeNodeWithAnnotation,
				osArgs: []string{version.Program, "server", "--flannel-backend=none"},
			},
			wantJSON: `[{"op":"add","path":"/metadata/annotations/k3s.io~1node-config-hash","value":"DRWW63TXZZGSKLARSFZLNSJ3RZ6VR7LQ46WPKZMSLTSGNI2J42WA===="},` +
				`{"op":"add","path":"/metadata/annotations/k3s.io~1node-env","value":"{\"K3S_NODE_NAME\":\"fakeNode-with-no-annotation\"}"},` +
				`{"op":"add","path":"/metadata/annotations/k3s.io~1node-args","value":"[\"server\",\"--flannel-backend\",\"none\"]"}]`,
		},
		{
			name: "Set args with equal",
			args: args{
				config: FakeNodeConfig,
				node:   FakeNodeWithNoAnnotation,
				osArgs: []string{version.Program, "server", "--flannel-backend=none", "--write-kubeconfig-mode=777"},
			},
			wantJSON: `[{"op":"add","path":"/metadata/annotations/k3s.io~1node-config-hash","value":"IOESDALHLYKDFVH2D3QV7ELCIOBMPEJVCK37ANBYFODYKKPS7HWA===="},` +
				`{"op":"add","path":"/metadata/annotations/k3s.io~1node-env","value":"{\"K3S_NODE_NAME\":\"fakeNode-with-no-annotation\"}"},` +
				`{"op":"add","path":"/metadata/annotations/k3s.io~1node-args","value":"[\"server\",\"--flannel-backend\",\"none\",\"--write-kubeconfig-mode\",\"777\"]"}]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer teardown()
			patch := util.NewPatchList()
			if err := setup(tt.args.osArgs); err != nil {
				t.Errorf("Setup for SetNodeConfigAnnotations() failed = %v", err)
				return
			}
			err := SetNodeConfigAnnotations(tt.args.config, patch, tt.args.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetNodeConfigAnnotations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if j, _ := patch.ToJSON(); j != tt.wantJSON {
				t.Errorf("Test_UnitSetNodeConfigAnnotations() JSON= %v, wantJSON %v", j, tt.wantJSON)
			}
		})
	}
}
