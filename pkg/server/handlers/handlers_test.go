package handlers

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/k3s-io/k3s/pkg/authenticator"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	testutil "github.com/k3s-io/k3s/tests"
	"github.com/k3s-io/k3s/tests/mock"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	certutil "github.com/rancher/dynamiclistener/cert"
	"github.com/sirupsen/logrus"
	"go.uber.org/mock/gomock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/user"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

func Test_UnitHandlers(t *testing.T) {
	type sub struct {
		name    string
		prepare func(control *config.Control, req *http.Request)
		match   func(control *config.Control) types.GomegaMatcher
	}

	genericFailures := []sub{
		{
			name: "000 anonymous",
			match: func(_ *config.Control) types.GomegaMatcher {
				return HaveHTTPStatus(http.StatusForbidden)
			},
		}, {
			name: "001 bad basic",
			prepare: func(control *config.Control, req *http.Request) {
				req.SetBasicAuth("server", control.AgentToken)
			},
			match: func(_ *config.Control) types.GomegaMatcher {
				return HaveHTTPStatus(http.StatusUnauthorized)
			},
		}, {
			name: "002 valid cert but untrusted CA",
			prepare: func(control *config.Control, req *http.Request) {
				withNewClientCert(req, control.Runtime.ServerCA, control.Runtime.ServerCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
					CommonName:   "system:node:" + control.ServerNodeName,
					Organization: []string{user.NodesGroup},
					Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
				})
			},
			match: func(_ *config.Control) types.GomegaMatcher {
				return HaveHTTPStatus(http.StatusUnauthorized)
			},
		}, {
			name: "003 valid cert but no RBAC",
			prepare: func(control *config.Control, req *http.Request) {
				withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
					CommonName:   "system:monitoring",
					Organization: []string{user.MonitoringGroup},
					Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
				})
			},
			match: func(_ *config.Control) types.GomegaMatcher {
				return HaveHTTPStatus(http.StatusForbidden)
			},
		},
	}

	type pathTest struct {
		method string
		path   string
		subs   []sub
	}

	tests := []struct {
		name        string
		controlFunc func(*testing.T) (*config.Control, context.CancelFunc)
		paths       []pathTest
	}{
		{
			//*** tests with runtime core not ready ***
			name:        "no runtime core",
			controlFunc: getCorelessControl,
			paths: []pathTest{
				//** paths accessible with node cert or agent token, and specific headers **
				{
					method: http.MethodGet,
					path:   "/v1-k3s/serving-kubelet.crt",
					subs: append(genericFailures,
						sub{
							name: "100 valid basic but missing headers",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "101 valid cert but missing headers",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "102 valid cert but wrong node name",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:k3s-agent-1",
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "103 valid cert but nonexistent node",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "nonexistent")
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:nonexistent",
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusServiceUnavailable)
							},
						},
						sub{
							name: "104 valid basic legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "105 valid cert legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "106 valid basic legacy key deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "107 valid cert legacy key deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "108 valid basic different node",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "k3s-agent-1")
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusServiceUnavailable)
							},
						},
						sub{
							name: "109 valid basic bad node password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "k3s-agent-1")
								req.Header.Add("k3s-Node-Password", "invalid-password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusServiceUnavailable)
							},
						},
					),
				}, {
					method: http.MethodPost,
					path:   "/v1-k3s/serving-kubelet.crt",
					subs: append(genericFailures,
						sub{
							name: "200 valid basic client key but bad password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "201 valid cert client key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "203 valid basic client key but bad password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusForbidden)
							},
						},
						sub{
							name: "204 valid cert client key but bad password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusForbidden)
							},
						},
						sub{
							name: "205 valid basic client key but bad deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusForbidden)
							},
						},
						sub{
							name: "206 valid cert client key but bad deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusForbidden)
							},
						},
					),
				},
			},
		},
		{
			//*** tests with runtime core not ready and bind address set ***
			name: "no runtime core with bind-address",
			controlFunc: func(t *testing.T) (*config.Control, context.CancelFunc) {
				control, cancel := getCorelessControl(t)
				control.BindAddress = "192.0.2.100"
				return control, cancel
			},
			paths: []pathTest{
				//** paths accessible with node cert or agent token, and specific headers **
				{
					method: http.MethodGet,
					path:   "/v1-k3s/serving-kubelet.crt",
					subs: append(genericFailures,
						sub{
							name: "300 valid basic but missing headers",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "301 valid cert but missing headers",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "302 valid cert but wrong node name",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:k3s-agent-1",
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "303 valid cert but nonexistent node",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "nonexistent")
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:nonexistent",
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusServiceUnavailable)
							},
						},
						sub{
							name: "304 valid basic legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "305 valid cert legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "306 valid basic legacy key deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "307 valid cert legacy key deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "308 valid basic different node",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "k3s-agent-1")
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusServiceUnavailable)
							},
						},
						sub{
							name: "309 valid basic bad node password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "k3s-agent-1")
								req.Header.Add("k3s-Node-Password", "invalid-password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusServiceUnavailable)
							},
						},
					),
				}, {
					method: http.MethodPost,
					path:   "/v1-k3s/serving-kubelet.crt",
					subs: append(genericFailures,
						sub{
							name: "400 valid basic client key but bad password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "401 valid cert client key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "402 valid basic client key but bad password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusForbidden)
							},
						},
						sub{
							name: "403 valid cert client key but bad password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusForbidden)
							},
						},
						sub{
							name: "404 valid basic client key but bad deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusForbidden)
							},
						},
						sub{
							name: "405 valid cert client key but bad deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusForbidden)
							},
						},
					),
				},
			},
		},
		{
			//*** tests with no agent and runtime core not ready ***
			name:        "agentless no runtime core",
			controlFunc: getCorelessAgentlessControl,
			paths: []pathTest{
				//** paths accessible with node cert or agent token, and specific headers **
				{
					method: http.MethodGet,
					path:   "/v1-k3s/serving-kubelet.crt",
					subs: append(genericFailures,
						sub{
							name: "500 valid basic but missing headers",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "501 valid cert but missing headers",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "502 valid cert but wrong node name",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:k3s-agent-1",
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "503 valid cert but nonexistent node",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "nonexistent")
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:nonexistent",
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusServiceUnavailable)
							},
						},
						sub{
							name: "504 valid basic legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "505 valid cert legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "506 valid basic legacy key deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "507 valid cert legacy key deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "508 valid basic different node",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "k3s-agent-1")
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusServiceUnavailable)
							},
						},
						sub{
							name: "509 valid basic bad node password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "k3s-agent-1")
								req.Header.Add("k3s-Node-Password", "invalid-password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusServiceUnavailable)
							},
						},
					),
				}, {
					method: http.MethodPost,
					path:   "/v1-k3s/serving-kubelet.crt",
					subs: append(genericFailures,
						sub{
							name: "600 valid basic client key but bad password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "601 valid cert client key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "602 valid basic client key but bad password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "603 valid cert client key but bad password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "604 valid basic client key but bad deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "605 valid cert client key but bad deferred local password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "invalid-password")
								withClientAddress(req, control.BindAddressOrLoopback(false, false))
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
					),
				},
			},
		},
		{
			//*** tests with mocked core controllers ***
			name:        "mocked",
			controlFunc: getMockedControl,
			paths: []pathTest{
				//** paths accessible with node cert or agent token, and specific headers **
				{
					method: http.MethodGet,
					path:   "/v1-k3s/serving-kubelet.crt",
					subs: append(genericFailures,
						sub{
							name: "700 valid basic but missing headers",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "701 valid cert but missing headers",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "702 valid cert but wrong node name",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:k3s-agent-1",
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "703 valid cert but nonexistent node",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "nonexistent")
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:nonexistent",
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusUnauthorized)
							},
						},
						sub{
							name: "704 valid basic legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "705 valid cert legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "706 valid basic different node",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "k3s-agent-1")
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "707 valid basic bad node password",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", "k3s-agent-1")
								req.Header.Add("k3s-Node-Password", "invalid-password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusForbidden),
								)
							},
						},
					),
				}, {
					method: http.MethodPost,
					path:   "/v1-k3s/serving-kubelet.crt",
					subs: append(genericFailures,
						sub{
							name: "800 valid basic client key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "801 valid cert client key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/client-kubelet.crt",
					subs: append(genericFailures,
						sub{
							name: "900 valid basic but missing headers",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "901 valid cert but missing headers",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusBadRequest)
							},
						},
						sub{
							name: "902 valid basic legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "903 valid cert legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
					),
				}, {
					method: http.MethodPost,
					path:   "/v1-k3s/client-kubelet.crt",
					subs: append(genericFailures,
						sub{
							name: "A00 valid basic client key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "A01 valid cert client key",
							prepare: func(control *config.Control, req *http.Request) {
								req.Header.Add("k3s-Node-Name", control.ServerNodeName)
								req.Header.Add("k3s-Node-Password", "password")
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
					),
				},
				//** paths accessible with node cert or agent token **
				{
					method: http.MethodGet,
					path:   "/v1-k3s/client-kube-proxy.crt",
					subs: append(genericFailures,
						sub{
							name: "B00 valid basic legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "B01 valid cert legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
					),
				}, {
					method: http.MethodPost,
					path:   "/v1-k3s/client-kube-proxy.crt",
					subs: append(genericFailures,
						sub{
							name: "C00 valid basic client key",
							prepare: func(control *config.Control, req *http.Request) {
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "C01 valid cert client key",
							prepare: func(control *config.Control, req *http.Request) {
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/client-k3s-controller.crt",
					subs: append(genericFailures,
						sub{
							name: "D00 valid basic legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
						sub{
							name: "D01 valid cert legacy key",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(ContainSubstring("PRIVATE KEY")),
								)
							},
						},
					),
				}, {
					method: http.MethodPost,
					path:   "/v1-k3s/client-k3s-controller.crt",
					subs: append(genericFailures,
						sub{
							name: "E00 valid basic client key",
							prepare: func(control *config.Control, req *http.Request) {
								withCertificateRequest(req)
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
						sub{
							name: "E01 valid cert client key",
							prepare: func(control *config.Control, req *http.Request) {
								withCertificateRequest(req)
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(Not(ContainSubstring("PRIVATE KEY"))),
								)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/client-ca.crt",
					subs: append(genericFailures,
						sub{
							name: "F00 valid basic",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(control *config.Control) types.GomegaMatcher {
								certs, _ := os.ReadFile(control.Runtime.ClientCA)
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(certs),
								)
							},
						},
						sub{
							name: "F01 valid cert",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(control *config.Control) types.GomegaMatcher {
								certs, _ := os.ReadFile(control.Runtime.ClientCA)
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(certs),
								)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/server-ca.crt",
					subs: append(genericFailures,
						sub{
							name: "G00 valid basic",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(control *config.Control) types.GomegaMatcher {
								certs, _ := os.ReadFile(control.Runtime.ServerCA)
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(certs),
								)
							},
						},
						sub{
							name: "G01 valid cert",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(control *config.Control) types.GomegaMatcher {
								certs, _ := os.ReadFile(control.Runtime.ServerCA)
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(certs),
								)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/apiservers",
					subs: append(genericFailures,
						sub{
							name: "G00 valid basic",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPHeaderWithValue("content-type", "application/json"),
								)
							},
						},
						sub{
							name: "G01 valid cert",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPHeaderWithValue("content-type", "application/json"),
								)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/config",
					subs: append(genericFailures,
						sub{
							name: "H00 valid basic",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPHeaderWithValue("content-type", "application/json"),
								)
							},
						},
						sub{
							name: "H01 valid cert",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPHeaderWithValue("content-type", "application/json"),
								)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/readyz",
					subs: append(genericFailures,
						sub{
							name: "I00 valid basic",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("node", control.AgentToken)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody("ok"),
								)
							},
						},
						sub{
							name: "I01 valid cert",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody("ok"),
								)
							},
						},
					),
				},
				//** paths accessible with node cert **
				{
					method: http.MethodGet,
					path:   "/v1-k3s/connect",
					subs: append(genericFailures,
						sub{
							name: "J00 valid cert",
							prepare: func(control *config.Control, req *http.Request) {
								withNewClientCert(req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeletKey, certutil.Config{
									CommonName:   "system:node:" + control.ServerNodeName,
									Organization: []string{user.NodesGroup},
									Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
								})
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusOK)
							},
						},
					),
				},
				//** paths accessible with server token **
				{
					method: http.MethodGet,
					path:   "/v1-k3s/encrypt/status",
					subs: append(genericFailures,
						sub{
							name: "K00 valid basic",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("server", control.Token)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusOK)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/encrypt/config",
					subs: append(genericFailures,
						sub{
							name: "L00 valid basic",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("server", control.Token)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusMethodNotAllowed)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/cert/cacerts",
					subs: append(genericFailures,
						sub{
							name: "M00 valid basic",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("server", control.Token)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusMethodNotAllowed)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/server-bootstrap",
					subs: append(genericFailures,
						sub{
							name: "N00 valid basic",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("server", control.Token)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusBadRequest),
									HaveHTTPBody(ContainSubstring("etcd disabled")),
								)
							},
						},
					),
				}, {
					method: http.MethodGet,
					path:   "/v1-k3s/token",
					subs: append(genericFailures,
						sub{
							name: "O00 valid basic",
							prepare: func(control *config.Control, req *http.Request) {
								req.SetBasicAuth("server", control.Token)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusMethodNotAllowed)
							},
						},
					),
				},
				//** paths accessible with apiserver cert **
				{
					method: http.MethodConnect,
					path:   "/",
					subs: append(genericFailures,
						sub{
							name: "P00 valid cert",
							prepare: func(control *config.Control, req *http.Request) {
								withClientCert(req, control.Runtime.ClientKubeAPICert)
							},
							match: func(_ *config.Control) types.GomegaMatcher {
								return HaveHTTPStatus(http.StatusOK)
							},
						},
					),
				},
				//** paths accessible anonymously **
				{
					method: http.MethodGet,
					path:   "/ping",
					subs: []sub{
						{
							name: "Q00 anonymous",
							match: func(_ *config.Control) types.GomegaMatcher {
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody("pong"),
								)
							},
						},
					},
				}, {
					method: http.MethodGet,
					path:   "/cacerts",
					subs: []sub{
						{
							name: "R00 anonymous",
							match: func(control *config.Control) types.GomegaMatcher {
								certs, _ := os.ReadFile(control.Runtime.ServerCA)
								return And(
									HaveHTTPStatus(http.StatusOK),
									HaveHTTPBody(certs),
								)
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			control, cancel := tt.controlFunc(t)
			for _, ttt := range tt.paths {
				t.Run(ttt.method+" "+ttt.path, func(t *testing.T) {
					for _, ss := range ttt.subs {
						t.Run("handles "+ss.name+" request", func(t *testing.T) {
							req := httptest.NewRequest(ttt.method, ttt.path, nil)

							if ss.prepare != nil {
								ss.prepare(control, req)
							}

							resp := httptest.NewRecorder()
							control.Runtime.Handler.ServeHTTP(resp, req)
							t.Logf("Validating response: %s %s %s", resp.Result().Proto, resp.Result().Status, resp.Result().Header.Get("Content-Type"))
							NewWithT(t).Expect(resp).To(ss.match(control))
						})
					}
				})
			}
			cancel()
			testutil.CleanupDataDir(control)
		})
	}

	os.Unsetenv("NODE_NAME")
}

// getCorelessControl returns a Control structure with no mocked core controllers,
// as if the apiserver were not yet available.
func getCorelessControl(t *testing.T) (*config.Control, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	control := &config.Control{
		Token:          "token",
		AgentToken:     "agent-token",
		ServerNodeName: "k3s-server-1",
	}

	os.Setenv("NODE_NAME", control.ServerNodeName)
	control.DataDir = t.TempDir()
	testutil.GenerateRuntime(control)

	// add dummy handler for tunnel/proxy CONNECT requests, since we're not
	// setting up a whole remotedialer tunnel server here
	control.Runtime.Tunnel = http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {})

	// Set up node password file in rootless path to avoid having to stage test fixtures in /etc/rancher
	control.Rootless = true
	nodePasswordRoot := filepath.Join(path.Dir(control.DataDir), "agent")
	nodeConfigPath := filepath.Join(nodePasswordRoot, "etc", "rancher", "node")
	nodePasswordFile := filepath.Join(nodeConfigPath, "password")

	os.MkdirAll(nodeConfigPath, 0700)
	os.WriteFile(nodePasswordFile, []byte("password"), 0644)

	// add authenticator
	auth, err := authenticator.FromArgs([]string{
		"--basic-auth-file=" + control.Runtime.PasswdFile,
		"--client-ca-file=" + control.Runtime.ClientCA,
	})
	NewWithT(t).Expect(err).ToNot(HaveOccurred())
	control.Runtime.Authenticator = auth

	// finally, bind request handlers
	control.Runtime.Handler = NewHandler(ctx, control, &cmds.Server{})

	return control, cancel
}

// getCorelessAgentlessControl returns a Control structure with no mocked core controllers,
// as if the apiserver were not yet available on a node with no local agent.
func getCorelessAgentlessControl(t *testing.T) (*config.Control, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	control := &config.Control{
		Token:          "token",
		AgentToken:     "agent-token",
		ServerNodeName: "k3s-server-1",
	}

	os.Setenv("NODE_NAME", control.ServerNodeName)
	control.DataDir = t.TempDir()
	testutil.GenerateRuntime(control)

	// add dummy handler for tunnel/proxy CONNECT requests, since we're not
	// setting up a whole remotedialer tunnel server here
	control.Runtime.Tunnel = http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {})

	// set up agentless node
	control.DisableAgent = true

	// add authenticator
	auth, err := authenticator.FromArgs([]string{
		"--basic-auth-file=" + control.Runtime.PasswdFile,
		"--client-ca-file=" + control.Runtime.ClientCA,
	})
	NewWithT(t).Expect(err).ToNot(HaveOccurred())
	control.Runtime.Authenticator = auth

	// finally, bind request handlers
	control.Runtime.Handler = NewHandler(ctx, control, &cmds.Server{})

	return control, cancel
}

// getMockedControl returns a Control structure with mocked core controllers in place
// of a full functional datastore and apiserver.
func getMockedControl(t *testing.T) (*config.Control, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	control := &config.Control{
		Token:          "token",
		AgentToken:     "agent-token",
		ServerNodeName: "k3s-server-1",
	}

	os.Setenv("NODE_NAME", control.ServerNodeName)
	control.DataDir = t.TempDir()
	testutil.GenerateRuntime(control)

	// add dummy handler for tunnel/proxy CONNECT requests, since we're not
	// setting up a whole remotedialer tunnel server here
	control.Runtime.Tunnel = http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {})

	// wire up mock controllers and cache stores
	secretStore := &mock.SecretStore{}
	nodeStore := &mock.NodeStore{}
	nodeStore.Create(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: control.ServerNodeName}})
	nodeStore.Create(&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "k3s-agent-1"}})

	ctrl := gomock.NewController(t)
	coreFactory := mock.NewCoreFactory(ctrl)
	coreFactory.CoreMock.V1Mock.SecretMock.EXPECT().Cache().AnyTimes().Return(coreFactory.CoreMock.V1Mock.SecretCache)
	coreFactory.CoreMock.V1Mock.SecretMock.EXPECT().Create(gomock.Any()).AnyTimes().DoAndReturn(secretStore.Create)
	coreFactory.CoreMock.V1Mock.SecretCache.EXPECT().Get(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(secretStore.Get)
	coreFactory.CoreMock.V1Mock.NodeMock.EXPECT().Cache().AnyTimes().Return(coreFactory.CoreMock.V1Mock.NodeCache)
	coreFactory.CoreMock.V1Mock.NodeCache.EXPECT().Get(gomock.Any()).AnyTimes().DoAndReturn(nodeStore.Get)
	control.Runtime.Core = coreFactory

	// add authenticator
	auth, err := authenticator.FromArgs([]string{
		"--basic-auth-file=" + control.Runtime.PasswdFile,
		"--client-ca-file=" + control.Runtime.ClientCA,
	})
	NewWithT(t).Expect(err).ToNot(HaveOccurred())
	control.Runtime.Authenticator = auth

	// finally, bind request handlers
	control.Runtime.Handler = NewHandler(ctx, control, &cmds.Server{})

	return control, cancel
}

