package util

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
)

func GetAddresses(endpoint *v1.Endpoints) []string {
	serverAddresses := []string{}
	if endpoint == nil {
		return serverAddresses
	}
	for _, subset := range endpoint.Subsets {
		var port string
		if len(subset.Ports) > 0 {
			port = strconv.Itoa(int(subset.Ports[0].Port))
		}
		if port == "" {
			port = "443"
		}
		for _, address := range subset.Addresses {
			serverAddresses = append(serverAddresses, net.JoinHostPort(address.IP, port))
		}
	}
	return serverAddresses
}

// WaitForAPIServerReady waits for the API Server's /readyz endpoint to report "ok" with timeout.
// This is cribbed from the Kubernetes controller-manager app, but checks the readyz endpoint instead of the deprecated healthz endpoint.
func WaitForAPIServerReady(client clientset.Interface, timeout time.Duration) error {
	var lastErr error
	restClient := client.Discovery().RESTClient()

	err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		healthStatus := 0
		result := restClient.Get().AbsPath("/readyz").Do(context.TODO()).StatusCode(&healthStatus)
		if rerr := result.Error(); rerr != nil {
			lastErr = errors.Wrap(rerr, "failed to get apiserver /readyz status")
			return false, nil
		}
		if healthStatus != http.StatusOK {
			content, _ := result.Raw()
			lastErr = fmt.Errorf("APIServer isn't ready: %v", string(content))
			logrus.Warnf("APIServer isn't ready yet: %v. Waiting a little while.", string(content))
			return false, nil
		}

		return true, nil
	})

	if err != nil {
		return merr.NewErrors(err, lastErr)
	}

	return nil
}
