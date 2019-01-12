package leader

import (
	"context"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	// ensure that core is loaded into legacyschema
	_ "k8s.io/kubernetes/pkg/apis/core/install"
)

type Callback func(cb context.Context)

func RunOrDie(ctx context.Context, namespace, name string, client kubernetes.Interface, cb Callback) {
	if namespace == "" {
		namespace = "kube-system"
	}

	err := run(ctx, namespace, name, client, cb)
	if err != nil {
		logrus.Fatalf("Failed to start leader election for %s", name)
	}
	panic("Failed to start leader election for " + name)
}

func run(ctx context.Context, namespace, name string, client kubernetes.Interface, cb Callback) error {
	id, err := os.Hostname()
	if err != nil {
		return err
	}

	recorder := createRecorder(name, client)

	rl, err := resourcelock.New(resourcelock.ConfigMapsResourceLock,
		namespace,
		name,
		client.CoreV1(),
		resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: recorder,
		})
	if err != nil {
		logrus.Fatalf("error creating leader lock for %s: %v", name, err)
	}

	t := time.Second
	if dl := os.Getenv("DEV_LEADERELECTION"); dl != "" {
		t = time.Hour
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 45 * t,
		RenewDeadline: 30 * t,
		RetryPeriod:   2 * t,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				go cb(ctx)
			},
			OnStoppedLeading: func() {
				logrus.Fatalf("leaderelection lost for %s", name)
			},
		},
	})
	panic("unreachable")
}

func createRecorder(name string, kubeClient kubernetes.Interface) record.EventRecorder {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Debugf)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: v1core.New(kubeClient.CoreV1().RESTClient()).Events("")})
	return eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: name})
}
