package health

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func Test_UnitTCPChecker(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	c := TCP("ok", ln.Addr().String())
	if c.Name() != "ok" {
		t.Errorf("Name() = %q, want %q", c.Name(), "ok")
	}
	if err := c.Check(nil); err != nil {
		t.Errorf("expected open port to pass, got %v", err)
	}
	ln.Close()
	if err := TCP("closed", ln.Addr().String()).Check(nil); err == nil {
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

	if err := HTTPGet("ok", srv.URL+"/ok").Check(nil); err != nil {
		t.Errorf("expected 200 to pass, got %v", err)
	}
	if err := HTTPGet("fail", srv.URL+"/fail").Check(nil); err == nil {
		t.Errorf("expected 500 to fail")
	}
	if err := HTTPGet("dial", "http://127.0.0.1:1/never").Check(nil); err == nil {
		t.Errorf("expected unreachable URL to fail")
	}
}

func Test_UnitHTTPGetCheckerSkipsTLSVerify(t *testing.T) {
	// httptest.NewTLSServer uses a self-signed cert; the checker must accept it.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	if err := HTTPGet("tls", srv.URL+"/livez").Check(nil); err != nil {
		t.Errorf("expected TLS-skip-verify probe to pass against self-signed cert, got %v", err)
	}
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
	if err := GRPC("cri", socket).Check(nil); err != nil {
		t.Errorf("expected SERVING status to pass, got %v", err)
	}
}

func Test_UnitGRPCCheckerNotServing(t *testing.T) {
	socket := startHealthServer(t, healthpb.HealthCheckResponse_NOT_SERVING)
	if err := GRPC("cri", socket).Check(nil); err == nil {
		t.Errorf("expected NOT_SERVING status to fail")
	}
}

func Test_UnitGRPCCheckerStripsUnixScheme(t *testing.T) {
	socket := startHealthServer(t, healthpb.HealthCheckResponse_SERVING)
	if err := GRPC("cri", "unix://"+socket).Check(nil); err != nil {
		t.Errorf("expected unix:// scheme to be stripped, got %v", err)
	}
}

func Test_UnitGRPCCheckerMissingSocket(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "absent.sock")
	if err := GRPC("cri", socket).Check(nil); err == nil {
		t.Errorf("expected missing socket to fail")
	}
}
