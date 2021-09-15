/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package server

import (
	"fmt"
	"net"
	"net/http"
	goruntime "runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/sets"
	utilwaitgroup "k8s.io/apimachinery/pkg/util/waitgroup"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/audit"
	auditpolicy "k8s.io/apiserver/pkg/audit/policy"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	authenticatorunion "k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	authorizerunion "k8s.io/apiserver/pkg/authorization/union"
	"k8s.io/apiserver/pkg/endpoints/discovery"
	"k8s.io/apiserver/pkg/endpoints/filterlatency"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apiopenapi "k8s.io/apiserver/pkg/endpoints/openapi"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/features"
	genericfeatures "k8s.io/apiserver/pkg/features"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/apiserver/pkg/server/egressselector"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apiserver/pkg/server/routes"
	serverstore "k8s.io/apiserver/pkg/server/storage"
	"k8s.io/apiserver/pkg/storageversion"
	"k8s.io/apiserver/pkg/util/feature"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	utilflowcontrol "k8s.io/apiserver/pkg/util/flowcontrol"
	flowcontrolrequest "k8s.io/apiserver/pkg/util/flowcontrol/request"
	"k8s.io/client-go/informers"
	restclient "k8s.io/client-go/rest"
	"k8s.io/component-base/logs"
	"k8s.io/klog/v2"
	openapicommon "k8s.io/kube-openapi/pkg/common"
	"k8s.io/kube-openapi/pkg/validation/spec"
	utilsnet "k8s.io/utils/net"

	// install apis
	_ "k8s.io/apiserver/pkg/apis/apiserver/install"
)

const (
	// DefaultLegacyAPIPrefix is where the legacy APIs will be located.
	DefaultLegacyAPIPrefix = "/api"

	// APIGroupPrefix is where non-legacy API group will be located.
	APIGroupPrefix = "/apis"
)

