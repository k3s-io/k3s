package handlers

import (
	"context"
	"crypto"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/bootstrap"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/nodepassword"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
	certutil "github.com/rancher/dynamiclistener/cert"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func CACerts(config *config.Control) http.Handler {
	var ca []byte
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if ca == nil {
			var err error
			ca, err = os.ReadFile(config.Runtime.ServerCA)
			if err != nil {
				util.SendError(err, resp, req)
				return
			}
		}
		resp.Header().Set("content-type", "text/plain")
		resp.Write(ca)
	})
}

func Ping() http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		data := []byte("pong")
		resp.WriteHeader(http.StatusOK)
		resp.Header().Set("Content-Type", "text/plain")
		resp.Header().Set("Content-Length", strconv.Itoa(len(data)))
		resp.Write(data)
	})
}

func ServingKubeletCert(control *config.Control, auth nodepassword.NodeAuthValidator) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		nodeName, errCode, err := auth(req)
		if err != nil {
			util.SendError(err, resp, req, errCode)
			return
		}

		ips := []net.IP{net.ParseIP("127.0.0.1")}
		program := mux.Vars(req)["program"]
		if nodeIP := req.Header.Get(program + "-Node-IP"); nodeIP != "" {
			for _, v := range strings.Split(nodeIP, ",") {
				ip := net.ParseIP(v)
				if ip == nil {
					util.SendError(fmt.Errorf("invalid node IP address %s", ip), resp, req)
					return
				}
				ips = append(ips, ip)
			}
		}

		signAndSend(resp, req, control.Runtime.ServerCA, control.Runtime.ServerCAKey, control.Runtime.ServingKubeletKey, certutil.Config{
			CommonName: nodeName,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			AltNames: certutil.AltNames{
				DNSNames: []string{nodeName, "localhost"},
				IPs:      ips,
			},
		})
	})
}

func ClientKubeletCert(control *config.Control, auth nodepassword.NodeAuthValidator) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		nodeName, errCode, err := auth(req)
		if err != nil {
			util.SendError(err, resp, req, errCode)
			return
		}
		signAndSend(resp, req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeProxyKey, certutil.Config{
			CommonName:   "system:node:" + nodeName,
			Organization: []string{user.NodesGroup},
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
	})
}

func ClientKubeProxyCert(control *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		signAndSend(resp, req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientKubeProxyKey, certutil.Config{
			CommonName: user.KubeProxy,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
	})
}

func ClientControllerCert(control *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		program := mux.Vars(req)["program"]
		signAndSend(resp, req, control.Runtime.ClientCA, control.Runtime.ClientCAKey, control.Runtime.ClientK3sControllerKey, certutil.Config{
			CommonName: "system:" + program + "-controller",
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		})
	})
}

func File(fileName ...string) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		resp.Header().Set("Content-Type", "text/plain")

		if len(fileName) == 1 {
			http.ServeFile(resp, req, fileName[0])
			return
		}

		for _, f := range fileName {
			bytes, err := os.ReadFile(f)
			if err != nil {
				util.SendError(errors.Wrapf(err, "failed to read %s", f), resp, req, http.StatusInternalServerError)
				return
			}
			resp.Write(bytes)
		}
	})
}

func APIServer(control *config.Control, cfg *cmds.Server) http.Handler {
	if cfg.DisableAPIServer {
		return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
			util.SendError(util.ErrAPIDisabled, resp, req, http.StatusServiceUnavailable)
		})
	}
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if control.Runtime != nil && control.Runtime.APIServer != nil {
			control.Runtime.APIServer.ServeHTTP(resp, req)
		} else {
			util.SendError(util.ErrAPINotReady, resp, req, http.StatusServiceUnavailable)
		}
	})
}

func APIServers(control *config.Control) http.Handler {
	collectAddresses := getAddressCollector(control)
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 5*time.Second)
		defer cancel()
		endpoints := collectAddresses(ctx)
		resp.Header().Set("content-type", "application/json")
		if err := json.NewEncoder(resp).Encode(endpoints); err != nil {
			util.SendError(errors.Wrap(err, "failed to encode apiserver endpoints"), resp, req, http.StatusInternalServerError)
		}
	})
}

func Config(control *config.Control, cfg *cmds.Server) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		// Startup hooks may read and modify cmds.Server in a goroutine, but as these are copied into
		// config.Control before the startup hooks are called, any modifications need to be sync'd back
		// into the struct before it is sent to agents.
		// At this time we don't sync all the fields, just those known to be touched by startup hooks.
		control.DisableKubeProxy = cfg.DisableKubeProxy
		resp.Header().Set("content-type", "application/json")
		if err := json.NewEncoder(resp).Encode(control); err != nil {
			util.SendError(errors.Wrap(err, "failed to encode agent config"), resp, req, http.StatusInternalServerError)
		}
	})
}

func Readyz(control *config.Control) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		if control.Runtime.Core == nil {
			util.SendError(util.ErrCoreNotReady, resp, req, http.StatusServiceUnavailable)
			return
		}
		data := []byte("ok")
		resp.WriteHeader(http.StatusOK)
		resp.Header().Set("Content-Type", "text/plain")
		resp.Header().Set("Content-Length", strconv.Itoa(len(data)))
		resp.Write(data)
	})
}

func Bootstrap(control *config.Control) http.Handler {
	if control.Runtime.HTTPBootstrap {
		return bootstrap.Handler(&control.Runtime.ControlRuntimeBootstrap)
	}
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		logrus.Warnf("Received HTTP bootstrap request from %s, but embedded etcd is not enabled.", req.RemoteAddr)
		util.SendError(errors.New("etcd disabled"), resp, req, http.StatusBadRequest)
	})
}

