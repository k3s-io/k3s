package health

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func Test_UnitTCPChecker(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	c := NewTCPConnectHealthz("ok", ln.Addr().String())
	if c.Name() != "ok" {
		t.Errorf("Name() = %q, want %q", c.Name(), "ok")
	}
	if err := c.Check(nil); err != nil {
		t.Errorf("expected open port to pass, got %v", err)
	}
	ln.Close()
	if err := NewTCPConnectHealthz("closed", ln.Addr().String()).Check(nil); err == nil {
		t.Errorf("expected closed port to fail")
	}
}

func Test_UnitHTTPGetChecker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)

	if err := NewHTTPGetHealthz("ok", srv.URL+"/ok").Check(nil); err != nil {
		t.Errorf("expected 200 to pass, got %v", err)
	}
	if err := NewHTTPGetHealthz("fail", srv.URL+"/fail").Check(nil); err == nil {
		t.Errorf("expected 500 to fail")
	}
	if err := NewHTTPGetHealthz("dial", "http://127.0.0.1:1/never").Check(nil); err == nil {
		t.Errorf("expected unreachable URL to fail")
	}
}

func Test_UnitHTTPGetCheckerSkipsTLSVerify(t *testing.T) {
	// httptest.NewTLSServer uses a self-signed cert; the checker must accept it.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	if err := NewHTTPGetHealthz("tls", srv.URL+"/livez").Check(nil); err != nil {
		t.Errorf("expected TLS-skip-verify probe to pass against self-signed cert, got %v", err)
	}
}

func Test_UnitHTTPGetWithClientCertChecker(t *testing.T) {
	// TLS server that requires (and inspects) a client cert.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.TLS.PeerCertificates) == 0 {
			http.Error(w, "no client cert", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	srv.TLS = &tls.Config{ClientAuth: tls.RequireAnyClientCert}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	// Generate a throwaway client cert + key on disk.
	certPath, keyPath := writeSelfSignedCert(t)

	if err := NewHTTPGetWithClientCertHealthz("apiserver", srv.URL+"/livez", certPath, keyPath).Check(nil); err != nil {
		t.Errorf("expected probe with client cert to pass, got %v", err)
	}

	// Without a cert the same endpoint should 401.
	if err := NewHTTPGetHealthz("apiserver-anon", srv.URL+"/livez").Check(nil); err == nil {
		t.Errorf("expected anonymous probe to fail without client cert")
	}
}

func Test_UnitHTTPGetWithClientCertMissingFiles(t *testing.T) {
	// Cert path doesn't exist — Check should return an error every call,
	// not panic.
	c := NewHTTPGetWithClientCertHealthz("apiserver", "https://127.0.0.1:1/livez", "/nonexistent.crt", "/nonexistent.key")
	if err := c.Check(nil); err == nil {
		t.Errorf("expected missing cert files to surface as a Check error")
	}
}

// writeSelfSignedCert generates an in-memory self-signed cert and writes the
// PEM-encoded cert and key to a tempdir. Returns their paths.
func writeSelfSignedCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-client"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	dir := t.TempDir()
	certPath = filepath.Join(dir, "client.crt")
	keyPath = filepath.Join(dir, "client.key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatalf("write cert: %v", err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return certPath, keyPath
}

// healthServer is a minimal implementation of grpc.health.v1.Health that
// returns a configurable status — used to test the gRPC health checker
// against both SERVING and NOT_SERVING responses.
type healthServer struct {
	healthpb.UnimplementedHealthServer
	status healthpb.HealthCheckResponse_ServingStatus
}

func (h *healthServer) Check(_ context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: h.status}, nil
}

func startHealthServer(t *testing.T, status healthpb.HealthCheckResponse_ServingStatus) string {
	t.Helper()
	dir := t.TempDir()
	socket := filepath.Join(dir, "grpc.sock")

	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, &healthServer{status: status})
	go srv.Serve(ln)
	t.Cleanup(func() {
		srv.Stop()
		ln.Close()
	})
	return socket
}

func Test_UnitGRPCCheckerServing(t *testing.T) {
	socket := startHealthServer(t, healthpb.HealthCheckResponse_SERVING)
	if err := NewGRPCHealthz("cri", socket).Check(nil); err != nil {
		t.Errorf("expected SERVING status to pass, got %v", err)
	}
}

func Test_UnitGRPCCheckerNotServing(t *testing.T) {
	socket := startHealthServer(t, healthpb.HealthCheckResponse_NOT_SERVING)
	if err := NewGRPCHealthz("cri", socket).Check(nil); err == nil {
		t.Errorf("expected NOT_SERVING status to fail")
	}
}

func Test_UnitGRPCCheckerStripsUnixScheme(t *testing.T) {
	socket := startHealthServer(t, healthpb.HealthCheckResponse_SERVING)
	if err := NewGRPCHealthz("cri", "unix://"+socket).Check(nil); err != nil {
		t.Errorf("expected unix:// scheme to be stripped, got %v", err)
	}
}

func Test_UnitGRPCCheckerMissingSocket(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "absent.sock")
	if err := NewGRPCHealthz("cri", socket).Check(nil); err == nil {
		t.Errorf("expected missing socket to fail")
	}
}