// Config is a structure used to configure a GenericAPIServer.
// Its members are sorted roughly in order of importance for composers.
type Config struct {
	// SecureServing is required to serve https
	SecureServing *SecureServingInfo

	// Authentication is the configuration for authentication
	Authentication AuthenticationInfo

	// Authorization is the configuration for authorization
	Authorization AuthorizationInfo

	// LoopbackClientConfig is a config for a privileged loopback connection to the API server
	// This is required for proper functioning of the PostStartHooks on a GenericAPIServer
	// TODO: move into SecureServing(WithLoopback) as soon as insecure serving is gone
	LoopbackClientConfig *restclient.Config

	// EgressSelector provides a lookup mechanism for dialing outbound connections.
	// It does so based on a EgressSelectorConfiguration which was read at startup.
	EgressSelector *egressselector.EgressSelector

	// RuleResolver is required to get the list of rules that apply to a given user
	// in a given namespace
	RuleResolver authorizer.RuleResolver
	// AdmissionControl performs deep inspection of a given request (including content)
	// to set values and determine whether its allowed
	AdmissionControl      admission.Interface
	CorsAllowedOriginList []string
	HSTSDirectives        []string
	// FlowControl, if not nil, gives priority and fairness to request handling
	FlowControl utilflowcontrol.Interface

	EnableIndex     bool
	EnableProfiling bool
	EnableDiscovery bool
	// Requires generic profiling enabled
	EnableContentionProfiling bool
	EnableMetrics             bool

	DisabledPostStartHooks sets.String
	// done values in this values for this map are ignored.
	PostStartHooks map[string]PostStartHookConfigEntry

	// Version will enable the /version endpoint if non-nil
	Version *version.Info
	// AuditBackend is where audit events are sent to.
	AuditBackend audit.Backend
	// AuditPolicyChecker makes the decision of whether and how to audit log a request.
	AuditPolicyChecker auditpolicy.Checker
	// ExternalAddress is the host name to use for external (public internet) facing URLs (e.g. Swagger)
	// Will default to a value based on secure serving info and available ipv4 IPs.
	ExternalAddress string

	// TracerProvider can provide a tracer, which records spans for distributed tracing.
	TracerProvider *trace.TracerProvider

	//===========================================================================
	// Fields you probably don't care about changing
	//===========================================================================

	// BuildHandlerChainFunc allows you to build custom handler chains by decorating the apiHandler.
	BuildHandlerChainFunc func(apiHandler http.Handler, c *Config) (secure http.Handler)
	// HandlerChainWaitGroup allows you to wait for all chain handlers exit after the server shutdown.
	HandlerChainWaitGroup *utilwaitgroup.SafeWaitGroup
	// DiscoveryAddresses is used to build the IPs pass to discovery. If nil, the ExternalAddress is
	// always reported
	DiscoveryAddresses discovery.Addresses
	// The default set of healthz checks. There might be more added via AddHealthChecks dynamically.
	HealthzChecks []healthz.HealthChecker
	// The default set of livez checks. There might be more added via AddHealthChecks dynamically.
	LivezChecks []healthz.HealthChecker
	// The default set of readyz-only checks. There might be more added via AddReadyzChecks dynamically.
	ReadyzChecks []healthz.HealthChecker
	// LegacyAPIGroupPrefixes is used to set up URL parsing for authorization and for validating requests
	// to InstallLegacyAPIGroup. New API servers don't generally have legacy groups at all.
	LegacyAPIGroupPrefixes sets.String
	// RequestInfoResolver is used to assign attributes (used by admission and authorization) based on a request URL.
	// Use-cases that are like kubelets may need to customize this.
	RequestInfoResolver apirequest.RequestInfoResolver
	// Serializer is required and provides the interface for serializing and converting objects to and from the wire
	// The default (api.Codecs) usually works fine.
	Serializer runtime.NegotiatedSerializer
	// OpenAPIConfig will be used in generating OpenAPI spec. This is nil by default. Use DefaultOpenAPIConfig for "working" defaults.
	OpenAPIConfig *openapicommon.Config
	// SkipOpenAPIInstallation avoids installing the OpenAPI handler if set to true.
	SkipOpenAPIInstallation bool

	// RESTOptionsGetter is used to construct RESTStorage types via the generic registry.
	RESTOptionsGetter genericregistry.RESTOptionsGetter

	// If specified, all requests except those which match the LongRunningFunc predicate will timeout
	// after this duration.
	RequestTimeout time.Duration
	// If specified, long running requests such as watch will be allocated a random timeout between this value, and
	// twice this value.  Note that it is up to the request handlers to ignore or honor this timeout. In seconds.
	MinRequestTimeout int

	// This represents the maximum amount of time it should take for apiserver to complete its startup
	// sequence and become healthy. From apiserver's start time to when this amount of time has
	// elapsed, /livez will assume that unfinished post-start hooks will complete successfully and
	// therefore return true.
	LivezGracePeriod time.Duration
	// ShutdownDelayDuration allows to block shutdown for some time, e.g. until endpoints pointing to this API server
	// have converged on all node. During this time, the API server keeps serving, /healthz will return 200,
	// but /readyz will return failure.
	ShutdownDelayDuration time.Duration

	// The limit on the total size increase all "copy" operations in a json
	// patch may cause.
	// This affects all places that applies json patch in the binary.
	JSONPatchMaxCopyBytes int64
	// The limit on the request size that would be accepted and decoded in a write request
	// 0 means no limit.
	MaxRequestBodyBytes int64
	// MaxRequestsInFlight is the maximum number of parallel non-long-running requests. Every further
	// request has to wait. Applies only to non-mutating requests.
	MaxRequestsInFlight int
	// MaxMutatingRequestsInFlight is the maximum number of parallel mutating requests. Every further
	// request has to wait.
	MaxMutatingRequestsInFlight int
	// Predicate which is true for paths of long-running http requests
	LongRunningFunc apirequest.LongRunningRequestCheck

	// GoawayChance is the probability that send a GOAWAY to HTTP/2 clients. When client received
	// GOAWAY, the in-flight requests will not be affected and new requests will use
	// a new TCP connection to triggering re-balancing to another server behind the load balance.
	// Default to 0, means never send GOAWAY. Max is 0.02 to prevent break the apiserver.
	GoawayChance float64

	// MergedResourceConfig indicates which groupVersion enabled and its resources enabled/disabled.
	// This is composed of genericapiserver defaultAPIResourceConfig and those parsed from flags.
	// If not specify any in flags, then genericapiserver will only enable defaultAPIResourceConfig.
	MergedResourceConfig *serverstore.ResourceConfig

	// RequestWidthEstimator is used to estimate the "width" of the incoming request(s).
	RequestWidthEstimator flowcontrolrequest.WidthEstimatorFunc

	// lifecycleSignals provides access to the various signals
	// that happen during lifecycle of the apiserver.
	// it's intentionally marked private as it should never be overridden.
	lifecycleSignals lifecycleSignals

	//===========================================================================
	// values below here are targets for removal
	//===========================================================================

	// PublicAddress is the IP address where members of the cluster (kubelet,
	// kube-proxy, services, etc.) can reach the GenericAPIServer.
	// If nil or 0.0.0.0, the host's default interface will be used.
	PublicAddress net.IP

	// EquivalentResourceRegistry provides information about resources equivalent to a given resource,
	// and the kind associated with a given resource. As resources are installed, they are registered here.
	EquivalentResourceRegistry runtime.EquivalentResourceRegistry

	// APIServerID is the ID of this API server
	APIServerID string

	// StorageVersionManager holds the storage versions of the API resources installed by this server.
	StorageVersionManager storageversion.Manager
}

