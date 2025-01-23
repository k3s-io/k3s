package nodepassword

import (
	"context"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
	coreclient "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/kubernetes/pkg/auth/nodeidentifier"
)

var identifier = nodeidentifier.NewDefaultNodeIdentifier()

// NodeAuthValidator returns a node name, or http error code and error
type NodeAuthValidator func(req *http.Request) (string, int, error)

// nodeInfo contains information on the requesting node, derived from auth creds
// and request headers.
type nodeInfo struct {
	Name     string
	Password string
	User     user.Info
}

// GetNodeAuthValidator returns a function that will be called to validate node password authentication.
// Node password authentication is used when requesting kubelet certificates, and verifies that the
// credentials are valid for the requested node name, and that the node password is valid if it exists.
// These checks prevent a user with access to one agent from requesting kubelet certificates that
// could be used to impersonate another cluster member.
func GetNodeAuthValidator(ctx context.Context, control *config.Control) NodeAuthValidator {
	runtime := control.Runtime
	deferredNodes := map[string]bool{}
	var secretClient coreclient.SecretController
	var nodeClient coreclient.NodeController
	var mu sync.Mutex

	return func(req *http.Request) (string, int, error) {
		node, err := getNodeInfo(req)
		if err != nil {
			return "", http.StatusBadRequest, err
		}

		// node identity auth uses an existing kubelet client cert instead of auth token.
		// If used, validate that the node identity matches the requested node name.
		nodeName, isNodeAuth := identifier.NodeIdentity(node.User)
		if isNodeAuth && nodeName != node.Name {
			return "", http.StatusBadRequest, errors.New("header node name does not match auth node name")
		}

		if secretClient == nil || nodeClient == nil {
			if runtime.Core != nil {
				// initialize the client if we can
				secretClient = runtime.Core.Core().V1().Secret()
				nodeClient = runtime.Core.Core().V1().Node()
			} else if node.Name == os.Getenv("NODE_NAME") {
				// If we're verifying our own password, verify it locally and ensure a secret later.
				return verifyLocalPassword(ctx, control, &mu, deferredNodes, node)
			} else if control.DisableAPIServer && !isNodeAuth {
				// If we're running on an etcd-only node, and the request didn't use Node Identity auth,
				// defer node password verification until an apiserver joins the cluster.
				return verifyRemotePassword(ctx, control, &mu, deferredNodes, node)
			} else {
				// Otherwise, reject the request until the core is ready.
				return "", http.StatusServiceUnavailable, util.ErrCoreNotReady
			}
		}

		// verify that the node exists, if using Node Identity auth
		if err := verifyNode(ctx, nodeClient, node); err != nil {
			return "", http.StatusUnauthorized, err
		}

		// verify that the node password secret matches, or create it if it does not
		if err := Ensure(secretClient, node.Name, node.Password); err != nil {
			// if the verification failed, reject the request
			if errors.Is(err, ErrVerifyFailed) {
				return "", http.StatusForbidden, err
			}
			// If verification failed due to an error creating the node password secret, allow
			// the request, but retry verification until the outage is resolved.  This behavior
			// allows nodes to join the cluster during outages caused by validating webhooks
			// blocking secret creation - if the outage requires new nodes to join in order to
			// run the webhook pods, we must fail open here to resolve the outage.
			return verifyRemotePassword(ctx, control, &mu, deferredNodes, node)
		}

		return node.Name, http.StatusOK, nil
	}
}

// getNodeInfo returns node name, password, and user extracted
// from request headers and context. An error is returned
// if any critical fields are missing.
func getNodeInfo(req *http.Request) (*nodeInfo, error) {
	user, ok := request.UserFrom(req.Context())
	if !ok {
		return nil, errors.New("auth user not set")
	}

	program := mux.Vars(req)["program"]
	nodeName := req.Header.Get(program + "-Node-Name")
	if nodeName == "" {
		return nil, errors.New("node name not set")
	}

	nodePassword := req.Header.Get(program + "-Node-Password")
	if nodePassword == "" {
		return nil, errors.New("node password not set")
	}

	return &nodeInfo{
		Name:     strings.ToLower(nodeName),
		Password: nodePassword,
		User:     user,
	}, nil
}

