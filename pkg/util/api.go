package util

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/rancher/wrangler/v3/pkg/schemes"
	"github.com/sirupsen/logrus"
	authorizationv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	clientset "k8s.io/client-go/kubernetes"
	authorizationv1client "k8s.io/client-go/kubernetes/typed/authorization/v1"
	coregetter "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
)

// This sets a default duration to wait for the apiserver to become ready. This is primarily used to
// block startup of agent supervisor controllers until the apiserver is ready to serve requests, in the
// same way that the apiReady channel is used in the server packages, so it can be fairly long. It must
// be at least long enough for downstream projects like RKE2 to start the apiserver in the background.
const DefaultAPIServerReadyTimeout = 15 * time.Minute

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
// This is modified from WaitForAPIServer from the Kubernetes controller-manager app, but checks the
// readyz endpoint instead of the deprecated healthz endpoint, and supports context.
func WaitForAPIServerReady(ctx context.Context, kubeconfigPath string, timeout time.Duration) error {
	var lastErr error
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}

	// By default, idle connections to the apiserver are returned to a global pool
	// between requests.  Explicitly flag this client's request for closure so that
	// we re-dial through the loadbalancer in case the endpoints have changed.
	restConfig.Wrap(func(rt http.RoundTripper) http.RoundTripper {
		return roundTripFunc(func(req *http.Request) (*http.Response, error) {
			req.Close = true
			return rt.RoundTrip(req)
		})
	})

	restConfig = dynamic.ConfigFor(restConfig)
	restConfig.GroupVersion = &schema.GroupVersion{}
	restClient, err := rest.RESTClientFor(restConfig)
	if err != nil {
		return err
	}

	err = wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		healthStatus := 0
		result := restClient.Get().AbsPath("/readyz").Do(ctx).StatusCode(&healthStatus)
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

type genericAccessReviewRequest func(context.Context) (*authorizationv1.SubjectAccessReviewStatus, error)

// WaitForRBACReady polls an AccessReview request until it returns an allowed response. If the user
// and group are empty, it uses SelfSubjectAccessReview, otherwise SubjectAccessReview is used.  It
// will return an error if the timeout expires, or nil if the SubjectAccessReviewStatus indicates
// the access would be allowed.
func WaitForRBACReady(ctx context.Context, kubeconfigPath string, timeout time.Duration, ra authorizationv1.ResourceAttributes, user string, groups ...string) error {
	var lastErr error
	restConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return err
	}
	authClient, err := authorizationv1client.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	var reviewFunc genericAccessReviewRequest
	if len(user) == 0 && len(groups) == 0 {
		reviewFunc = selfSubjectAccessReview(authClient, ra)
	} else {
		reviewFunc = subjectAccessReview(authClient, ra, user, groups)
	}

	err = wait.PollUntilContextTimeout(ctx, time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		status, rerr := reviewFunc(ctx)
		if rerr != nil {
			lastErr = rerr
			return false, nil
		}
		if status.Allowed {
			return true, nil
		}
		lastErr = errors.New(status.Reason)
		return false, nil
	})

	if err != nil {
		return merr.NewErrors(err, lastErr)
	}

	return nil
}

// selfSubjectAccessReview returns a function that makes SelfSubjectAccessReview requests using the
// provided client and attributes, returning a status or error.
func selfSubjectAccessReview(authClient *authorizationv1client.AuthorizationV1Client, ra authorizationv1.ResourceAttributes) genericAccessReviewRequest {
	return func(ctx context.Context) (*authorizationv1.SubjectAccessReviewStatus, error) {
		r, err := authClient.SelfSubjectAccessReviews().Create(ctx, &authorizationv1.SelfSubjectAccessReview{
			Spec: authorizationv1.SelfSubjectAccessReviewSpec{
				ResourceAttributes: &ra,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
		return &r.Status, nil
	}
}

// subjectAccessReview returns a function that makes SubjectAccessReview requests using the
// provided client, attributes, user, and group, returning a status or error.
func subjectAccessReview(authClient *authorizationv1client.AuthorizationV1Client, ra authorizationv1.ResourceAttributes, user string, groups []string) genericAccessReviewRequest {
	return func(ctx context.Context) (*authorizationv1.SubjectAccessReviewStatus, error) {
		r, err := authClient.SubjectAccessReviews().Create(ctx, &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				ResourceAttributes: &ra,
				User:               user,
				Groups:             groups,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
		return &r.Status, nil
	}
}

func BuildControllerEventRecorder(k8s clientset.Interface, controllerName, namespace string) record.EventRecorder {
	logrus.Infof("Creating %s event broadcaster", controllerName)
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&coregetter.EventSinkImpl{Interface: k8s.CoreV1().Events(namespace)})
	nodeName := os.Getenv("NODE_NAME")
	return eventBroadcaster.NewRecorder(schemes.All, v1.EventSource{Component: controllerName, Host: nodeName})
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (w roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return w(req)
}