type RecommendedConfig struct {
	Config

	// SharedInformerFactory provides shared informers for Kubernetes resources. This value is set by
	// RecommendedOptions.CoreAPI.ApplyTo called by RecommendedOptions.ApplyTo. It uses an in-cluster client config
	// by default, or the kubeconfig given with kubeconfig command line flag.
	SharedInformerFactory informers.SharedInformerFactory

	// ClientConfig holds the kubernetes client configuration.
	// This value is set by RecommendedOptions.CoreAPI.ApplyTo called by RecommendedOptions.ApplyTo.
	// By default in-cluster client config is used.
	ClientConfig *restclient.Config
}

type SecureServingInfo struct {
	// Listener is the secure server network listener.
	Listener net.Listener

	// Cert is the main server cert which is used if SNI does not match. Cert must be non-nil and is
	// allowed to be in SNICerts.
	Cert dynamiccertificates.CertKeyContentProvider

	// SNICerts are the TLS certificates used for SNI.
	SNICerts []dynamiccertificates.SNICertKeyContentProvider

	// ClientCA is the certificate bundle for all the signers that you'll recognize for incoming client certificates
	ClientCA dynamiccertificates.CAContentProvider

	// MinTLSVersion optionally overrides the minimum TLS version supported.
	// Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants).
	MinTLSVersion uint16

	// CipherSuites optionally overrides the list of allowed cipher suites for the server.
	// Values are from tls package constants (https://golang.org/pkg/crypto/tls/#pkg-constants).
	CipherSuites []uint16

	// HTTP2MaxStreamsPerConnection is the limit that the api server imposes on each client.
	// A value of zero means to use the default provided by golang's HTTP/2 support.
	HTTP2MaxStreamsPerConnection int

	AdvertisePort int

	// DisableHTTP2 indicates that http2 should not be enabled.
	DisableHTTP2 bool
}

type AuthenticationInfo struct {
	// APIAudiences is a list of identifier that the API identifies as. This is
	// used by some authenticators to validate audience bound credentials.
	APIAudiences authenticator.Audiences
	// Authenticator determines which subject is making the request
	Authenticator authenticator.Request
}

type AuthorizationInfo struct {
	// Authorizer determines whether the subject is allowed to make the request based only
	// on the RequestURI
	Authorizer authorizer.Authorizer
}

// NewConfig returns a Config struct with the default values
func NewConfig(codecs serializer.CodecFactory) *Config {
	defaultHealthChecks := []healthz.HealthChecker{healthz.PingHealthz, healthz.LogHealthz}
	var id string
	if feature.DefaultFeatureGate.Enabled(features.APIServerIdentity) {
		id = "kube-apiserver-" + uuid.New().String()
	}
	return &Config{
		Serializer:                  codecs,
		BuildHandlerChainFunc:       DefaultBuildHandlerChain,
		HandlerChainWaitGroup:       new(utilwaitgroup.SafeWaitGroup),
		LegacyAPIGroupPrefixes:      sets.NewString(DefaultLegacyAPIPrefix),
		DisabledPostStartHooks:      sets.NewString(),
		PostStartHooks:              map[string]PostStartHookConfigEntry{},
		HealthzChecks:               append([]healthz.HealthChecker{}, defaultHealthChecks...),
		ReadyzChecks:                append([]healthz.HealthChecker{}, defaultHealthChecks...),
		LivezChecks:                 append([]healthz.HealthChecker{}, defaultHealthChecks...),
		EnableIndex:                 true,
		EnableDiscovery:             true,
		EnableProfiling:             true,
		EnableMetrics:               true,
		MaxRequestsInFlight:         400,
		MaxMutatingRequestsInFlight: 200,
		RequestTimeout:              time.Duration(60) * time.Second,
		MinRequestTimeout:           1800,
		LivezGracePeriod:            time.Duration(0),
		ShutdownDelayDuration:       time.Duration(0),
		// 1.5MB is the default client request size in bytes
		// the etcd server should accept. See
		// https://github.com/etcd-io/etcd/blob/release-3.4/embed/config.go#L56.
		// A request body might be encoded in json, and is converted to
		// proto when persisted in etcd, so we allow 2x as the largest size
		// increase the "copy" operations in a json patch may cause.
		JSONPatchMaxCopyBytes: int64(3 * 1024 * 1024),
		// 1.5MB is the recommended client request size in byte
		// the etcd server should accept. See
		// https://github.com/etcd-io/etcd/blob/release-3.4/embed/config.go#L56.
		// A request body might be encoded in json, and is converted to
		// proto when persisted in etcd, so we allow 2x as the largest request
		// body size to be accepted and decoded in a write request.
		MaxRequestBodyBytes: int64(3 * 1024 * 1024),

		// Default to treating watch as a long-running operation
		// Generic API servers have no inherent long-running subresources
		LongRunningFunc:       genericfilters.BasicLongRunningRequestCheck(sets.NewString("watch"), sets.NewString()),
		RequestWidthEstimator: flowcontrolrequest.DefaultWidthEstimator,
		lifecycleSignals:      newLifecycleSignals(),

		APIServerID:           id,
		StorageVersionManager: storageversion.NewDefaultManager(),
	}
}

