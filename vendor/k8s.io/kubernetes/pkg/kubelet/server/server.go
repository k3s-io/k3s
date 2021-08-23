/*
Copyright 2014 The Kubernetes Authors.

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
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"reflect"
	goruntime "runtime"
	"strconv"
	"strings"
	"time"

	"github.com/emicklei/go-restful"
	cadvisormetrics "github.com/google/cadvisor/container"
	cadvisorapi "github.com/google/cadvisor/info/v1"
	cadvisorv2 "github.com/google/cadvisor/info/v2"
	"github.com/google/cadvisor/metrics"
	"google.golang.org/grpc"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/kubelet/metrics/collectors"
	"k8s.io/utils/clock"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/proxy"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/apiserver/pkg/server/httplog"
	"k8s.io/apiserver/pkg/server/routes"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/apiserver/pkg/util/flushwriter"
	"k8s.io/component-base/configz"
	"k8s.io/component-base/logs"
	compbasemetrics "k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
	podresourcesapi "k8s.io/kubelet/pkg/apis/podresources/v1"
	podresourcesapiv1alpha1 "k8s.io/kubelet/pkg/apis/podresources/v1alpha1"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/core/v1/validation"
	"k8s.io/kubernetes/pkg/features"
	kubeletconfiginternal "k8s.io/kubernetes/pkg/kubelet/apis/config"
	"k8s.io/kubernetes/pkg/kubelet/apis/podresources"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/cri/streaming"
	"k8s.io/kubernetes/pkg/kubelet/cri/streaming/portforward"
	remotecommandserver "k8s.io/kubernetes/pkg/kubelet/cri/streaming/remotecommand"
	"k8s.io/kubernetes/pkg/kubelet/prober"
	servermetrics "k8s.io/kubernetes/pkg/kubelet/server/metrics"
	"k8s.io/kubernetes/pkg/kubelet/server/stats"
	kubelettypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util"
)

const (
	metricsPath         = "/metrics"
	cadvisorMetricsPath = "/metrics/cadvisor"
	resourceMetricsPath = "/metrics/resource"
	proberMetricsPath   = "/metrics/probes"
	statsPath           = "/stats/"
	logsPath            = "/logs/"
	pprofBasePath       = "/debug/pprof/"
	debugFlagPath       = "/debug/flags/v"
)

// Server is a http.Handler which exposes kubelet functionality over HTTP.
type Server struct {
	auth                 AuthInterface
	host                 HostInterface
	restfulCont          containerInterface
	metricsBuckets       sets.String
	metricsMethodBuckets sets.String
	resourceAnalyzer     stats.ResourceAnalyzer
}

// TLSOptions holds the TLS options.
type TLSOptions struct {
	Config   *tls.Config
	CertFile string
	KeyFile  string
}

// containerInterface defines the restful.Container functions used on the root container
type containerInterface interface {
	Add(service *restful.WebService) *restful.Container
	Handle(path string, handler http.Handler)
	Filter(filter restful.FilterFunction)
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	RegisteredWebServices() []*restful.WebService

	// RegisteredHandlePaths returns the paths of handlers registered directly with the container (non-web-services)
	// Used to test filters are being applied on non-web-service handlers
	RegisteredHandlePaths() []string
}

// filteringContainer delegates all Handle(...) calls to Container.HandleWithFilter(...),
// so we can ensure restful.FilterFunctions are used for all handlers
type filteringContainer struct {
	*restful.Container
	registeredHandlePaths []string
}

func (a *filteringContainer) Handle(path string, handler http.Handler) {
	a.HandleWithFilter(path, handler)
	a.registeredHandlePaths = append(a.registeredHandlePaths, path)
}
func (a *filteringContainer) RegisteredHandlePaths() []string {
	return a.registeredHandlePaths
}

// ListenAndServeKubeletServer initializes a server to respond to HTTP network requests on the Kubelet.
func ListenAndServeKubeletServer(
	host HostInterface,
	resourceAnalyzer stats.ResourceAnalyzer,
	kubeCfg *kubeletconfiginternal.KubeletConfiguration,
	tlsOptions *TLSOptions,
	auth AuthInterface) {

	address := net.ParseIP(kubeCfg.Address)
	port := uint(kubeCfg.Port)
	klog.InfoS("Starting to listen", "address", address, "port", port)
	handler := NewServer(host, resourceAnalyzer, auth, kubeCfg)
	s := &http.Server{
		Addr:           net.JoinHostPort(address.String(), strconv.FormatUint(uint64(port), 10)),
		Handler:        &handler,
		IdleTimeout:    90 * time.Second, // matches http.DefaultTransport keep-alive timeout
		ReadTimeout:    4 * 60 * time.Minute,
		WriteTimeout:   4 * 60 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}
	if tlsOptions != nil {
		s.TLSConfig = tlsOptions.Config
		// Passing empty strings as the cert and key files means no
		// cert/keys are specified and GetCertificate in the TLSConfig
		// should be called instead.
		if err := s.ListenAndServeTLS(tlsOptions.CertFile, tlsOptions.KeyFile); err != nil {
			klog.ErrorS(err, "Failed to listen and serve")
			os.Exit(1)
		}
	} else if err := s.ListenAndServe(); err != nil {
		klog.ErrorS(err, "Failed to listen and serve")
		os.Exit(1)
	}
}

// ListenAndServeKubeletReadOnlyServer initializes a server to respond to HTTP network requests on the Kubelet.
func ListenAndServeKubeletReadOnlyServer(host HostInterface, resourceAnalyzer stats.ResourceAnalyzer, address net.IP, port uint) {
	klog.InfoS("Starting to listen read-only", "address", address, "port", port)
	s := NewServer(host, resourceAnalyzer, nil, nil)

	server := &http.Server{
		Addr:           net.JoinHostPort(address.String(), strconv.FormatUint(uint64(port), 10)),
		Handler:        &s,
		IdleTimeout:    90 * time.Second, // matches http.DefaultTransport keep-alive timeout
		ReadTimeout:    4 * 60 * time.Minute,
		WriteTimeout:   4 * 60 * time.Minute,
		MaxHeaderBytes: 1 << 20,
	}

	if err := server.ListenAndServe(); err != nil {
		klog.ErrorS(err, "Failed to listen and serve")
		os.Exit(1)
	}
}

// ListenAndServePodResources initializes a gRPC server to serve the PodResources service
func ListenAndServePodResources(socket string, podsProvider podresources.PodsProvider, devicesProvider podresources.DevicesProvider, cpusProvider podresources.CPUsProvider, memoryProvider podresources.MemoryProvider) {
	server := grpc.NewServer()
	podresourcesapiv1alpha1.RegisterPodResourcesListerServer(server, podresources.NewV1alpha1PodResourcesServer(podsProvider, devicesProvider))
	podresourcesapi.RegisterPodResourcesListerServer(server, podresources.NewV1PodResourcesServer(podsProvider, devicesProvider, cpusProvider, memoryProvider))
	l, err := util.CreateListener(socket)
	if err != nil {
		klog.ErrorS(err, "Failed to create listener for podResources endpoint")
		os.Exit(1)
	}

	if err := server.Serve(l); err != nil {
		klog.ErrorS(err, "Failed to serve")
		os.Exit(1)
	}
}

// AuthInterface contains all methods required by the auth filters
type AuthInterface interface {
	authenticator.Request
	authorizer.RequestAttributesGetter
	authorizer.Authorizer
}

// HostInterface contains all the kubelet methods required by the server.
// For testability.
type HostInterface interface {
	stats.Provider
	GetVersionInfo() (*cadvisorapi.VersionInfo, error)
	GetCachedMachineInfo() (*cadvisorapi.MachineInfo, error)
	GetRunningPods() ([]*v1.Pod, error)
	RunInContainer(name string, uid types.UID, container string, cmd []string) ([]byte, error)
	GetKubeletContainerLogs(ctx context.Context, podFullName, containerName string, logOptions *v1.PodLogOptions, stdout, stderr io.Writer) error
	ServeLogs(w http.ResponseWriter, req *http.Request)
	ResyncInterval() time.Duration
	GetHostname() string
	LatestLoopEntryTime() time.Time
	GetExec(podFullName string, podUID types.UID, containerName string, cmd []string, streamOpts remotecommandserver.Options) (*url.URL, error)
	GetAttach(podFullName string, podUID types.UID, containerName string, streamOpts remotecommandserver.Options) (*url.URL, error)
	GetPortForward(podName, podNamespace string, podUID types.UID, portForwardOpts portforward.V4Options) (*url.URL, error)
}

// NewServer initializes and configures a kubelet.Server object to handle HTTP requests.
func NewServer(
	host HostInterface,
	resourceAnalyzer stats.ResourceAnalyzer,
	auth AuthInterface,
	kubeCfg *kubeletconfiginternal.KubeletConfiguration) Server {
	server := Server{
		host:                 host,
		resourceAnalyzer:     resourceAnalyzer,
		auth:                 auth,
		restfulCont:          &filteringContainer{Container: restful.NewContainer()},
		metricsBuckets:       sets.NewString(),
		metricsMethodBuckets: sets.NewString("OPTIONS", "GET", "HEAD", "POST", "PUT", "DELETE", "TRACE", "CONNECT"),
	}
	if auth != nil {
		server.InstallAuthFilter()
	}
	server.InstallDefaultHandlers()
	if kubeCfg != nil && kubeCfg.EnableDebuggingHandlers {
		server.InstallDebuggingHandlers()
		// To maintain backward compatibility serve logs and pprof only when enableDebuggingHandlers is also enabled
		// see https://github.com/kubernetes/kubernetes/pull/87273
		server.InstallSystemLogHandler(kubeCfg.EnableSystemLogHandler)
		server.InstallProfilingHandler(kubeCfg.EnableProfilingHandler, kubeCfg.EnableContentionProfiling)
		server.InstallDebugFlagsHandler(kubeCfg.EnableDebugFlagsHandler)
	} else {
		server.InstallDebuggingDisabledHandlers()
	}
	return server
}

// InstallAuthFilter installs authentication filters with the restful Container.
func (s *Server) InstallAuthFilter() {
	s.restfulCont.Filter(func(req *restful.Request, resp *restful.Response, chain *restful.FilterChain) {
		// Authenticate
		info, ok, err := s.auth.AuthenticateRequest(req.Request)
		if err != nil {
			klog.ErrorS(err, "Unable to authenticate the request due to an error")
			resp.WriteErrorString(http.StatusUnauthorized, "Unauthorized")
			return
		}
		if !ok {
			resp.WriteErrorString(http.StatusUnauthorized, "Unauthorized")
			return
		}

		// Get authorization attributes
		attrs := s.auth.GetRequestAttributes(info.User, req.Request)

		// Authorize
		decision, _, err := s.auth.Authorize(req.Request.Context(), attrs)
		if err != nil {
			klog.ErrorS(err, "Authorization error", "user", attrs.GetUser().GetName(), "verb", attrs.GetVerb(), "resource", attrs.GetResource(), "subresource", attrs.GetSubresource())
			msg := fmt.Sprintf("Authorization error (user=%s, verb=%s, resource=%s, subresource=%s)", attrs.GetUser().GetName(), attrs.GetVerb(), attrs.GetResource(), attrs.GetSubresource())
			resp.WriteErrorString(http.StatusInternalServerError, msg)
			return
		}
		if decision != authorizer.DecisionAllow {
			klog.V(2).InfoS("Forbidden", "user", attrs.GetUser().GetName(), "verb", attrs.GetVerb(), "resource", attrs.GetResource(), "subresource", attrs.GetSubresource())
			msg := fmt.Sprintf("Forbidden (user=%s, verb=%s, resource=%s, subresource=%s)", attrs.GetUser().GetName(), attrs.GetVerb(), attrs.GetResource(), attrs.GetSubresource())
			resp.WriteErrorString(http.StatusForbidden, msg)
			return
		}

		// Continue
		chain.ProcessFilter(req, resp)
	})
}

// addMetricsBucketMatcher adds a regexp matcher and the relevant bucket to use when
// it matches. Please be aware this is not thread safe and should not be used dynamically
func (s *Server) addMetricsBucketMatcher(bucket string) {
	s.metricsBuckets.Insert(bucket)
}

// getMetricBucket find the appropriate metrics reporting bucket for the given path
func (s *Server) getMetricBucket(path string) string {
	root := getURLRootPath(path)
	if s.metricsBuckets.Has(root) {
		return root
	}
	return "other"
}

// getMetricMethodBucket checks for unknown or invalid HTTP verbs
func (s *Server) getMetricMethodBucket(method string) string {
	if s.metricsMethodBuckets.Has(method) {
		return method
	}
	return "other"
}

// InstallDefaultHandlers registers the default set of supported HTTP request
// patterns with the restful Container.
func (s *Server) InstallDefaultHandlers() {
	s.addMetricsBucketMatcher("healthz")
	healthz.InstallHandler(s.restfulCont,
		healthz.PingHealthz,
		healthz.LogHealthz,
		healthz.NamedCheck("syncloop", s.syncLoopHealthCheck),
	)

	s.addMetricsBucketMatcher("pods")
	ws := new(restful.WebService)
	ws.
		Path("/pods").
		Produces(restful.MIME_JSON)
	ws.Route(ws.GET("").
		To(s.getPods).
		Operation("getPods"))
	s.restfulCont.Add(ws)

	s.addMetricsBucketMatcher("stats")
	s.restfulCont.Add(stats.CreateHandlers(statsPath, s.host, s.resourceAnalyzer))

	s.addMetricsBucketMatcher("metrics")
	s.addMetricsBucketMatcher("metrics/cadvisor")
	s.addMetricsBucketMatcher("metrics/probes")
	s.addMetricsBucketMatcher("metrics/resource")
	//lint:ignore SA1019 https://github.com/kubernetes/enhancements/issues/1206
	s.restfulCont.Handle(metricsPath, legacyregistry.Handler())

	// cAdvisor metrics are exposed under the secured handler as well
	r := compbasemetrics.NewKubeRegistry()

	includedMetrics := cadvisormetrics.MetricSet{
		cadvisormetrics.CpuUsageMetrics:     struct{}{},
		cadvisormetrics.MemoryUsageMetrics:  struct{}{},
		cadvisormetrics.CpuLoadMetrics:      struct{}{},
		cadvisormetrics.DiskIOMetrics:       struct{}{},
		cadvisormetrics.DiskUsageMetrics:    struct{}{},
		cadvisormetrics.NetworkUsageMetrics: struct{}{},
		cadvisormetrics.AppMetrics:          struct{}{},
		cadvisormetrics.ProcessMetrics:      struct{}{},
	}

	// Only add the Accelerator metrics if the feature is inactive
	// Note: Accelerator metrics will be removed in the future, hence the feature gate.
	if !utilfeature.DefaultFeatureGate.Enabled(features.DisableAcceleratorUsageMetrics) {
		includedMetrics.Add(cadvisormetrics.MetricKind(cadvisormetrics.AcceleratorUsageMetrics))
	}

	cadvisorOpts := cadvisorv2.RequestOptions{
		IdType:    cadvisorv2.TypeName,
		Count:     1,
		Recursive: true,
	}
	r.RawMustRegister(metrics.NewPrometheusCollector(prometheusHostAdapter{s.host}, containerPrometheusLabelsFunc(s.host), includedMetrics, clock.RealClock{}, cadvisorOpts))
	r.RawMustRegister(metrics.NewPrometheusMachineCollector(prometheusHostAdapter{s.host}, includedMetrics))
	s.restfulCont.Handle(cadvisorMetricsPath,
		compbasemetrics.HandlerFor(r, compbasemetrics.HandlerOpts{ErrorHandling: compbasemetrics.ContinueOnError}),
	)

	s.addMetricsBucketMatcher("metrics/resource")
	resourceRegistry := compbasemetrics.NewKubeRegistry()
	resourceRegistry.CustomMustRegister(collectors.NewResourceMetricsCollector(s.resourceAnalyzer))
	s.restfulCont.Handle(resourceMetricsPath,
		compbasemetrics.HandlerFor(resourceRegistry, compbasemetrics.HandlerOpts{ErrorHandling: compbasemetrics.ContinueOnError}),
	)

	// prober metrics are exposed under a different endpoint

	s.addMetricsBucketMatcher("metrics/probes")
	p := compbasemetrics.NewKubeRegistry()
	_ = compbasemetrics.RegisterProcessStartTime(p.Register)
	p.MustRegister(prober.ProberResults)
	s.restfulCont.Handle(proberMetricsPath,
		compbasemetrics.HandlerFor(p, compbasemetrics.HandlerOpts{ErrorHandling: compbasemetrics.ContinueOnError}),
	)
}

// InstallDebuggingHandlers registers the HTTP request patterns that serve logs or run commands/containers
func (s *Server) InstallDebuggingHandlers() {
	klog.InfoS("Adding debug handlers to kubelet server")

	s.addMetricsBucketMatcher("run")
	ws := new(restful.WebService)
	ws.
		Path("/run")
	ws.Route(ws.POST("/{podNamespace}/{podID}/{containerName}").
		To(s.getRun).
		Operation("getRun"))
	ws.Route(ws.POST("/{podNamespace}/{podID}/{uid}/{containerName}").
		To(s.getRun).
		Operation("getRun"))
	s.restfulCont.Add(ws)

	s.addMetricsBucketMatcher("exec")
	ws = new(restful.WebService)
	ws.
		Path("/exec")
	ws.Route(ws.GET("/{podNamespace}/{podID}/{containerName}").
		To(s.getExec).
		Operation("getExec"))
	ws.Route(ws.POST("/{podNamespace}/{podID}/{containerName}").
		To(s.getExec).
		Operation("getExec"))
	ws.Route(ws.GET("/{podNamespace}/{podID}/{uid}/{containerName}").
		To(s.getExec).
		Operation("getExec"))
	ws.Route(ws.POST("/{podNamespace}/{podID}/{uid}/{containerName}").
		To(s.getExec).
		Operation("getExec"))
	s.restfulCont.Add(ws)

	s.addMetricsBucketMatcher("attach")
	ws = new(restful.WebService)
	ws.
		Path("/attach")
	ws.Route(ws.GET("/{podNamespace}/{podID}/{containerName}").
		To(s.getAttach).
		Operation("getAttach"))
	ws.Route(ws.POST("/{podNamespace}/{podID}/{containerName}").
		To(s.getAttach).
		Operation("getAttach"))
	ws.Route(ws.GET("/{podNamespace}/{podID}/{uid}/{containerName}").
		To(s.getAttach).
		Operation("getAttach"))
	ws.Route(ws.POST("/{podNamespace}/{podID}/{uid}/{containerName}").
		To(s.getAttach).
		Operation("getAttach"))
	s.restfulCont.Add(ws)

	s.addMetricsBucketMatcher("portForward")
	ws = new(restful.WebService)
	ws.
		Path("/portForward")
	ws.Route(ws.GET("/{podNamespace}/{podID}").
		To(s.getPortForward).
		Operation("getPortForward"))
	ws.Route(ws.POST("/{podNamespace}/{podID}").
		To(s.getPortForward).
		Operation("getPortForward"))
	ws.Route(ws.GET("/{podNamespace}/{podID}/{uid}").
		To(s.getPortForward).
		Operation("getPortForward"))
	ws.Route(ws.POST("/{podNamespace}/{podID}/{uid}").
		To(s.getPortForward).
		Operation("getPortForward"))
	s.restfulCont.Add(ws)

	s.addMetricsBucketMatcher("containerLogs")
	ws = new(restful.WebService)
	ws.
		Path("/containerLogs")
	ws.Route(ws.GET("/{podNamespace}/{podID}/{containerName}").
		To(s.getContainerLogs).
		Operation("getContainerLogs"))
	s.restfulCont.Add(ws)

	s.addMetricsBucketMatcher("configz")
	configz.InstallHandler(s.restfulCont)

	// The /runningpods endpoint is used for testing only.
	s.addMetricsBucketMatcher("runningpods")
	ws = new(restful.WebService)
	ws.
		Path("/runningpods/").
		Produces(restful.MIME_JSON)
	ws.Route(ws.GET("").
		To(s.getRunningPods).
		Operation("getRunningPods"))
	s.restfulCont.Add(ws)
}

// InstallDebuggingDisabledHandlers registers the HTTP request patterns that provide better error message
func (s *Server) InstallDebuggingDisabledHandlers() {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Debug endpoints are disabled.", http.StatusMethodNotAllowed)
	})

	s.addMetricsBucketMatcher("run")
	s.addMetricsBucketMatcher("exec")
	s.addMetricsBucketMatcher("attach")
	s.addMetricsBucketMatcher("portForward")
	s.addMetricsBucketMatcher("containerLogs")
	s.addMetricsBucketMatcher("runningpods")
	s.addMetricsBucketMatcher("pprof")
	s.addMetricsBucketMatcher("logs")
	paths := []string{
		"/run/", "/exec/", "/attach/", "/portForward/", "/containerLogs/",
		"/runningpods/", pprofBasePath, logsPath}
	for _, p := range paths {
		s.restfulCont.Handle(p, h)
	}
}

// InstallSystemLogHandler registers the HTTP request patterns for logs endpoint.
func (s *Server) InstallSystemLogHandler(enableSystemLogHandler bool) {
	s.addMetricsBucketMatcher("logs")
	if enableSystemLogHandler {
		ws := new(restful.WebService)
		ws.Path(logsPath)
		ws.Route(ws.GET("").
			To(s.getLogs).
			Operation("getLogs"))
		ws.Route(ws.GET("/{logpath:*}").
			To(s.getLogs).
			Operation("getLogs").
			Param(ws.PathParameter("logpath", "path to the log").DataType("string")))
		s.restfulCont.Add(ws)
	} else {
		s.restfulCont.Handle(logsPath, getHandlerForDisabledEndpoint("logs endpoint is disabled."))
	}
}

func getHandlerForDisabledEndpoint(errorMessage string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, errorMessage, http.StatusMethodNotAllowed)
	})
}

// InstallDebugFlagsHandler registers the HTTP request patterns for /debug/flags/v endpoint.
func (s *Server) InstallDebugFlagsHandler(enableDebugFlagsHandler bool) {
	if enableDebugFlagsHandler {
		// Setup flags handlers.
		// so far, only logging related endpoints are considered valid to add for these debug flags.
		s.restfulCont.Handle(debugFlagPath, routes.StringFlagPutHandler(logs.GlogSetter))
	} else {
		s.restfulCont.Handle(debugFlagPath, getHandlerForDisabledEndpoint("flags endpoint is disabled."))
		return
	}
}

// InstallProfilingHandler registers the HTTP request patterns for /debug/pprof endpoint.
func (s *Server) InstallProfilingHandler(enableProfilingLogHandler bool, enableContentionProfiling bool) {
	s.addMetricsBucketMatcher("debug")
	if !enableProfilingLogHandler {
		s.restfulCont.Handle(pprofBasePath, getHandlerForDisabledEndpoint("profiling endpoint is disabled."))
		return
	}

	handlePprofEndpoint := func(req *restful.Request, resp *restful.Response) {
		name := strings.TrimPrefix(req.Request.URL.Path, pprofBasePath)
		switch name {
		case "profile":
			pprof.Profile(resp, req.Request)
		case "symbol":
			pprof.Symbol(resp, req.Request)
		case "cmdline":
			pprof.Cmdline(resp, req.Request)
		case "trace":
			pprof.Trace(resp, req.Request)
		default:
			pprof.Index(resp, req.Request)
		}
	}

	// Setup pprof handlers.
	ws := new(restful.WebService).Path(pprofBasePath)
	ws.Route(ws.GET("/{subpath:*}").To(handlePprofEndpoint)).Doc("pprof endpoint")
	s.restfulCont.Add(ws)

	if enableContentionProfiling {
		goruntime.SetBlockProfileRate(1)
	}
}

// Checks if kubelet's sync loop  that updates containers is working.
func (s *Server) syncLoopHealthCheck(req *http.Request) error {
	duration := s.host.ResyncInterval() * 2
	minDuration := time.Minute * 5
	if duration < minDuration {
		duration = minDuration
	}
	enterLoopTime := s.host.LatestLoopEntryTime()
	if !enterLoopTime.IsZero() && time.Now().After(enterLoopTime.Add(duration)) {
		return fmt.Errorf("sync Loop took longer than expected")
	}
	return nil
}

// getContainerLogs handles containerLogs request against the Kubelet
func (s *Server) getContainerLogs(request *restful.Request, response *restful.Response) {
	podNamespace := request.PathParameter("podNamespace")
	podID := request.PathParameter("podID")
	containerName := request.PathParameter("containerName")
	ctx := request.Request.Context()

	if len(podID) == 0 {
		// TODO: Why return JSON when the rest return plaintext errors?
		// TODO: Why return plaintext errors?
		response.WriteError(http.StatusBadRequest, fmt.Errorf(`{"message": "Missing podID."}`))
		return
	}
	if len(containerName) == 0 {
		// TODO: Why return JSON when the rest return plaintext errors?
		response.WriteError(http.StatusBadRequest, fmt.Errorf(`{"message": "Missing container name."}`))
		return
	}
	if len(podNamespace) == 0 {
		// TODO: Why return JSON when the rest return plaintext errors?
		response.WriteError(http.StatusBadRequest, fmt.Errorf(`{"message": "Missing podNamespace."}`))
		return
	}

	query := request.Request.URL.Query()
	// backwards compatibility for the "tail" query parameter
	if tail := request.QueryParameter("tail"); len(tail) > 0 {
		query["tailLines"] = []string{tail}
		// "all" is the same as omitting tail
		if tail == "all" {
			delete(query, "tailLines")
		}
	}
	// container logs on the kubelet are locked to the v1 API version of PodLogOptions
	logOptions := &v1.PodLogOptions{}
	if err := legacyscheme.ParameterCodec.DecodeParameters(query, v1.SchemeGroupVersion, logOptions); err != nil {
		response.WriteError(http.StatusBadRequest, fmt.Errorf(`{"message": "Unable to decode query."}`))
		return
	}
	logOptions.TypeMeta = metav1.TypeMeta{}
	if errs := validation.ValidatePodLogOptions(logOptions); len(errs) > 0 {
		response.WriteError(http.StatusUnprocessableEntity, fmt.Errorf(`{"message": "Invalid request."}`))
		return
	}

	pod, ok := s.host.GetPodByName(podNamespace, podID)
	if !ok {
		response.WriteError(http.StatusNotFound, fmt.Errorf("pod %q does not exist", podID))
		return
	}
	// Check if containerName is valid.
	if kubecontainer.GetContainerSpec(pod, containerName) == nil {
		response.WriteError(http.StatusNotFound, fmt.Errorf("container %q not found in pod %q", containerName, podID))
		return
	}

	if _, ok := response.ResponseWriter.(http.Flusher); !ok {
		response.WriteError(http.StatusInternalServerError, fmt.Errorf("unable to convert %v into http.Flusher, cannot show logs", reflect.TypeOf(response)))
		return
	}
	fw := flushwriter.Wrap(response.ResponseWriter)
	response.Header().Set("Transfer-Encoding", "chunked")
	if err := s.host.GetKubeletContainerLogs(ctx, kubecontainer.GetPodFullName(pod), containerName, logOptions, fw, fw); err != nil {
		response.WriteError(http.StatusBadRequest, err)
		return
	}
}

// encodePods creates an v1.PodList object from pods and returns the encoded
// PodList.
func encodePods(pods []*v1.Pod) (data []byte, err error) {
	podList := new(v1.PodList)
	for _, pod := range pods {
		podList.Items = append(podList.Items, *pod)
	}
	// TODO: this needs to be parameterized to the kubelet, not hardcoded. Depends on Kubelet
	//   as API server refactor.
	// TODO: Locked to v1, needs to be made generic
	codec := legacyscheme.Codecs.LegacyCodec(schema.GroupVersion{Group: v1.GroupName, Version: "v1"})
	return runtime.Encode(codec, podList)
}

// getPods returns a list of pods bound to the Kubelet and their spec.
func (s *Server) getPods(request *restful.Request, response *restful.Response) {
	pods := s.host.GetPods()
	data, err := encodePods(pods)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	writeJSONResponse(response, data)
}

// getRunningPods returns a list of pods running on Kubelet. The list is
// provided by the container runtime, and is different from the list returned
// by getPods, which is a set of desired pods to run.
func (s *Server) getRunningPods(request *restful.Request, response *restful.Response) {
	pods, err := s.host.GetRunningPods()
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	data, err := encodePods(pods)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	writeJSONResponse(response, data)
}

// getLogs handles logs requests against the Kubelet.
func (s *Server) getLogs(request *restful.Request, response *restful.Response) {
	s.host.ServeLogs(response, request.Request)
}

type execRequestParams struct {
	podNamespace  string
	podName       string
	podUID        types.UID
	containerName string
	cmd           []string
}

func getExecRequestParams(req *restful.Request) execRequestParams {
	return execRequestParams{
		podNamespace:  req.PathParameter("podNamespace"),
		podName:       req.PathParameter("podID"),
		podUID:        types.UID(req.PathParameter("uid")),
		containerName: req.PathParameter("containerName"),
		cmd:           req.Request.URL.Query()[api.ExecCommandParam],
	}
}

type portForwardRequestParams struct {
	podNamespace string
	podName      string
	podUID       types.UID
}

func getPortForwardRequestParams(req *restful.Request) portForwardRequestParams {
	return portForwardRequestParams{
		podNamespace: req.PathParameter("podNamespace"),
		podName:      req.PathParameter("podID"),
		podUID:       types.UID(req.PathParameter("uid")),
	}
}

type responder struct{}

func (r *responder) Error(w http.ResponseWriter, req *http.Request, err error) {
	klog.ErrorS(err, "Error while proxying request")
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// proxyStream proxies stream to url.
func proxyStream(w http.ResponseWriter, r *http.Request, url *url.URL) {
	// TODO(random-liu): Set MaxBytesPerSec to throttle the stream.
	handler := proxy.NewUpgradeAwareHandler(url, nil /*transport*/, false /*wrapTransport*/, true /*upgradeRequired*/, &responder{})
	handler.ServeHTTP(w, r)
}

