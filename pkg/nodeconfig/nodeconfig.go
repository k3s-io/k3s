package nodeconfig

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
)

const (
	NodeArgsAnnotation       = "k3s.io/node-args"
	NodeEnvAnnotation        = "k3s.io/node-env"
	NodeConfigHashAnnotation = "k3s.io/node-config-hash"
)

func getNodeArgs() (string, error) {
	nodeArgsList := []string{}
	for _, arg := range os.Args[1:] {
		if strings.Contains(arg, "=") {
			parsedArg := strings.SplitN(arg, "=", 2)
			nodeArgsList = append(nodeArgsList, parsedArg...)
			continue
		}
		nodeArgsList = append(nodeArgsList, arg)
	}
	for i, arg := range nodeArgsList {
		if isSecret(arg) {
			if i+1 < len(nodeArgsList) {
				nodeArgsList[i+1] = ""
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
		if strings.HasPrefix(keyValue[0], "K3S_") {
			k3sEnv[keyValue[0]] = keyValue[1]
		}
	}
	for key := range k3sEnv {
		if isSecret(key) {
			k3sEnv[key] = ""
		}
	}
	k3sEnvJSON, err := json.Marshal(k3sEnv)
	if err != nil {
		return "", errors.Wrap(err, "Failed to retrieve environment map for node")
	}
	return string(k3sEnvJSON), nil
}

func SetNodeConfigAnnotations(node *corev1.Node) (bool, error) {
	nodeArgs, err := getNodeArgs()
	if err != nil {
		return false, err
	}
	nodeEnv, err := getNodeEnv()
	if err != nil {
		return false, err
	}
	h := sha256.New()
	_, err = h.Write([]byte(nodeArgs + nodeEnv))
	if err != nil {
		return false, fmt.Errorf("Failed to hash the node config: %v", err)
	}
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	configHash := h.Sum(nil)
	encoded := base32.StdEncoding.EncodeToString(configHash[:])
	if node.Annotations[NodeConfigHashAnnotation] == encoded {
		return false, nil
	}
	node.Annotations[NodeEnvAnnotation] = nodeEnv
	node.Annotations[NodeArgsAnnotation] = nodeArgs
	node.Annotations[NodeConfigHashAnnotation] = encoded
	return true, nil
}

func isSecret(key string) bool {
	secretData := []string{
		"K3S_TOKEN",
		"K3S_DATASTORE_ENDPOINT",
		"K3S_AGENT_TOKEN",
		"K3S_CLUSTER_SECRET",
		"--token",
		"-t",
		"--agent-token",
		"--datastore-endpoint",
		"--cluster-secret",
	}
	for _, secret := range secretData {
		if key == secret {
			return true
		}
	}
	return false
}
