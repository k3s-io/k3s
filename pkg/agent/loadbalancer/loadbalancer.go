package loadbalancer

import (
	"context"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"reflect"
	"syscall"

	"gopkg.in/yaml.v2"

	"github.com/rancher/k3s/pkg/agent/templates"
	"github.com/rancher/k3s/pkg/agent/util"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
)

type LoadBalancer struct {
	done chan bool

	paramFile  string
	confFile   string
	prefixPath string
	serverURL  *url.URL

	CfgKey          string
	LogFile         string
	PidFile         string
	LocalAddress    string
	ServerAddresses []string
}

const (
	nginxConfTemplate = `
{{- if .LogFile }}
error_log {{ .LogFile }};
{{ end -}}
{{- if .PidFile }}
pid {{ .PidFile }};
{{ end -}}
user nobody;

events {
	worker_connections  4096;  ## Default: 1024
}

stream {
	upstream stream_backend {
		least_conn;
		{{range .ServerAddresses}}server {{.}};
		{{end}}
	}

	server {
		listen        {{ .LocalAddress }};
		proxy_pass    stream_backend;
	}
}
`
)

func Setup(ctx context.Context, cfg cmds.Agent) (*LoadBalancer, error) {
	if cfg.DisableLoadBalancer {
		logrus.Infof("Skipping load balancer setup, disabled")
		return nil, nil
	}
	lb := &LoadBalancer{}
	lb.done = make(chan bool, 1)
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	lb.LocalAddress = fmt.Sprintf("127.0.0.1:%d", cfg.LoadBalancerPort)
	serverURL, err := url.Parse(cfg.ServerURL)
	if err != nil {
		return nil, err
	}
	lb.serverURL = serverURL
	lb.ServerAddresses = []string{serverURL.Host}

	etcDir := filepath.Join(cfg.DataDir, "etc", "nginx")
	os.MkdirAll(etcDir, 0700)
	lb.paramFile = filepath.Join(etcDir, "lb-params.yaml")
	lb.confFile = filepath.Join(etcDir, "nginx.conf")

	runDir := filepath.Join(cfg.DataDir, "nginx")
	lb.prefixPath = filepath.Join(runDir, "usr")
	os.MkdirAll(lb.prefixPath, 0700)

	lb.LogFile = filepath.Join(runDir, "nginx.log")
	lb.PidFile = filepath.Join(runDir, "nginx.pid")

	paramsOut, err := yaml.Marshal(lb)
	if err != nil {
		return nil, err
	}
	lb.CfgKey = fmt.Sprintf("%x", md5.Sum(paramsOut))

	updateConf := true
	if paramBytes, err := ioutil.ReadFile(lb.paramFile); err == nil {
		params := &LoadBalancer{}
		if err := yaml.Unmarshal(paramBytes, params); err == nil {
			if params.CfgKey == lb.CfgKey {
				updateConf = false
				lb.ServerAddresses = params.ServerAddresses
			}
		}
	}

	if updateConf {
		if err := lb.UpdateConf(nil, false); err != nil {
			return nil, err
		}
	}
	if err := lb.Restart(); err != nil {
		return nil, err
	}

	go func() {
		select {
		case sig := <-sigs:
			logrus.Infof("Signal caught for load balancer: %v", sig)
		case <-ctx.Done():
			logrus.Infof("Context done for load balancer: %s", ctx.Err())
		}
		lb.Stop()
		lb.done <- true
		close(lb.done)
	}()

	return lb, nil
}

func (lb *LoadBalancer) Done() chan bool {
	if lb == nil {
		done := make(chan bool, 1)
		done <- true
		close(done)
		return done
	}
	return lb.done
}

func (lb *LoadBalancer) LoadBalancerServerURL() string {
	if lb == nil {
		return ""
	}
	serverURL := *lb.serverURL
	serverURL.Host = lb.LocalAddress
	return serverURL.String()
}

func (lb *LoadBalancer) Restart() error {
	if lb == nil {
		return nil
	}
	if _, err := os.Stat(lb.PidFile); err == nil {
		lb.Stop()
	}
	return lb.Start()
}

func (lb *LoadBalancer) Start() error {
	if lb == nil {
		return nil
	}
	logrus.Infof("Starting load balancer %s -> %v", lb.LocalAddress, lb.ServerAddresses)
	return lb.nginxExec()
}

func (lb *LoadBalancer) Reload() error {
	if lb == nil {
		return nil
	}
	logrus.Infof("Reloading load balancer %s -> %v", lb.LocalAddress, lb.ServerAddresses)
	return lb.nginxExec("-s", "reload")
}

func (lb *LoadBalancer) Stop() error {
	if lb == nil {
		return nil
	}
	logrus.Infof("Stopping load balancer %s", lb.LocalAddress)
	return lb.nginxExec("-s", "stop")
}

func (lb *LoadBalancer) Update(serverAddresses []string) {
	if lb == nil {
		return
	}
	if len(serverAddresses) == 0 ||
		reflect.DeepEqual(serverAddresses, lb.ServerAddresses) {
		return
	}
	if err := lb.UpdateConf(serverAddresses, true); err != nil {
		logrus.Warnf("Error updating load balancer: %s", err)
	}
}

func (lb *LoadBalancer) UpdateConf(serverAddresses []string, reload bool) error {
	if lb == nil {
		return nil
	}
	if serverAddresses != nil {
		lb.ServerAddresses = serverAddresses
	}
	if lb.confFile == "" {
		return nil
	}
	parsedTemplate, err := templates.ParseTemplateFromConfig(nginxConfTemplate, lb)
	if err != nil {
		return err
	}
	if err := util.WriteFile(lb.confFile, parsedTemplate); err != nil {
		return err
	}
	if lb.paramFile == "" {
		return nil
	}
	paramsOut, err := yaml.Marshal(lb)
	if err != nil {
		return err
	}
	if err := util.WriteFile(lb.paramFile, string(paramsOut)); err != nil {
		return err
	}
	if !reload {
		return nil
	}
	return lb.Reload()
}

func (lb *LoadBalancer) nginxExec(args ...string) (_err error) {
	defer func() {
		if _err != nil {
			logrus.Errorf("nginxExec load balancer error: %s", _err)
		}
	}()

	nginxArgs := []string{}
	if lb.confFile != "" {
		nginxArgs = append(nginxArgs, "-c", lb.confFile)
	}
	if lb.prefixPath != "" {
		nginxArgs = append(nginxArgs, "-p", lb.prefixPath)
	}
	nginxArgs = append(nginxArgs, args...)
	cmd := exec.Command("nginx", nginxArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
