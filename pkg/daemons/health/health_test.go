package health

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func Test_UnitGroupCheckAll(t *testing.T) {
	boom := errors.New("boom")
	g := NewGroup()
	g.Add(
		Func{ComponentName: "first", Probe: func(_ context.Context) error { return nil }},
		Func{ComponentName: "second", Probe: func(_ context.Context) error { return boom }},
		Func{ComponentName: "third", Probe: func(_ context.Context) error { t.Fatal("third should not run after second failed"); return nil }},
	)
	name, err := g.CheckAll(context.Background())
	if name != "second" {
		t.Errorf("expected first failure to be %q, got %q", "second", name)
	}
	if !errors.Is(err, boom) {
		t.Errorf("expected wrapped boom error, got %v", err)
	}
}

func Test_UnitGroupCheckAllPasses(t *testing.T) {
	g := NewGroup()
	g.Add(
		Func{ComponentName: "a", Probe: func(_ context.Context) error { return nil }},
		Func{ComponentName: "b", Probe: func(_ context.Context) error { return nil }},
	)
	if name, err := g.CheckAll(context.Background()); name != "" || err != nil {
		t.Errorf("expected ('', nil), got (%q, %v)", name, err)
	}
}

func Test_UnitGroupAddSkipsNil(t *testing.T) {
	g := NewGroup()
	g.Add(nil, Func{ComponentName: "x", Probe: func(_ context.Context) error { return nil }}, nil)
	if g.Len() != 1 {
		t.Errorf("expected nil checkers to be skipped; got Len=%d", g.Len())
	}
	if names := g.Names(); len(names) != 1 || names[0] != "x" {
		t.Errorf("unexpected names: %v", names)
	}
}

func Test_UnitTCPChecker(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	if err := TCP("ok", ln.Addr().String()).Check(context.Background()); err != nil {
		t.Errorf("expected open port to pass, got %v", err)
	}
	ln.Close()
	if err := TCP("closed", ln.Addr().String()).Check(context.Background()); err == nil {
		t.Errorf("expected closed port to fail")
	}
}

func Test_UnitUnixSocketChecker(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "test.sock")

	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen unix: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	if err := UnixSocket("ok", socket).Check(context.Background()); err != nil {
		t.Errorf("expected open socket to pass, got %v", err)
	}

	ln.Close()
	_ = os.Remove(socket)
	if err := UnixSocket("missing", socket).Check(context.Background()); err == nil {
		t.Errorf("expected missing socket to fail")
	}
}

func Test_UnitHTTPGetChecker(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/fail":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)

	if err := HTTPGet("ok", srv.URL+"/ok").Check(context.Background()); err != nil {
		t.Errorf("expected 200 response to pass, got %v", err)
	}
	if err := HTTPGet("fail", srv.URL+"/fail").Check(context.Background()); err == nil {
		t.Errorf("expected 500 response to fail")
	}
	if err := HTTPGet("dial", "http://127.0.0.1:1/never").Check(context.Background()); err == nil {
		t.Errorf("expected unreachable URL to fail")
	}
}

func Test_UnitHTTPGetCheckerSkipsTLSVerify(t *testing.T) {
	// httptest.NewTLSServer uses a self-signed cert; the checker must accept it.
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	if err := HTTPGet("tls", srv.URL+"/livez").Check(context.Background()); err != nil {
		t.Errorf("expected TLS-skip-verify probe to pass against self-signed cert, got %v", err)
	}
}
