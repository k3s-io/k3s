package nodeconfig

import (
	"os"
	"testing"

	"github.com/k3s-io/k3s/pkg/daemons/config"
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
	os.Args = []string{version.Program, "server", "--flannel-backend=none"}
	os.Setenv(version.ProgramUpper+"_NODE_NAME", "fakeNode-with-annotation")
	nodeUpdated, err := SetNodeConfigAnnotations(FakeNodeConfig, FakeNodeWithAnnotation)
	if err != nil {
		t.Fatalf("Failed to set node config annotation: %v", err)
	}
	if nodeUpdated {
		t.Errorf("Test_UnitSetExistingNodeConfigAnnotations() expected false")
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
		name               string
		args               args
		want               bool
		wantErr            bool
		wantNodeArgs       string
		wantNodeEnv        string
		wantNodeConfigHash string
	}{
		{
			name: "Set empty NodeConfigAnnotations",
			args: args{
				config: FakeNodeConfig,
				node:   FakeNodeWithAnnotation,
				osArgs: []string{version.Program, "server", "--flannel-backend=none"},
			},
			want:               true,
			wantNodeArgs:       `["server","--flannel-backend","none"]`,
			wantNodeEnv:        `{"` + TestEnvName + `":"fakeNode-with-no-annotation"}`,
			wantNodeConfigHash: "DRWW63TXZZGSKLARSFZLNSJ3RZ6VR7LQ46WPKZMSLTSGNI2J42WA====",
		},
		{
			name: "Set args with equal",
			args: args{
				config: FakeNodeConfig,
				node:   FakeNodeWithNoAnnotation,
				osArgs: []string{version.Program, "server", "--flannel-backend=none", "--write-kubeconfig-mode=777"},
			},
			want:         true,
			wantNodeArgs: `["server","--flannel-backend","none","--write-kubeconfig-mode","777"]`,
			wantNodeEnv:  `{"` + TestEnvName + `":"fakeNode-with-no-annotation"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer teardown()
			if err := setup(tt.args.osArgs); err != nil {
				t.Errorf("Setup for SetNodeConfigAnnotations() failed = %v", err)
				return
			}
			got, err := SetNodeConfigAnnotations(tt.args.config, tt.args.node)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetNodeConfigAnnotations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SetNodeConfigAnnotations() = %+v\nWantRes = %+v", got, tt.want)
			}
			nodeAnn := tt.args.node.Annotations
			if nodeAnn[NodeArgsAnnotation] != tt.wantNodeArgs {
				t.Errorf("SetNodeConfigAnnotations() = %+v\nWantAnn.nodeArgs = %+v", nodeAnn[NodeArgsAnnotation], tt.wantNodeArgs)
			}
			if nodeAnn[NodeEnvAnnotation] != tt.wantNodeEnv {
				t.Errorf("SetNodeConfigAnnotations() = %+v\nWantAnn.nodeEnv = %+v", nodeAnn[NodeEnvAnnotation], tt.wantNodeEnv)
			}
			if tt.wantNodeConfigHash != "" && nodeAnn[NodeConfigHashAnnotation] != tt.wantNodeConfigHash {
				t.Errorf("SetNodeConfigAnnotations() = %+v\nWantAnn.nodeConfigHash = %+v", nodeAnn[NodeConfigHashAnnotation], tt.wantNodeConfigHash)
			}
		})
	}
}
