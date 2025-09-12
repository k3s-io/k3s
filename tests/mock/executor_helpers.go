package mock

import (
	"testing"

	"github.com/golang/mock/gomock"
	executor "github.com/k3s-io/k3s/pkg/daemons/executor"
)

// NewExecutorWithEmbeddedETCD creates a new mock executor, and sets it as the current executor.
// The executor exepects calls to ETCD(), and wraps the embedded executor method of the same name.
// The various ready channels are also mocked with immediate channel closure.
func NewExecutorWithEmbeddedETCD(t *testing.T) *Executor {
	mockController := gomock.NewController(t)
	mockExecutor := NewExecutor(mockController)

	embed := &executor.Embedded{}
	mockExecutor.EXPECT().Bootstrap(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(embed.Bootstrap)
	mockExecutor.EXPECT().CurrentETCDOptions().AnyTimes().DoAndReturn(embed.CurrentETCDOptions)
	mockExecutor.EXPECT().ETCD(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(embed.ETCD)
	mockExecutor.EXPECT().ETCDReadyChan().AnyTimes().DoAndReturn(embed.ETCDReadyChan)
	mockExecutor.EXPECT().IsSelfHosted().AnyTimes().DoAndReturn(embed.IsSelfHosted)

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