// getAttach handles requests to attach to a container.
func (s *Server) getAttach(request *restful.Request, response *restful.Response) {
	params := getExecRequestParams(request)
	streamOpts, err := remotecommandserver.NewOptions(request.Request)
	if err != nil {
		utilruntime.HandleError(err)
		response.WriteError(http.StatusBadRequest, err)
		return
	}
	pod, ok := s.host.GetPodByName(params.podNamespace, params.podName)
	if !ok {
		response.WriteError(http.StatusNotFound, fmt.Errorf("pod does not exist"))
		return
	}

	podFullName := kubecontainer.GetPodFullName(pod)
	url, err := s.host.GetAttach(podFullName, params.podUID, params.containerName, *streamOpts)
	if err != nil {
		streaming.WriteError(err, response.ResponseWriter)
		return
	}

	proxyStream(response.ResponseWriter, request.Request, url)
}

// getExec handles requests to run a command inside a container.
func (s *Server) getExec(request *restful.Request, response *restful.Response) {
	params := getExecRequestParams(request)
	streamOpts, err := remotecommandserver.NewOptions(request.Request)
	if err != nil {
		utilruntime.HandleError(err)
		response.WriteError(http.StatusBadRequest, err)
		return
	}
	pod, ok := s.host.GetPodByName(params.podNamespace, params.podName)
	if !ok {
		response.WriteError(http.StatusNotFound, fmt.Errorf("pod does not exist"))
		return
	}

	podFullName := kubecontainer.GetPodFullName(pod)
	url, err := s.host.GetExec(podFullName, params.podUID, params.containerName, params.cmd, *streamOpts)
	if err != nil {
		streaming.WriteError(err, response.ResponseWriter)
		return
	}
	proxyStream(response.ResponseWriter, request.Request, url)
}

