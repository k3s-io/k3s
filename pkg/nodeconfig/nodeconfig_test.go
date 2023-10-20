package nodeconfig

import (
	"os"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util/jsonpatch"
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
			NodeArgsAnnotation:       `["server","--flannel-backend","none"]`,
			NodeEnvAnnotation:        `{"` + TestEnvName + `":"fakeNode-with-annotation"}`,
			NodeConfigHashAnnotation: "5E6GSWFRVCOEB3BFFVXKWVD7IQEVJFJAALHPOTCLV7SL33N6SIYA====",
		},
	},
}

func Test_UnitSetExistingNodeConfigAnnotations(t *testing.T) {
	// adding same config
	os.Args = []string{version.Program, "server", "--flannel-backend=none"}
	os.Setenv(version.ProgramUpper+"_NODE_NAME", "fakeNode-with-annotation")
	patch := jsonpatch.NewBuilder()
	err := SetNodeConfigAnnotations(FakeNodeConfig, FakeNodeWithAnnotation, patch)
	if err != nil {
		t.Fatalf("Failed to set node config annotation: %v", err)
	}
	if patch.Len() != 0 {
		t.Errorf("Test_UnitSetExistingNodeConfigAnnotations() expected no patches, got %v", string(patch.MustMarshal()))
	}
}

func Test_UnitSetNodeConfigAnnotations(t *testing.T) {
	type args struct {
		config *config.Node
		node   *corev1.Node
		osArgs []string
	}
	setup := func(osArgs []string, nodeName string) error {
		os.Args = osArgs
		return os.Setenv(TestEnvName, nodeName)
	}
	teardown := func() error {
		return os.Unsetenv(TestEnvName)
	}
	tests := []struct {
		name      string
		args      args
		wantErr   bool
		wantPatch string
	}{
		{
			name: "Set NodeConfigAnnotations to same values",
			args: args{
				config: FakeNodeConfig,
				node:   FakeNodeWithAnnotation,
				osArgs: []string{version.Program, "server", "--flannel-backend=none"},
			},
			wantPatch: `[]`,
		},
		{
			name: "Set NodeConfigAnnotations to different values",
			args: args{
				config: FakeNodeConfig,
				node:   FakeNodeWithAnnotation,
				osArgs: []string{version.Program, "server", "--flannel-backend=none", "--write-kubeconfig-mode=777"},
			},
			wantPatch: `[` +
				`{"op":"add","path":"/metadata/annotations/k3s.io~1node-args","value":"[\"server\",\"--flannel-backend\",\"none\",\"--write-kubeconfig-mode\",\"777\"]"},` +
				`{"op":"add","path":"/metadata/annotations/k3s.io~1node-config-hash","value":"HG5SBLM6J7NNQ55XTG3HJ46UHNTA4AQOVCVEBME4SVNKG7RXBCTQ===="}` +
				`]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer teardown()
			if err := setup(tt.args.osArgs, tt.args.node.Name); err != nil {
				t.Errorf("Setup for SetNodeConfigAnnotations() failed = %v", err)
				return
			}
			patch := jsonpatch.NewBuilder()
			err := SetNodeConfigAnnotations(tt.args.config, tt.args.node, patch)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetNodeConfigAnnotations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			b, err := patch.Marshal()
			if err != nil {
				t.Errorf("patch.Marshal() error = %v", err)
				return
			}
			if p := string(b); p != tt.wantPatch {
				t.Errorf("Wanted patch: %s\nGot: %s", tt.wantPatch, p)
			}
		})
	}
}
