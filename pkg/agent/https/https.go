package https

import (
	"context"
	"net/http"
	"strconv"
	"sync"

	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/generated/clientset/versioned/scheme"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"k8s.io/apiserver/pkg/apis/apiserver"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/options"
)

// RouterFunc provides a hook for components to register additional routes to a request router
type RouterFunc func(ctx context.Context, nodeConfig *config.Node) (*mux.Router, error)

var once sync.Once
var router *mux.Router
var err error

// Start returns a router with authn/authz filters applied.
// The first time it is called, the router is created and a new HTTPS listener is started if the handler is nil.
// Subsequent calls will return the same router.
func Start(ctx context.Context, nodeConfig *config.Node, runtime *config.ControlRuntime) (*mux.Router, error) {
	once.Do(func() {
		router = mux.NewRouter().SkipClean(true)
		config := server.Config{}

		if runtime == nil {
			// If we do not have an existing handler, set up a new listener
			tcp, lerr := util.ListenWithLoopback(ctx, nodeConfig.AgentConfig.ListenAddress, strconv.Itoa(nodeConfig.SupervisorPort))
			if lerr != nil {
				err = lerr
				return
			}

			serving := options.NewSecureServingOptions()
			serving.Listener = tcp
			serving.CipherSuites = nodeConfig.AgentConfig.CipherSuites
			serving.MinTLSVersion = nodeConfig.AgentConfig.MinTLSVersion
			serving.ServerCert = options.GeneratableKeyCert{
				CertKey: options.CertKey{
					CertFile: nodeConfig.AgentConfig.ServingKubeletCert,
					KeyFile:  nodeConfig.AgentConfig.ServingKubeletKey,
				},
			}
			if aerr := serving.ApplyTo(&config.SecureServing); aerr != nil {
				err = aerr
				return
			}
		} else {
			// If we have an existing handler, wrap it
			router.NotFoundHandler = runtime.Handler
			runtime.Handler = router
		}

		authn := options.NewDelegatingAuthenticationOptions()
		authn.Anonymous = &apiserver.AnonymousAuthConfig{
			Enabled: false,
		}
		authn.SkipInClusterLookup = true
		authn.ClientCert = options.ClientCertAuthenticationOptions{
			ClientCA: nodeConfig.AgentConfig.ClientCA,
		}
		authn.RemoteKubeConfigFile = nodeConfig.AgentConfig.KubeConfigKubelet
		if applyErr := authn.ApplyTo(&config.Authentication, config.SecureServing, nil); applyErr != nil {
			err = applyErr
			return
		}

		authz := options.NewDelegatingAuthorizationOptions()
		authz.AlwaysAllowPaths = []string{ // skip authz for paths that should not use SubjectAccessReview; basically everything that will use this router other than metrics
			"/v1-" + version.Program + "/p2p", // spegel libp2p peer discovery
			"/v2/*",                           // spegel registry mirror
			"/debug/pprof/*",                  // profiling
		}
		authz.RemoteKubeConfigFile = nodeConfig.AgentConfig.KubeConfigKubelet
		if applyErr := authz.ApplyTo(&config.Authorization); applyErr != nil {
			err = applyErr
			return
		}

		router.Use(filterChain(config.Authentication.Authenticator, config.Authorization.Authorizer))

		if config.SecureServing != nil {
			_, _, err = config.SecureServing.Serve(router, 0, ctx.Done())
		}
	})

	return router, err
}

// filterChain runs the kubernetes authn/authz filter chain using the mux middleware API
func filterChain(authn authenticator.Request, authz authorizer.Authorizer) mux.MiddlewareFunc {
	return func(handler http.Handler) http.Handler {
		requestInfoResolver := &apirequest.RequestInfoFactory{}
		failedHandler := genericapifilters.Unauthorized(scheme.Codecs)
		handler = genericapifilters.WithAuthorization(handler, authz, scheme.Codecs)
		handler = genericapifilters.WithAuthentication(handler, authn, failedHandler, nil, nil)
		handler = genericapifilters.WithRequestInfo(handler, requestInfoResolver)
		handler = genericapifilters.WithCacheControl(handler)
		return handler
	}
}
