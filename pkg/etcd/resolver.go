package etcd

import (
	"net/url"
	"path"
	"strings"

	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
)

const scheme = "etcd-endpoint"

type EtcdSimpleResolver struct {
	*manual.Resolver
	endpoint string
}

// Cribbed from https://github.com/etcd-io/etcd/blob/v3.5.16/client/v3/internal/resolver/resolver.go
// but only supports a single fixed endpoint. We use this instead of the internal etcd client resolver
// because the agent loadbalancer handles failover and we don't want etcd or grpc's special behavior.
func NewSimpleResolver(endpoint string) *EtcdSimpleResolver {
	r := manual.NewBuilderWithScheme(scheme)
	return &EtcdSimpleResolver{Resolver: r, endpoint: endpoint}
}

func (r *EtcdSimpleResolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	res, err := r.Resolver.Build(target, cc, opts)
	if err != nil {
		return nil, err
	}

	if r.CC != nil {
		addr, serverName := interpret(r.endpoint)
		r.UpdateState(resolver.State{
			Addresses: []resolver.Address{{Addr: addr, ServerName: serverName}},
		})
	}

	return res, nil
}

func interpret(ep string) (string, string) {
	if strings.HasPrefix(ep, "unix:") || strings.HasPrefix(ep, "unixs:") {
		if strings.HasPrefix(ep, "unix:///") || strings.HasPrefix(ep, "unixs:///") {
			_, absolutePath, _ := strings.Cut(ep, "://")
			return "unix://" + absolutePath, path.Base(absolutePath)
		}
		if strings.HasPrefix(ep, "unix://") || strings.HasPrefix(ep, "unixs://") {
			_, localPath, _ := strings.Cut(ep, "://")
			return "unix:" + localPath, path.Base(localPath)
		}
		_, localPath, _ := strings.Cut(ep, ":")
		return "unix:" + localPath, path.Base(localPath)
	}
	if strings.Contains(ep, "://") {
		url, err := url.Parse(ep)
		if err != nil {
			return ep, ep
		}
		if url.Scheme == "http" || url.Scheme == "https" {
			return url.Host, url.Host
		}
		return ep, url.Host
	}
	return ep, ep
}

func authority(ep string) string {
	if _, authority, ok := strings.Cut(ep, "://"); ok {
		return authority
	}
	if suff, ok := strings.CutPrefix(ep, "unix:"); ok {
		return suff
	}
	if suff, ok := strings.CutPrefix(ep, "unixs:"); ok {
		return suff
	}
	return ep
}
