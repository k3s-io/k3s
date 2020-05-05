package nodeconfig

import (
	"os"
	"testing"

	"github.com/rancher/k3s/pkg/version"
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

var FakeNodeWithAnnotation = &corev1.Node{
	TypeMeta: metav1.TypeMeta{
		Kind:       "Node",
		APIVersion: "v1",
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: "fakeNode-with-annotation",
		Annotations: map[string]string{
			NodeArgsAnnotation:       `["server","--no-flannel"]`,
			NodeEnvAnnotation:        `{"` + version.ProgramUpper + `_NODE_NAME":"fakeNode-with-annotation"}`,
			NodeConfigHashAnnotation: "LNQOAOIMOQIBRMEMACW7LYHXUNPZADF6RFGOSPIHJCOS47UVUJAA====",
		},
	},
}

func assertEqual(t *testing.T, a interface{}, b interface{}) {
	if a != b {
		t.Fatalf("[ %v != %v ]", a, b)
	}
}

func TestSetEmptyNodeConfigAnnotations(t *testing.T) {
	os.Args = []string{version.Program, "server", "--no-flannel"}
	os.Setenv(version.ProgramUpper+"_NODE_NAME", "fakeNode-no-annotation")
	nodeUpdated, err := SetNodeConfigAnnotations(FakeNodeWithNoAnnotation)
	if err != nil {
		t.Fatalf("Failed to set node config annotation: %v", err)
	}
	assertEqual(t, true, nodeUpdated)

	expectedArgs := `["server","--no-flannel"]`
	actualArgs := FakeNodeWithNoAnnotation.Annotations[NodeArgsAnnotation]
	assertEqual(t, expectedArgs, actualArgs)

	expectedEnv := `{"` + version.ProgramUpper + `_NODE_NAME":"fakeNode-no-annotation"}`
	actualEnv := FakeNodeWithNoAnnotation.Annotations[NodeEnvAnnotation]
	assertEqual(t, expectedEnv, actualEnv)

	expectedHash := "MROOIJGRXUZ53BM74K76TZLRXQOLNNBNJBJOY7JJ22EAEUIBW7YA===="
	actualHash := FakeNodeWithNoAnnotation.Annotations[NodeConfigHashAnnotation]
	assertEqual(t, expectedHash, actualHash)
}

func TestSetExistingNodeConfigAnnotations(t *testing.T) {
	// adding same config
	os.Args = []string{version.Program, "server", "--no-flannel"}
	os.Setenv(version.ProgramUpper+"_NODE_NAME", "fakeNode-with-annotation")
	nodeUpdated, err := SetNodeConfigAnnotations(FakeNodeWithAnnotation)
	if err != nil {
		t.Fatalf("Failed to set node config annotation: %v", err)
	}
	assertEqual(t, false, nodeUpdated)
}

func TestSetArgsWithEqual(t *testing.T) {
	os.Args = []string{version.Program, "server", "--no-flannel", "--write-kubeconfig-mode=777"}
	os.Setenv("K3S_NODE_NAME", "fakeNode-with-no-annotation")
	nodeUpdated, err := SetNodeConfigAnnotations(FakeNodeWithNoAnnotation)
	if err != nil {
		t.Fatalf("Failed to set node config annotation: %v", err)
	}
	assertEqual(t, true, nodeUpdated)
	expectedArgs := `["server","--no-flannel","--write-kubeconfig-mode","777"]`
	actualArgs := FakeNodeWithNoAnnotation.Annotations[NodeArgsAnnotation]
	assertEqual(t, expectedArgs, actualArgs)
}