// getRun handles requests to run a command inside a container.
func (s *Server) getRun(request *restful.Request, response *restful.Response) {
	params := getExecRequestParams(request)
	pod, ok := s.host.GetPodByName(params.podNamespace, params.podName)
	if !ok {
		response.WriteError(http.StatusNotFound, fmt.Errorf("pod does not exist"))
		return
	}

	// For legacy reasons, run uses different query param than exec.
	params.cmd = strings.Split(request.QueryParameter("cmd"), " ")
	data, err := s.host.RunInContainer(kubecontainer.GetPodFullName(pod), params.podUID, params.containerName, params.cmd)
	if err != nil {
		response.WriteError(http.StatusInternalServerError, err)
		return
	}
	writeJSONResponse(response, data)
}

// Derived from go-restful writeJSON.
func writeJSONResponse(response *restful.Response, data []byte) {
	if data == nil {
		response.WriteHeader(http.StatusOK)
		// do not write a nil representation
		return
	}
	response.Header().Set(restful.HEADER_ContentType, restful.MIME_JSON)
	response.WriteHeader(http.StatusOK)
	if _, err := response.Write(data); err != nil {
		klog.ErrorS(err, "Error writing response")
	}
}

// getPortForward handles a new restful port forward request. It determines the
// pod name and uid and then calls ServePortForward.
func (s *Server) getPortForward(request *restful.Request, response *restful.Response) {
	params := getPortForwardRequestParams(request)

	portForwardOptions, err := portforward.NewV4Options(request.Request)
	if err != nil {
		utilruntime.HandleError(err)
		response.WriteError(http.StatusBadRequest, err)
		return
	}
	pod, ok := s.host.GetPodByName(params.podNamespace, params.podName)
	if !ok {
		response.WriteError(http.StatusNotFound, fmt.Errorf("pod does not exist"))
		return
	}
	if len(params.podUID) > 0 && pod.UID != params.podUID {
		response.WriteError(http.StatusNotFound, fmt.Errorf("pod not found"))
		return
	}

	url, err := s.host.GetPortForward(pod.Name, pod.Namespace, pod.UID, *portForwardOptions)
	if err != nil {
		streaming.WriteError(err, response.ResponseWriter)
		return
	}
	proxyStream(response.ResponseWriter, request.Request, url)
}