// NewRecommendedConfig returns a RecommendedConfig struct with the default values
func NewRecommendedConfig(codecs serializer.CodecFactory) *RecommendedConfig {
	return &RecommendedConfig{
		Config: *NewConfig(codecs),
	}
}

func DefaultOpenAPIConfig(getDefinitions openapicommon.GetOpenAPIDefinitions, defNamer *apiopenapi.DefinitionNamer) *openapicommon.Config {
	return &openapicommon.Config{
		ProtocolList:   []string{"https"},
		IgnorePrefixes: []string{},
		Info: &spec.Info{
			InfoProps: spec.InfoProps{
				Title: "Generic API Server",
			},
		},
		DefaultResponse: &spec.Response{
			ResponseProps: spec.ResponseProps{
				Description: "Default Response.",
			},
		},
		GetOperationIDAndTags: apiopenapi.GetOperationIDAndTags,
		GetDefinitionName:     defNamer.GetDefinitionName,
		GetDefinitions:        getDefinitions,
	}
}

func (c *AuthenticationInfo) ApplyClientCert(clientCA dynamiccertificates.CAContentProvider, servingInfo *SecureServingInfo) error {
	if servingInfo == nil {
		return nil
	}
	if clientCA == nil {
		return nil
	}
	if servingInfo.ClientCA == nil {
		servingInfo.ClientCA = clientCA
		return nil
	}

	servingInfo.ClientCA = dynamiccertificates.NewUnionCAContentProvider(servingInfo.ClientCA, clientCA)
	return nil
}

type completedConfig struct {
	*Config

	//===========================================================================
	// values below here are filled in during completion
	//===========================================================================

	// SharedInformerFactory provides shared informers for resources
	SharedInformerFactory informers.SharedInformerFactory
}

type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

// AddHealthChecks adds a health check to our config to be exposed by the health endpoints
// of our configured apiserver. We should prefer this to adding healthChecks directly to
// the config unless we explicitly want to add a healthcheck only to a specific health endpoint.
func (c *Config) AddHealthChecks(healthChecks ...healthz.HealthChecker) {
	for _, check := range healthChecks {
		c.HealthzChecks = append(c.HealthzChecks, check)
		c.LivezChecks = append(c.LivezChecks, check)
		c.ReadyzChecks = append(c.ReadyzChecks, check)
	}
}

// AddPostStartHook allows you to add a PostStartHook that will later be added to the server itself in a New call.
// Name conflicts will cause an error.
func (c *Config) AddPostStartHook(name string, hook PostStartHookFunc) error {
	if len(name) == 0 {
		return fmt.Errorf("missing name")
	}
	if hook == nil {
		return fmt.Errorf("hook func may not be nil: %q", name)
	}
	if c.DisabledPostStartHooks.Has(name) {
		klog.V(1).Infof("skipping %q because it was explicitly disabled", name)
		return nil
	}

	if postStartHook, exists := c.PostStartHooks[name]; exists {
		// this is programmer error, but it can be hard to debug
		return fmt.Errorf("unable to add %q because it was already registered by: %s", name, postStartHook.originatingStack)
	}
	c.PostStartHooks[name] = PostStartHookConfigEntry{hook: hook, originatingStack: string(debug.Stack())}

	return nil
}

// AddPostStartHookOrDie allows you to add a PostStartHook, but dies on failure.
func (c *Config) AddPostStartHookOrDie(name string, hook PostStartHookFunc) {
	if err := c.AddPostStartHook(name, hook); err != nil {
		klog.Fatalf("Error registering PostStartHook %q: %v", name, err)
	}
}

