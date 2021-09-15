package endpoint

import (
	"log"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	etcdVersion = []byte(`{"etcdserver":"3.5.0","etcdcluster":"3.5.0"}`)
	versionPath = "/version"
)

// httpServer returns a HTTP server with the basic mux handler.
func httpServer() *http.Server {
	// Set up root HTTP mux with basic response handlers
	mux := http.NewServeMux()
	handleBasic(mux)

	return &http.Server{
		Handler:  mux,
		ErrorLog: log.New(logrus.StandardLogger().Writer(), "kinehttp ", log.LstdFlags),
	}
}

// handleBasic binds basic HTTP response handlers to a mux.
func handleBasic(mux *http.ServeMux) {
	mux.HandleFunc(versionPath, serveVersion)
}

// serveVersion responds with a canned JSON version response.
func serveVersion(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(etcdVersion)
}

// allowMethod returns true if a method is allowed, or false (after sending a
// MethodNotAllowed error to the client) if it is not.
func allowMethod(w http.ResponseWriter, r *http.Request, m string) bool {
	if m == r.Method {
		return true
	}
	w.Header().Set("Allow", m)
	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	return false
}

// grpcHandlerFunc takes a GRPC server and HTTP handler, and returns a handler
// function that will route GRPC requests to the GRPC server, and everything
// else to the HTTP handler. This is based on sample code provided in the GRPC
// ServeHTTP documentation for sharing a port between GRPC and HTTP handlers.
func grpcHandlerFunc(grpcServer *grpc.Server, httpHandler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			httpHandler.ServeHTTP(w, r)
		}
	})
}
