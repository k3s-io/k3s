package nodeconfig

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/k3s-io/k3s/pkg/configfilearg"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util/jsonpatch"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	NodeArgsAnnotation       = version.Program + ".io/node-args"
	NodeEnvAnnotation        = version.Program + ".io/node-env"
	NodeConfigHashAnnotation = version.Program + ".io/node-config-hash"
	ClusterEgressLabel       = "egress." + version.Program + ".io/cluster"
)

const (
	OmittedValue = "********"
)

func getNodeArgs() (string, error) {
	nodeArgsList := []string{}
	for _, arg := range configfilearg.MustParse(os.Args[1:]) {
		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			parsedArg := strings.SplitN(arg, "=", 2)
			nodeArgsList = append(nodeArgsList, parsedArg...)
			continue
		}
		nodeArgsList = append(nodeArgsList, arg)
	}
	for i, arg := range nodeArgsList {
		if isSecret(arg) {
			if i+1 < len(nodeArgsList) {
				nodeArgsList[i+1] = OmittedValue
			}
		}
	}
	nodeArgs, err := json.Marshal(nodeArgsList)
	if err != nil {
		return "", errors.Wrap(err, "Failed to retrieve argument list for node")
	}
	return string(nodeArgs), nil
}

func getNodeEnv() (string, error) {
	k3sEnv := make(map[string]string)
	for _, v := range os.Environ() {
		keyValue := strings.SplitN(v, "=", 2)
		if strings.HasPrefix(keyValue[0], version.ProgramUpper+"_") {
			k3sEnv[keyValue[0]] = keyValue[1]
		}
	}
	for key := range k3sEnv {
		if isSecret(key) {
			k3sEnv[key] = OmittedValue
		}
	}
	k3sEnvJSON, err := json.Marshal(k3sEnv)
	if err != nil {
		return "", errors.Wrap(err, "Failed to retrieve environment map for node")
	}
	return string(k3sEnvJSON), nil
}

// SetNodeConfigAnnotations stores a redacted version of the k3s cli args and
// environment variables as annotations on the node object. It also stores a
// hash of the combined args + variables. These are used by other components
// to determine if the node configuration has been changed.
func SetNodeConfigAnnotations(nodeConfig *config.Node, node *corev1.Node, patch jsonpatch.PatchBuilder) error {
	ls := labels.Set(node.Annotations)
	patch = patch.WithPath("metadata", "annotations")
	nodeArgs, err := getNodeArgs()
	if err != nil {
		return err
	}
	nodeEnv, err := getNodeEnv()
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = h.Write([]byte(nodeArgs + nodeEnv))
	if err != nil {
		return fmt.Errorf("Failed to hash the node config: %v", err)
	}
	configHash := h.Sum(nil)
	encoded := base32.StdEncoding.EncodeToString(configHash[:])

	patch.AddIfNotEqual(ls, NodeEnvAnnotation, nodeEnv)
	patch.AddIfNotEqual(ls, NodeArgsAnnotation, nodeArgs)
	patch.AddIfNotEqual(ls, NodeConfigHashAnnotation, encoded)
	return nil
}

// SetNodeConfigLabels adds labels for functionality flags
// that may not be present on down-level or up-level nodes.
// These labels are used by other components to determine whether
// or not a node supports particular functionality.
func SetNodeConfigLabels(nodeConfig *config.Node, node *corev1.Node, patch jsonpatch.PatchBuilder) error {
	ls := labels.Set(node.Labels)
	patch = patch.WithPath("metadata", "labels")
	switch nodeConfig.EgressSelectorMode {
	case config.EgressSelectorModeCluster, config.EgressSelectorModePod:
		patch.AddIfNotEqual(ls, ClusterEgressLabel, "true")
	default:
		patch.RemoveIfHas(ls, ClusterEgressLabel)
	}
	return nil
}

func isSecret(key string) bool {
	secretData := []string{
		version.ProgramUpper + "_TOKEN",
		version.ProgramUpper + "_DATASTORE_ENDPOINT",
		version.ProgramUpper + "_AGENT_TOKEN",
		version.ProgramUpper + "_CLUSTER_SECRET",
		version.ProgramUpper + "_VPN_AUTH",
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"--token",
		"-t",
		"--agent-token",
		"--datastore-endpoint",
		"--etcd-s3-access-key",
		"--etcd-s3-secret-key",
		"--vpn-auth",
	}
	for _, secret := range secretData {
		if key == secret {
			return true
		}
	}
	return false
}
