package mock

import (
	"context"
	sync "sync"
	"testing"

	cmds "github.com/k3s-io/k3s/pkg/cli/cmds"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	executor "github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/executor/embed/etcd"
	"go.uber.org/mock/gomock"
)

// NewExecutorWithEmbeddedETCD creates a new mock executor, and sets it as the current executor.
// The executor exepects calls to ETCD(), and wraps the embedded executor method of the same name.
// The various ready channels are also mocked with immediate channel closure.
func NewExecutorWithEmbeddedETCD(t *testing.T) *Executor {
	mockController := gomock.NewController(t)
	mockExecutor := NewExecutor(mockController)
	fake := &fakeExecutor{}

	mockExecutor.EXPECT().Bootstrap(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(fake.Bootstrap)
	mockExecutor.EXPECT().CurrentETCDOptions().AnyTimes().DoAndReturn(fake.CurrentETCDOptions)
	mockExecutor.EXPECT().ETCD(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(fake.ETCD)
	mockExecutor.EXPECT().ETCDReadyChan().AnyTimes().DoAndReturn(fake.ETCDReadyChan)
	mockExecutor.EXPECT().IsSelfHosted().AnyTimes().DoAndReturn(fake.IsSelfHosted)

	closedChannel := func() <-chan struct{} {
		c := make(chan struct{})
		close(c)
		return c
	}
	mockExecutor.EXPECT().APIServerReadyChan().AnyTimes().DoAndReturn(closedChannel)
	mockExecutor.EXPECT().CRIReadyChan().AnyTimes().DoAndReturn(closedChannel)

	executor.Set(mockExecutor)

	return mockExecutor
}

type fakeExecutor struct {
	etcdReady chan struct{}
}

func (f *fakeExecutor) Bootstrap(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	f.etcdReady = make(chan struct{})
	return nil
}

func (f *fakeExecutor) CurrentETCDOptions() (executor.InitialOptions, error) {
	return executor.InitialOptions{}, nil
}

func (f *fakeExecutor) ETCD(ctx context.Context, wg *sync.WaitGroup, args *executor.ETCDConfig, extraArgs []string, test executor.TestFunc) error {
	return etcd.StartETCD(ctx, wg, args, extraArgs)
}

func (f *fakeExecutor) ETCDReadyChan() <-chan struct{} {
	return f.etcdReady
}

func (f *fakeExecutor) IsSelfHosted() bool {
	return true
}
