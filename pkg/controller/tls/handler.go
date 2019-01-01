package tls

import (
	"context"

	v1 "github.com/rancher/rio/types/apis/k3s.cattle.io/v1"
)

func Register(ctx context.Context) error {
	clients := v1.ClientsFrom(ctx)
	clients.ListenerConfig.OnChange(ctx)
}