// getURLRootPath trims a URL path.
// For paths in the format of "/metrics/xxx", "metrics/xxx" is returned;
// For all other paths, the first part of the path is returned.
func getURLRootPath(path string) string {
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 3)
	if len(parts) == 0 {
		return path
	}

	if parts[0] == "metrics" && len(parts) > 1 {
		return fmt.Sprintf("%s/%s", parts[0], parts[1])

	}
	return parts[0]
}

var longRunningRequestPathMap = map[string]bool{
	"exec":        true,
	"attach":      true,
	"portforward": true,
	"debug":       true,
}

// isLongRunningRequest determines whether the request is long-running or not.
func isLongRunningRequest(path string) bool {
	_, ok := longRunningRequestPathMap[path]
	return ok
}

var statusesNoTracePred = httplog.StatusIsNot(
	http.StatusOK,
	http.StatusFound,
	http.StatusMovedPermanently,
	http.StatusTemporaryRedirect,
	http.StatusBadRequest,
	http.StatusNotFound,
	http.StatusSwitchingProtocols,
)

// ServeHTTP responds to HTTP requests on the Kubelet.
func (s *Server) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler := httplog.WithLogging(s.restfulCont, statusesNoTracePred)

	// monitor http requests
	var serverType string
	if s.auth == nil {
		serverType = "readonly"
	} else {
		serverType = "readwrite"
	}

	method, path := s.getMetricMethodBucket(req.Method), s.getMetricBucket(req.URL.Path)

	longRunning := strconv.FormatBool(isLongRunningRequest(path))

	servermetrics.HTTPRequests.WithLabelValues(method, path, serverType, longRunning).Inc()

	servermetrics.HTTPInflightRequests.WithLabelValues(method, path, serverType, longRunning).Inc()
	defer servermetrics.HTTPInflightRequests.WithLabelValues(method, path, serverType, longRunning).Dec()

	startTime := time.Now()
	defer servermetrics.HTTPRequestsDuration.WithLabelValues(method, path, serverType, longRunning).Observe(servermetrics.SinceInSeconds(startTime))

	handler.ServeHTTP(w, req)
}

