package health

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"k8s.io/apiserver/pkg/server/healthz"
)

const dialTimeout = 5 * time.Second

func HTTPGet(name, url string) healthz.HealthChecker {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
	}
	return healthz.NamedCheck(name, func(_ *http.Request) error {
		ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("get %s: %w", url, err)
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return fmt.Errorf("get %s: status %d", url, resp.StatusCode)
		}
		return nil
	})
}

func TCP(name, addr string) healthz.HealthChecker {
	return healthz.NamedCheck(name, func(_ *http.Request) error {
		ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
		defer cancel()
		var d net.Dialer
		conn, err := d.DialContext(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("dial tcp %s: %w", addr, err)
		}
		return conn.Close()
	})
}

func GRPC(name, target string) healthz.HealthChecker {
	target = strings.TrimPrefix(target, "unix://")
	dialTarget := "unix:" + target
	return healthz.NamedCheck(name, func(_ *http.Request) error {
		ctx, cancel := context.WithTimeout(context.Background(), dialTimeout)
		defer cancel()
		conn, err := grpc.NewClient(dialTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return fmt.Errorf("dial grpc %s: %w", dialTarget, err)
		}
		defer conn.Close()
		client := healthpb.NewHealthClient(conn)
		resp, err := client.Check(ctx, &healthpb.HealthCheckRequest{})
		if err != nil {
			return fmt.Errorf("grpc health check %s: %w", dialTarget, err)
		}
		if resp.Status != healthpb.HealthCheckResponse_SERVING {
			return fmt.Errorf("grpc health check %s: status %s", dialTarget, resp.Status)
		}
		return nil
	})
}