// Complete fills in any fields not set that are required to have valid data and can be derived
// from other fields. If you're going to `ApplyOptions`, do that first. It's mutating the receiver.
func (c *Config) Complete(informers informers.SharedInformerFactory) CompletedConfig {
	if len(c.ExternalAddress) == 0 && c.PublicAddress != nil {
		c.ExternalAddress = c.PublicAddress.String()
	}

	// if there is no port, and we listen on one securely, use that one
	if _, _, err := net.SplitHostPort(c.ExternalAddress); err != nil {
		if c.SecureServing == nil {
			klog.Fatalf("cannot derive external address port without listening on a secure port.")
		}
		_, port, err := c.SecureServing.HostPort()
		if err != nil {
			klog.Fatalf("cannot derive external address from the secure port: %v", err)
		}
		c.ExternalAddress = net.JoinHostPort(c.ExternalAddress, strconv.Itoa(port))
	}

	if c.OpenAPIConfig != nil {
		if c.OpenAPIConfig.SecurityDefinitions != nil {
			// Setup OpenAPI security: all APIs will have the same authentication for now.
			c.OpenAPIConfig.DefaultSecurity = []map[string][]string{}
			keys := []string{}
			for k := range *c.OpenAPIConfig.SecurityDefinitions {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				c.OpenAPIConfig.DefaultSecurity = append(c.OpenAPIConfig.DefaultSecurity, map[string][]string{k: {}})
			}
			if c.OpenAPIConfig.CommonResponses == nil {
				c.OpenAPIConfig.CommonResponses = map[int]spec.Response{}
			}
			if _, exists := c.OpenAPIConfig.CommonResponses[http.StatusUnauthorized]; !exists {
				c.OpenAPIConfig.CommonResponses[http.StatusUnauthorized] = spec.Response{
					ResponseProps: spec.ResponseProps{
						Description: "Unauthorized",
					},
				}
			}
		}

		// make sure we populate info, and info.version, if not manually set
		if c.OpenAPIConfig.Info == nil {
			c.OpenAPIConfig.Info = &spec.Info{}
		}
		if c.OpenAPIConfig.Info.Version == "" {
			if c.Version != nil {
				c.OpenAPIConfig.Info.Version = strings.Split(c.Version.String(), "-")[0]
			} else {
				c.OpenAPIConfig.Info.Version = "unversioned"
			}
		}
	}
	if c.DiscoveryAddresses == nil {
		c.DiscoveryAddresses = discovery.DefaultAddresses{DefaultAddress: c.ExternalAddress}
	}

	AuthorizeClientBearerToken(c.LoopbackClientConfig, &c.Authentication, &c.Authorization)

	if c.RequestInfoResolver == nil {
		c.RequestInfoResolver = NewRequestInfoResolver(c)
	}

	if c.EquivalentResourceRegistry == nil {
		if c.RESTOptionsGetter == nil {
			c.EquivalentResourceRegistry = runtime.NewEquivalentResourceRegistry()
		} else {
			c.EquivalentResourceRegistry = runtime.NewEquivalentResourceRegistryWithIdentity(func(groupResource schema.GroupResource) string {
				// use the storage prefix as the key if possible
				if opts, err := c.RESTOptionsGetter.GetRESTOptions(groupResource); err == nil {
					return opts.ResourcePrefix
				}
				// otherwise return "" to use the default key (parent GV name)
				return ""
			})
		}
	}

	return CompletedConfig{&completedConfig{c, informers}}
}

// Complete fills in any fields not set that are required to have valid data and can be derived
// from other fields. If you're going to `ApplyOptions`, do that first. It's mutating the receiver.
func (c *RecommendedConfig) Complete() CompletedConfig {
	return c.Config.Complete(c.SharedInformerFactory)
}

