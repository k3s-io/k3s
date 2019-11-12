package dqlite

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"net/http"

	"github.com/canonical/go-dqlite"
	"github.com/canonical/go-dqlite/client"
	"github.com/gorilla/mux"
)

func router(ctx context.Context, next http.Handler, nodeInfo dqlite.NodeInfo, clientCA *x509.Certificate, clientCN string, bindAddress string) http.Handler {
	mux := mux.NewRouter()
	mux.Handle("/db/connect", newChecker(newProxy(ctx, bindAddress), clientCA, clientCN))
	mux.Handle("/db/info", infoHandler(ctx, nodeInfo, bindAddress))
	mux.NotFoundHandler = next
	return mux
}

func infoHandler(ctx context.Context, nodeInfo dqlite.NodeInfo, bindAddress string) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		client, err := client.New(ctx, bindAddress, client.WithLogFunc(log()))
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		defer client.Close()

		info, err := client.Cluster(ctx)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(info)
	})
}

type checker struct {
	next   http.Handler
	verify x509.VerifyOptions
	cn     string
}

func newChecker(next http.Handler, ca *x509.Certificate, cn string) http.Handler {
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return &checker{
		next: next,
		verify: x509.VerifyOptions{
			Roots: pool,
			KeyUsages: []x509.ExtKeyUsage{
				x509.ExtKeyUsageClientAuth,
			},
			DNSName: cn,
		},
		cn: cn,
	}
}

func (c *checker) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if !c.check(req) {
		http.Error(rw, "unauthorized", http.StatusUnauthorized)
		return
	}
	c.next.ServeHTTP(rw, req)
}

func (c *checker) check(r *http.Request) bool {
	for _, cert := range r.TLS.PeerCertificates {
		_, err := cert.Verify(c.verify)
		if err == nil {
			return cert.Subject.CommonName == c.cn
		}
	}
	return false
}