func Static(urlPrefix, staticDir string) http.Handler {
	return http.StripPrefix(urlPrefix, http.FileServer(http.Dir(staticDir)))
}

// csrSigner wraps a CSR with a Public() method and dummy Sign() method to satisfy the
// crypto.Signer interface required by dynamiclistener's cert helpers.
type csrSigner struct {
	csr *x509.CertificateRequest
}

func (c *csrSigner) Public() crypto.PublicKey {
	return c.csr.PublicKey
}

func (c csrSigner) Sign(_ io.Reader, _ []byte, _ crypto.SignerOpts) ([]byte, error) {
	return nil, errors.New("not implemented")
}

// signAndSend generates a signed certificate using the requested CA cert/key and config,
// and sends it to the client.  If the client request is a POST with a signing request as
// the body, the public key from the CSR is used to generate the certificate. If the
// client did not submit a signing request, the legacy shared key is used to generate the
// certificate, and the key is sent along with the certificate.
func signAndSend(resp http.ResponseWriter, req *http.Request, caCertFile, caKeyFile, signingKeyFile string, certConfig certutil.Config) {
	caCerts, caKey, err := getCACertAndKey(caCertFile, caKeyFile)
	if err != nil {
		util.SendError(err, resp, req)
		return
	}

	var key crypto.Signer
	var keyBytes []byte
	if csr, err := getCSR(req); err == nil {
		// If the client sent a valid CSR, use the CSR to retrieve the public key
		key = &csrSigner{csr: csr}
	} else {
		// For legacy clients, just use the common key
		keyBytes, err = os.ReadFile(signingKeyFile)
		if err != nil {
			util.SendError(err, resp, req)
			return
		}
		pk, err := certutil.ParsePrivateKeyPEM(keyBytes)
		if err != nil {
			util.SendError(err, resp, req)
			return
		}
		key = pk.(crypto.Signer)
	}

	// create the signed cert using dynamiclistener cert utils
	cert, err := certutil.NewSignedCert(certConfig, key, caCerts[0], caKey)
	if err != nil {
		util.SendError(err, resp, req)
		return
	}

	// send the cert and CA bundle
	resp.Write(util.EncodeCertsPEM(cert, caCerts))

	// also send the common private key, if the client didn't send a CSR
	if len(keyBytes) > 0 {
		resp.Write(keyBytes)
	}
}

// getCACertAndKey loads the CA bundle and key at the specified paths.
func getCACertAndKey(caCertFile, caKeyFile string) ([]*x509.Certificate, crypto.Signer, error) {
	caKeyBytes, err := os.ReadFile(caKeyFile)
	if err != nil {
		return nil, nil, err
	}

	caKey, err := certutil.ParsePrivateKeyPEM(caKeyBytes)
	if err != nil {
		return nil, nil, err
	}

	caBytes, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, nil, err
	}

	caCert, err := certutil.ParseCertsPEM(caBytes)
	if err != nil {
		return nil, nil, err
	}

	return caCert, caKey.(crypto.Signer), nil
}

// getCSR decodes a x509.CertificateRequest from a POST request body.
// If the request is not a POST, or cannot be parsed as a request, an error is returned.
func getCSR(req *http.Request) (*x509.CertificateRequest, error) {
	if req.Method != http.MethodPost {
		return nil, mux.ErrMethodMismatch
	}
	csrBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificateRequest(csrBytes)
}

// addressGetter is a common signature for functions that return an address channel
type addressGetter func(ctx context.Context) <-chan []string

// kubernetesGetter returns a function that returns a channel that can be read to get apiserver addresses from kubernetes endpoints
func kubernetesGetter(control *config.Control) addressGetter {
	var endpointsClient typedcorev1.EndpointsInterface
	return func(ctx context.Context) <-chan []string {
		ch := make(chan []string, 1)
		go func() {
			if endpointsClient == nil {
				if control.Runtime.K8s != nil {
					endpointsClient = control.Runtime.K8s.CoreV1().Endpoints(metav1.NamespaceDefault)
				}
			}
			if endpointsClient != nil {
				if endpoint, err := endpointsClient.Get(ctx, "kubernetes", metav1.GetOptions{}); err != nil {
					logrus.Debugf("Failed to get apiserver addresses from kubernetes: %v", err)
				} else {
					ch <- util.GetAddresses(endpoint)
				}
			}
			close(ch)
		}()
		return ch
	}
}

// etcdGetter returns a function that returns a channel that can be read to get apiserver addresses from etcd
func etcdGetter(control *config.Control) addressGetter {
	return func(ctx context.Context) <-chan []string {
		ch := make(chan []string, 1)
		go func() {
			if addresses, err := etcd.GetAPIServerURLsFromETCD(ctx, control); err != nil {
				logrus.Debugf("Failed to get apiserver addresses from etcd: %v", err)
			} else {
				ch <- addresses
			}
			close(ch)
		}()
		return ch
	}
}

// getAddressCollector returns a function that can be called to return
// apiserver addresses from both kubernetes and etcd
func getAddressCollector(control *config.Control) func(ctx context.Context) []string {
	getFromKubernetes := kubernetesGetter(control)
	getFromEtcd := etcdGetter(control)

	// read from both kubernetes and etcd in parallel, returning the collected results
	return func(ctx context.Context) []string {
		a := sets.Set[string]{}
		r := []string{}
		k8sCh := getFromKubernetes(ctx)
		k8sOk := true
		etcdCh := getFromEtcd(ctx)
		etcdOk := true

		for k8sOk || etcdOk {
			select {
			case r, k8sOk = <-k8sCh:
				a.Insert(r...)
			case r, etcdOk = <-etcdCh:
				a.Insert(r...)
			case <-ctx.Done():
				return a.UnsortedList()
			}
		}
		return a.UnsortedList()
	}
}