// verifyLocalPassword is used to validate the local node's password secret directly against the node password file, when the apiserver is unavailable.
// This is only used early in startup, when a control-plane node's agent is starting up without a functional apiserver.
func verifyLocalPassword(ctx context.Context, control *config.Control, mu *sync.Mutex, deferredNodes map[string]bool, node *nodeInfo) (string, int, error) {
	// do not attempt to verify the node password if the local host is not running an agent and does not have a node resource.
	// note that the agent certs and kubeconfigs are created even if the agent is disabled; the only thing that is skipped is starting the kubelet and container runtime.
	if control.DisableAgent {
		return node.Name, http.StatusOK, nil
	}

	// use same password file location that the agent creates
	nodePasswordRoot := "/"
	if control.Rootless {
		nodePasswordRoot = filepath.Join(path.Dir(control.DataDir), "agent")
	}
	nodeConfigPath := filepath.Join(nodePasswordRoot, "etc", "rancher", "node")
	nodePasswordFile := filepath.Join(nodeConfigPath, "password")

	passBytes, err := os.ReadFile(nodePasswordFile)
	if err != nil {
		return "", http.StatusInternalServerError, errors.Wrap(err, "unable to read node password file")
	}

	passHash, err := Hasher.CreateHash(strings.TrimSpace(string(passBytes)))
	if err != nil {
		return "", http.StatusInternalServerError, errors.Wrap(err, "unable to hash node password file")
	}

	if err := Hasher.VerifyHash(passHash, node.Password); err != nil {
		return "", http.StatusForbidden, errors.Wrap(err, "unable to verify local node password")
	}

	mu.Lock()
	defer mu.Unlock()

	if _, ok := deferredNodes[node.Name]; !ok {
		deferredNodes[node.Name] = true
		go ensureSecret(ctx, control, node)
		logrus.Infof("Password verified locally for node %s", node.Name)
	}

	return node.Name, http.StatusOK, nil
}

// verifyRemotePassword is used when the server does not have a local apisever, as in the case of etcd-only nodes.
// The node password is ensured once an apiserver joins the cluster.
func verifyRemotePassword(ctx context.Context, control *config.Control, mu *sync.Mutex, deferredNodes map[string]bool, node *nodeInfo) (string, int, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := deferredNodes[node.Name]; !ok {
		deferredNodes[node.Name] = true
		go ensureSecret(ctx, control, node)
		logrus.Infof("Password verification deferred for node %s", node.Name)
	}

	return node.Name, http.StatusOK, nil
}

// verifyNode confirms that a node with the given name exists, to prevent auth
// from succeeding with a client certificate for a node that has been deleted from the cluster.
func verifyNode(ctx context.Context, nodeClient coreclient.NodeController, node *nodeInfo) error {
	if nodeName, isNodeAuth := identifier.NodeIdentity(node.User); isNodeAuth {
		if _, err := nodeClient.Cache().Get(nodeName); err != nil {
			return errors.Wrap(err, "unable to verify node identity")
		}
	}
	return nil
}

// ensureSecret validates a server's node password secret once the apiserver is up.
// As the node has already joined the cluster at this point, this is purely informational.
func ensureSecret(ctx context.Context, control *config.Control, node *nodeInfo) {
	runtime := control.Runtime
	_ = wait.PollUntilContextCancel(ctx, time.Second*5, true, func(ctx context.Context) (bool, error) {
		if runtime.Core != nil {
			secretClient := runtime.Core.Core().V1().Secret()
			// This is consistent with events attached to the node generated by the kubelet
			// https://github.com/kubernetes/kubernetes/blob/612130dd2f4188db839ea5c2dea07a96b0ad8d1c/pkg/kubelet/kubelet.go#L479-L485
			nodeRef := &corev1.ObjectReference{
				Kind:      "Node",
				Name:      node.Name,
				UID:       types.UID(node.Name),
				Namespace: "",
			}
			if err := Ensure(secretClient, node.Name, node.Password); err != nil {
				runtime.Event.Eventf(nodeRef, corev1.EventTypeWarning, "NodePasswordValidationFailed", "Deferred node password secret validation failed: %v", err)
				// Return true to stop polling if the password verification failed; only retry on secret creation errors.
				return errors.Is(err, ErrVerifyFailed), nil
			}
			runtime.Event.Event(nodeRef, corev1.EventTypeNormal, "NodePasswordValidationComplete", "Deferred node password secret validation complete")
			return true, nil
		}
		return false, nil
	})
}