// New creates a new server which logically combines the handling chain with the passed server.
// name is used to differentiate for logging. The handler chain in particular can be difficult as it starts delegating.
// delegationTarget may not be nil.
func (c completedConfig) New(name string, delegationTarget DelegationTarget) (*GenericAPIServer, error) {
	if c.Serializer == nil {
		return nil, fmt.Errorf("Genericapiserver.New() called with config.Serializer == nil")
	}
	if c.LoopbackClientConfig == nil {
		return nil, fmt.Errorf("Genericapiserver.New() called with config.LoopbackClientConfig == nil")
	}
	if c.EquivalentResourceRegistry == nil {
		return nil, fmt.Errorf("Genericapiserver.New() called with config.EquivalentResourceRegistry == nil")
	}

	handlerChainBuilder := func(handler http.Handler) http.Handler {
		return c.BuildHandlerChainFunc(handler, c.Config)
	}
	apiServerHandler := NewAPIServerHandler(name, c.Serializer, handlerChainBuilder, delegationTarget.UnprotectedHandler())

	s := &GenericAPIServer{
		discoveryAddresses:         c.DiscoveryAddresses,
		LoopbackClientConfig:       c.LoopbackClientConfig,
		legacyAPIGroupPrefixes:     c.LegacyAPIGroupPrefixes,
		admissionControl:           c.AdmissionControl,
		Serializer:                 c.Serializer,
		AuditBackend:               c.AuditBackend,
		Authorizer:                 c.Authorization.Authorizer,
		delegationTarget:           delegationTarget,
		EquivalentResourceRegistry: c.EquivalentResourceRegistry,
		HandlerChainWaitGroup:      c.HandlerChainWaitGroup,

		minRequestTimeout:     time.Duration(c.MinRequestTimeout) * time.Second,
		ShutdownTimeout:       c.RequestTimeout,
		ShutdownDelayDuration: c.ShutdownDelayDuration,
		SecureServingInfo:     c.SecureServing,
		ExternalAddress:       c.ExternalAddress,

		Handler: apiServerHandler,

		listedPathProvider: apiServerHandler,

		openAPIConfig:           c.OpenAPIConfig,
		skipOpenAPIInstallation: c.SkipOpenAPIInstallation,

		postStartHooks:         map[string]postStartHookEntry{},
		preShutdownHooks:       map[string]preShutdownHookEntry{},
		disabledPostStartHooks: c.DisabledPostStartHooks,

		healthzChecks:    c.HealthzChecks,
		livezChecks:      c.LivezChecks,
		readyzChecks:     c.ReadyzChecks,
		livezGracePeriod: c.LivezGracePeriod,

		DiscoveryGroupManager: discovery.NewRootAPIsHandler(c.DiscoveryAddresses, c.Serializer),

		maxRequestBodyBytes: c.MaxRequestBodyBytes,
		livezClock:          clock.RealClock{},

		lifecycleSignals: c.lifecycleSignals,

		APIServerID:           c.APIServerID,
		StorageVersionManager: c.StorageVersionManager,

		Version: c.Version,
	}

	for {
		if c.JSONPatchMaxCopyBytes <= 0 {
			break
		}
		existing := atomic.LoadInt64(&jsonpatch.AccumulatedCopySizeLimit)
		if existing > 0 && existing < c.JSONPatchMaxCopyBytes {
			break
		}
		if atomic.CompareAndSwapInt64(&jsonpatch.AccumulatedCopySizeLimit, existing, c.JSONPatchMaxCopyBytes) {
			break
		}
	}

	// first add poststarthooks from delegated targets
	for k, v := range delegationTarget.PostStartHooks() {
		s.postStartHooks[k] = v
	}

	for k, v := range delegationTarget.PreShutdownHooks() {
		s.preShutdownHooks[k] = v
	}

	// add poststarthooks that were preconfigured.  Using the add method will give us an error if the same name has already been registered.
	for name, preconfiguredPostStartHook := range c.PostStartHooks {
		if err := s.AddPostStartHook(name, preconfiguredPostStartHook.hook); err != nil {
			return nil, err
		}
	}

	genericApiServerHookName := "generic-apiserver-start-informers"
	if c.SharedInformerFactory != nil {
		if !s.isPostStartHookRegistered(genericApiServerHookName) {
			err := s.AddPostStartHook(genericApiServerHookName, func(context PostStartHookContext) error {
				c.SharedInformerFactory.Start(context.StopCh)
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
		// TODO: Once we get rid of /healthz consider changing this to post-start-hook.
		err := s.AddReadyzChecks(healthz.NewInformerSyncHealthz(c.SharedInformerFactory))
		if err != nil {
			return nil, err
		}
	}

	const priorityAndFairnessConfigConsumerHookName = "priority-and-fairness-config-consumer"
	if s.isPostStartHookRegistered(priorityAndFairnessConfigConsumerHookName) {
	} else if c.FlowControl != nil {
		err := s.AddPostStartHook(priorityAndFairnessConfigConsumerHookName, func(context PostStartHookContext) error {
			go c.FlowControl.MaintainObservations(context.StopCh)
			go c.FlowControl.Run(context.StopCh)
			return nil
		})
		if err != nil {
			return nil, err
		}
		// TODO(yue9944882): plumb pre-shutdown-hook for request-management system?
	} else {
		klog.V(3).Infof("Not requested to run hook %s", priorityAndFairnessConfigConsumerHookName)
	}

	// Add PostStartHooks for maintaining the watermarks for the Priority-and-Fairness and the Max-in-Flight filters.
	if c.FlowControl != nil {
		const priorityAndFairnessFilterHookName = "priority-and-fairness-filter"
		if !s.isPostStartHookRegistered(priorityAndFairnessFilterHookName) {
			err := s.AddPostStartHook(priorityAndFairnessFilterHookName, func(context PostStartHookContext) error {
				genericfilters.StartPriorityAndFairnessWatermarkMaintenance(context.StopCh)
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	} else {
		const maxInFlightFilterHookName = "max-in-flight-filter"
		if !s.isPostStartHookRegistered(maxInFlightFilterHookName) {
			err := s.AddPostStartHook(maxInFlightFilterHookName, func(context PostStartHookContext) error {
				genericfilters.StartMaxInFlightWatermarkMaintenance(context.StopCh)
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}

	for _, delegateCheck := range delegationTarget.HealthzChecks() {
		skip := false
		for _, existingCheck := range c.HealthzChecks {
			if existingCheck.Name() == delegateCheck.Name() {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		s.AddHealthChecks(delegateCheck)
	}

	s.listedPathProvider = routes.ListedPathProviders{s.listedPathProvider, delegationTarget}

	installAPI(s, c.Config)

	// use the UnprotectedHandler from the delegation target to ensure that we don't attempt to double authenticator, authorize,
	// or some other part of the filter chain in delegation cases.
	if delegationTarget.UnprotectedHandler() == nil && c.EnableIndex {
		s.Handler.NonGoRestfulMux.NotFoundHandler(routes.IndexLister{
			StatusCode:   http.StatusNotFound,
			PathProvider: s.listedPathProvider,
		})
	}

	return s, nil
}

func BuildHandlerChainWithStorageVersionPrecondition(apiHandler http.Handler, c *Config) http.Handler {
	// WithStorageVersionPrecondition needs the WithRequestInfo to run first
	handler := genericapifilters.WithStorageVersionPrecondition(apiHandler, c.StorageVersionManager, c.Serializer)
	return DefaultBuildHandlerChain(handler, c)
}

func DefaultBuildHandlerChain(apiHandler http.Handler, c *Config) http.Handler {
	handler := filterlatency.TrackCompleted(apiHandler)
	handler = genericapifilters.WithAuthorization(handler, c.Authorization.Authorizer, c.Serializer)
	handler = filterlatency.TrackStarted(handler, "authorization")

	if c.FlowControl != nil {
		handler = filterlatency.TrackCompleted(handler)
		handler = genericfilters.WithPriorityAndFairness(handler, c.LongRunningFunc, c.FlowControl, c.RequestWidthEstimator)
		handler = filterlatency.TrackStarted(handler, "priorityandfairness")
	} else {
		handler = genericfilters.WithMaxInFlightLimit(handler, c.MaxRequestsInFlight, c.MaxMutatingRequestsInFlight, c.LongRunningFunc)
	}

	handler = filterlatency.TrackCompleted(handler)
	handler = genericapifilters.WithImpersonation(handler, c.Authorization.Authorizer, c.Serializer)
	handler = filterlatency.TrackStarted(handler, "impersonation")

	handler = filterlatency.TrackCompleted(handler)
	handler = genericapifilters.WithAudit(handler, c.AuditBackend, c.AuditPolicyChecker, c.LongRunningFunc)
	handler = filterlatency.TrackStarted(handler, "audit")

	failedHandler := genericapifilters.Unauthorized(c.Serializer)
	failedHandler = genericapifilters.WithFailedAuthenticationAudit(failedHandler, c.AuditBackend, c.AuditPolicyChecker)

	failedHandler = filterlatency.TrackCompleted(failedHandler)
	handler = filterlatency.TrackCompleted(handler)
	handler = genericapifilters.WithAuthentication(handler, c.Authentication.Authenticator, failedHandler, c.Authentication.APIAudiences)
	handler = filterlatency.TrackStarted(handler, "authentication")

	handler = genericfilters.WithCORS(handler, c.CorsAllowedOriginList, nil, nil, nil, "true")

	// WithTimeoutForNonLongRunningRequests will call the rest of the request handling in a go-routine with the
	// context with deadline. The go-routine can keep running, while the timeout logic will return a timeout to the client.
	handler = genericfilters.WithTimeoutForNonLongRunningRequests(handler, c.LongRunningFunc)

	handler = genericapifilters.WithRequestDeadline(handler, c.AuditBackend, c.AuditPolicyChecker,
		c.LongRunningFunc, c.Serializer, c.RequestTimeout)
	handler = genericfilters.WithWaitGroup(handler, c.LongRunningFunc, c.HandlerChainWaitGroup)
	if c.SecureServing != nil && !c.SecureServing.DisableHTTP2 && c.GoawayChance > 0 {
		handler = genericfilters.WithProbabilisticGoaway(handler, c.GoawayChance)
	}
	handler = genericapifilters.WithAuditAnnotations(handler, c.AuditBackend, c.AuditPolicyChecker)
	handler = genericapifilters.WithWarningRecorder(handler)
	handler = genericapifilters.WithCacheControl(handler)
	handler = genericfilters.WithHSTS(handler, c.HSTSDirectives)
	handler = genericfilters.WithHTTPLogging(handler)
	if utilfeature.DefaultFeatureGate.Enabled(genericfeatures.APIServerTracing) {
		handler = genericapifilters.WithTracing(handler, c.TracerProvider)
	}
	handler = genericapifilters.WithRequestInfo(handler, c.RequestInfoResolver)
	handler = genericapifilters.WithRequestReceivedTimestamp(handler)
	handler = genericfilters.WithPanicRecovery(handler, c.RequestInfoResolver)
	handler = genericapifilters.WithAuditID(handler)
	return handler
}

func installAPI(s *GenericAPIServer, c *Config) {
	if c.EnableIndex {
		routes.Index{}.Install(s.listedPathProvider, s.Handler.NonGoRestfulMux)
	}
	if c.EnableProfiling {
		routes.Profiling{}.Install(s.Handler.NonGoRestfulMux)
		if c.EnableContentionProfiling {
			goruntime.SetBlockProfileRate(1)
		}
		// so far, only logging related endpoints are considered valid to add for these debug flags.
		routes.DebugFlags{}.Install(s.Handler.NonGoRestfulMux, "v", routes.StringFlagPutHandler(logs.GlogSetter))
	}
	if c.EnableMetrics {
		if c.EnableProfiling {
			routes.MetricsWithReset{}.Install(s.Handler.NonGoRestfulMux)
		} else {
			routes.DefaultMetrics{}.Install(s.Handler.NonGoRestfulMux)
		}
	}

	routes.Version{Version: c.Version}.Install(s.Handler.GoRestfulContainer)

	if c.EnableDiscovery {
		s.Handler.GoRestfulContainer.Add(s.DiscoveryGroupManager.WebService())
	}
	if c.FlowControl != nil && feature.DefaultFeatureGate.Enabled(features.APIPriorityAndFairness) {
		c.FlowControl.Install(s.Handler.NonGoRestfulMux)
	}
}

func NewRequestInfoResolver(c *Config) *apirequest.RequestInfoFactory {
	apiPrefixes := sets.NewString(strings.Trim(APIGroupPrefix, "/")) // all possible API prefixes
	legacyAPIPrefixes := sets.String{}                               // APIPrefixes that won't have groups (legacy)
	for legacyAPIPrefix := range c.LegacyAPIGroupPrefixes {
		apiPrefixes.Insert(strings.Trim(legacyAPIPrefix, "/"))
		legacyAPIPrefixes.Insert(strings.Trim(legacyAPIPrefix, "/"))
	}

	return &apirequest.RequestInfoFactory{
		APIPrefixes:          apiPrefixes,
		GrouplessAPIPrefixes: legacyAPIPrefixes,
	}
}

func (s *SecureServingInfo) HostPort() (string, int, error) {
	if s == nil || s.Listener == nil {
		return "", 0, fmt.Errorf("no listener found")
	}
	addr := s.Listener.Addr().String()
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get port from listener address %q: %v", addr, err)
	}
	port, err := utilsnet.ParsePort(portStr, true)
	if err != nil {
		return "", 0, fmt.Errorf("invalid non-numeric port %q", portStr)
	}
	if s.AdvertisePort != 0 {
		port = s.AdvertisePort
	}
	return host, port, nil
}

// AuthorizeClientBearerToken wraps the authenticator and authorizer in loopback authentication logic
// if the loopback client config is specified AND it has a bearer token. Note that if either authn or
// authz is nil, this function won't add a token authenticator or authorizer.
func AuthorizeClientBearerToken(loopback *restclient.Config, authn *AuthenticationInfo, authz *AuthorizationInfo) {
	if loopback == nil || len(loopback.BearerToken) == 0 {
		return
	}
	if authn == nil || authz == nil {
		// prevent nil pointer panic
		return
	}
	if authn.Authenticator == nil || authz.Authorizer == nil {
		// authenticator or authorizer might be nil if we want to bypass authz/authn
		// and we also do nothing in this case.
		return
	}

	privilegedLoopbackToken := loopback.BearerToken
	var uid = uuid.New().String()
	tokens := make(map[string]*user.DefaultInfo)
	tokens[privilegedLoopbackToken] = &user.DefaultInfo{
		Name:   user.APIServerUser,
		UID:    uid,
		Groups: []string{user.SystemPrivilegedGroup},
	}

	tokenAuthenticator := authenticatorfactory.NewFromTokens(tokens, authn.APIAudiences)
	authn.Authenticator = authenticatorunion.New(tokenAuthenticator, authn.Authenticator)

	tokenAuthorizer := authorizerfactory.NewPrivilegedGroups(user.SystemPrivilegedGroup)
	authz.Authorizer = authorizerunion.New(tokenAuthorizer, authz.Authorizer)
}
