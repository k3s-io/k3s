package restwatch

import (
	"time"

	"k8s.io/client-go/rest"
)

type WatchClient interface {
	WatchClient() rest.Interface
}

func UnversionedRESTClientFor(config *rest.Config) (rest.Interface, error) {
	client, err := rest.UnversionedRESTClientFor(config)
	if err != nil {
		return nil, err
	}

	if config.Timeout == 0 {
		return client, err
	}

	newConfig := *config
	newConfig.Timeout = time.Hour
	watchClient, err := rest.UnversionedRESTClientFor(&newConfig)
	if err != nil {
		return nil, err
	}

	return &clientWithWatch{
		RESTClient:  client,
		watchClient: watchClient,
	}, nil
}

type clientWithWatch struct {
	*rest.RESTClient
	watchClient *rest.RESTClient
}

func (c *clientWithWatch) WatchClient() rest.Interface {
	return c.watchClient
}
