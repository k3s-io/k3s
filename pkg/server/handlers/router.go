package handlers

import (
	"context"
	"net/http"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/nodepassword"
	"github.com/k3s-io/k3s/pkg/server/auth"
	"github.com/k3s-io/k3s/pkg/version"
	"k8s.io/apiserver/pkg/authentication/user"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
)

const (
	staticURL = "/static/"
)

func NewHandler(ctx context.Context, control *config.Control, cfg *cmds.Server) http.Handler {
	nodeAuth := nodepassword.GetNodeAuthValidator(ctx, control)

	prefix := "/v1-{program}"
	authed := mux.NewRouter().SkipClean(true)
	authed.NotFoundHandler = APIServer(control, cfg)
	authed.Use(auth.HasRole(control, version.Program+":agent", user.NodesGroup, bootstrapapi.BootstrapDefaultGroup))
	authed.Handle(prefix+"/serving-kubelet.crt", ServingKubeletCert(control, nodeAuth))
	authed.Handle(prefix+"/client-kubelet.crt", ClientKubeletCert(control, nodeAuth))
	authed.Handle(prefix+"/client-kube-proxy.crt", ClientKubeProxyCert(control))
	authed.Handle(prefix+"/client-{program}-controller.crt", ClientControllerCert(control))
	authed.Handle(prefix+"/client-ca.crt", File(control.Runtime.ClientCA))
	authed.Handle(prefix+"/server-ca.crt", File(control.Runtime.ServerCA))
	authed.Handle(prefix+"/apiservers", APIServers(control))
	authed.Handle(prefix+"/config", Config(control, cfg))
	authed.Handle(prefix+"/readyz", Readyz(control))

	nodeAuthed := mux.NewRouter().SkipClean(true)
	nodeAuthed.NotFoundHandler = authed
	nodeAuthed.Use(auth.HasRole(control, user.NodesGroup))
	nodeAuthed.Handle(prefix+"/connect", control.Runtime.Tunnel)

	serverAuthed := mux.NewRouter().SkipClean(true)
	serverAuthed.NotFoundHandler = nodeAuthed
	serverAuthed.Use(auth.HasRole(control, version.Program+":server"))
	serverAuthed.Handle(prefix+"/encrypt/status", EncryptionStatus(control))
	serverAuthed.Handle(prefix+"/encrypt/config", EncryptionConfig(ctx, control))
	serverAuthed.Handle(prefix+"/cert/cacerts", CACertReplace(control))
	serverAuthed.Handle(prefix+"/server-bootstrap", Bootstrap(control))
	serverAuthed.Handle(prefix+"/token", TokenRequest(ctx, control))

	systemAuthed := mux.NewRouter().SkipClean(true)
	systemAuthed.NotFoundHandler = serverAuthed
	systemAuthed.MethodNotAllowedHandler = serverAuthed
	systemAuthed.Use(auth.HasRole(control, user.SystemPrivilegedGroup))
	systemAuthed.Methods(http.MethodConnect).Handler(control.Runtime.Tunnel)

	router := mux.NewRouter().SkipClean(true)
	router.NotFoundHandler = systemAuthed
	router.PathPrefix(staticURL).Handler(Static(staticURL, filepath.Join(control.DataDir, "static")))
	router.Handle("/cacerts", CACerts(control))
	router.Handle("/ping", Ping())

	return router
}