func withClientCert(req *http.Request, certFile string) {
	bytes, err := os.ReadFile(certFile)
	if err != nil {
		panic(err)
	}
	certs, err := certutil.ParseCertsPEM(bytes)
	if err != nil {
		panic(err)
	}
	req.TLS = &tls.ConnectionState{
		PeerCertificates: certs,
	}
}

func withNewClientCert(req *http.Request, caCertFile, caKeyFile, signingKeyFile string, certConfig certutil.Config) {
	caCerts, caKey, err := getCACertAndKey(caCertFile, caKeyFile)
	if err != nil {
		panic(err)
	}
	keyBytes, err := os.ReadFile(signingKeyFile)
	if err != nil {
		panic(err)
	}
	key, err := certutil.ParsePrivateKeyPEM(keyBytes)
	if err != nil {
		panic(err)
	}
	cert, err := certutil.NewSignedCert(certConfig, key.(crypto.Signer), caCerts[0], caKey)
	if err != nil {
		panic(err)
	}

	req.TLS = &tls.ConnectionState{}
	req.TLS.PeerCertificates = append(req.TLS.PeerCertificates, cert)
	req.TLS.PeerCertificates = append(req.TLS.PeerCertificates, caCerts...)
}

func withCertificateRequest(req *http.Request) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, key)
	if err != nil {
		panic(err)
	}
	req.Body = io.NopCloser(bytes.NewReader(csr))
}

func withClientAddress(req *http.Request, address string) {
	req.RemoteAddr = net.JoinHostPort(address, "1234")
}
