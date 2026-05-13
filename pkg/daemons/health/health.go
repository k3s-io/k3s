// Package health provides a small framework for registering per-component
// liveness probes used by the systemd watchdog notifier. Each registered
// Checker is invoked on every watchdog tick; if any check fails the notifier
// stays silent and systemd will eventually restart k3s.
package health

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

const dialTimeout = 5 * time.Second

type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

type Func struct {
	ComponentName string
	Probe         func(ctx context.Context) error
}

func (f Func) Name() string                    { return f.ComponentName }
func (f Func) Check(ctx context.Context) error { return f.Probe(ctx) }

type Group struct {
	checkers []Checker
}

func NewGroup() *Group { return &Group{} }

func (g *Group) Add(c ...Checker) {
	for _, ck := range c {
		if ck == nil {
			continue
		}
		g.checkers = append(g.checkers, ck)
	}
}

func (g *Group) Len() int { return len(g.checkers) }

func (g *Group) Names() []string {
	names := make([]string, len(g.checkers))
	for i, c := range g.checkers {
		names[i] = c.Name()
	}
	return names
}

func (g *Group) CheckAll(ctx context.Context) (string, error) {
	for _, c := range g.checkers {
		if err := c.Check(ctx); err != nil {
			return c.Name(), err
		}
	}
	return "", nil
}

func TCP(name, addr string) Checker {
	return Func{ComponentName: name, Probe: func(ctx context.Context) error {
		probeCtx, cancel := context.WithTimeout(ctx, dialTimeout)
		defer cancel()
		var d net.Dialer
		conn, err := d.DialContext(probeCtx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("dial tcp %s: %w", addr, err)
		}
		return conn.Close()
	}}
}

func UnixSocket(name, path string) Checker {
	return Func{ComponentName: name, Probe: func(ctx context.Context) error {
		probeCtx, cancel := context.WithTimeout(ctx, dialTimeout)
		defer cancel()
		var d net.Dialer
		conn, err := d.DialContext(probeCtx, "unix", path)
		if err != nil {
			return fmt.Errorf("dial unix %s: %w", path, err)
		}
		return conn.Close()
	}}
}

func HTTPGet(name, url string) Checker {
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			DisableKeepAlives: true,
		},
	}
	return Func{ComponentName: name, Probe: func(ctx context.Context) error {
		probeCtx, cancel := context.WithTimeout(ctx, dialTimeout)
		defer cancel()
		req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, url, nil)
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
	}}
}