// prometheusHostAdapter adapts the HostInterface to the interface expected by the
// cAdvisor prometheus collector.
type prometheusHostAdapter struct {
	host HostInterface
}

func (a prometheusHostAdapter) GetRequestedContainersInfo(containerName string, options cadvisorv2.RequestOptions) (map[string]*cadvisorapi.ContainerInfo, error) {
	return a.host.GetRequestedContainersInfo(containerName, options)
}
func (a prometheusHostAdapter) GetVersionInfo() (*cadvisorapi.VersionInfo, error) {
	return a.host.GetVersionInfo()
}
func (a prometheusHostAdapter) GetMachineInfo() (*cadvisorapi.MachineInfo, error) {
	return a.host.GetCachedMachineInfo()
}

func containerPrometheusLabelsFunc(s stats.Provider) metrics.ContainerLabelsFunc {
	// containerPrometheusLabels maps cAdvisor labels to prometheus labels.
	return func(c *cadvisorapi.ContainerInfo) map[string]string {
		// Prometheus requires that all metrics in the same family have the same labels,
		// so we arrange to supply blank strings for missing labels
		var name, image, podName, namespace, containerName string
		if len(c.Aliases) > 0 {
			name = c.Aliases[0]
		}
		image = c.Spec.Image
		if v, ok := c.Spec.Labels[kubelettypes.KubernetesPodNameLabel]; ok {
			podName = v
		}
		if v, ok := c.Spec.Labels[kubelettypes.KubernetesPodNamespaceLabel]; ok {
			namespace = v
		}
		if v, ok := c.Spec.Labels[kubelettypes.KubernetesContainerNameLabel]; ok {
			containerName = v
		}
		// Associate pod cgroup with pod so we have an accurate accounting of sandbox
		if podName == "" && namespace == "" {
			if pod, found := s.GetPodByCgroupfs(c.Name); found {
				podName = pod.Name
				namespace = pod.Namespace
			}
		}
		set := map[string]string{
			metrics.LabelID:    c.Name,
			metrics.LabelName:  name,
			metrics.LabelImage: image,
			"pod":              podName,
			"namespace":        namespace,
			"container":        containerName,
		}
		return set
	}
}
