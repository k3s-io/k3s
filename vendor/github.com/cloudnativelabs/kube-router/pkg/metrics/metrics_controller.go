package metrics

import (
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/cloudnativelabs/kube-router/pkg/healthcheck"
	"github.com/cloudnativelabs/kube-router/pkg/options"
	"github.com/cloudnativelabs/kube-router/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/net/context"
	"k8s.io/klog/v2"
)

const (
	namespace = "kube_router"
)

var (
	// BuildInfo Expose version and other build information
	BuildInfo = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "build_info",
		Help:      "Expose version and other build information",
	}, []string{"goversion", "version"})
	// ServiceTotalConn Total incoming connections made
	ServiceTotalConn = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_total_connections",
		Help:      "Total incoming connections made",
	}, []string{"svc_namespace", "service_name", "service_vip", "protocol", "port"})
	// ServicePacketsIn Total incoming packets
	ServicePacketsIn = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_packets_in",
		Help:      "Total incoming packets",
	}, []string{"svc_namespace", "service_name", "service_vip", "protocol", "port"})
	// ServicePacketsOut Total outgoing packets
	ServicePacketsOut = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_packets_out",
		Help:      "Total outgoing packets",
	}, []string{"svc_namespace", "service_name", "service_vip", "protocol", "port"})
	// ServiceBytesIn Total incoming bytes
	ServiceBytesIn = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_bytes_in",
		Help:      "Total incoming bytes",
	}, []string{"svc_namespace", "service_name", "service_vip", "protocol", "port"})
	// ServiceBytesOut Total outgoing bytes
	ServiceBytesOut = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_bytes_out",
		Help:      "Total outgoing bytes",
	}, []string{"svc_namespace", "service_name", "service_vip", "protocol", "port"})
	// ServicePpsIn Incoming packets per second
	ServicePpsIn = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_pps_in",
		Help:      "Incoming packets per second",
	}, []string{"svc_namespace", "service_name", "service_vip", "protocol", "port"})
	// ServicePpsOut Outgoing packets per second
	ServicePpsOut = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_pps_out",
		Help:      "Outgoing packets per second",
	}, []string{"svc_namespace", "service_name", "service_vip", "protocol", "port"})
	// ServiceCPS Service connections per second
	ServiceCPS = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_cps",
		Help:      "Service connections per second",
	}, []string{"svc_namespace", "service_name", "service_vip", "protocol", "port"})
	// ServiceBpsIn Incoming bytes per second
	ServiceBpsIn = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_bps_in",
		Help:      "Incoming bytes per second",
	}, []string{"svc_namespace", "service_name", "service_vip", "protocol", "port"})
	// ServiceBpsOut Outgoing bytes per second
	ServiceBpsOut = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "service_bps_out",
		Help:      "Outgoing bytes per second",
	}, []string{"svc_namespace", "service_name", "service_vip", "protocol", "port"})
	// ControllerIpvsServices Number of ipvs services in the instance
	ControllerIpvsServices = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "controller_ipvs_services",
		Help:      "Number of ipvs services in the instance",
	})
	// ControllerIptablesSyncTime Time it took for controller to sync iptables
	ControllerIptablesSyncTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "controller_iptables_sync_time",
		Help:      "Time it took for controller to sync iptables",
	})
	// ControllerIptablesSyncTotalTime Time it took for controller to sync iptables
	ControllerIptablesSyncTotalTime = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "controller_iptables_sync_total_time",
		Help:      "Time it took for controller to sync iptables as a counter",
	})
	// ControllerIptablesSyncTotalCount Number of times the controller synced iptables for individual pods
	ControllerIptablesSyncTotalCount = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "controller_iptables_sync_total_count",
		Help:      "Total number of times kube-router synced iptables",
	})
	// ControllerIpvsServicesSyncTime Time it took for controller to sync ipvs services
	ControllerIpvsServicesSyncTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "controller_ipvs_services_sync_time",
		Help:      "Time it took for controller to sync ipvs services",
	})
	// ControllerRoutesSyncTime Time it took for controller to sync ipvs services
	ControllerRoutesSyncTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "controller_routes_sync_time",
		Help:      "Time it took for controller to sync routes",
	})
	// ControllerBPGpeers BGP peers in the runtime configuration
	ControllerBPGpeers = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "controller_bgp_peers",
		Help:      "BGP peers in the runtime configuration",
	})
	// ControllerBGPInternalPeersSyncTime Time it took to sync internal bgp peers
	ControllerBGPInternalPeersSyncTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "controller_bgp_internal_peers_sync_time",
		Help:      "Time it took to sync internal bgp peers",
	})
	// ControllerBGPadvertisementsReceived Time it took to sync internal bgp peers
	ControllerBGPadvertisementsReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "controller_bgp_advertisements_received",
		Help:      "BGP advertisements received",
	})
	// ControllerBGPadvertisementsSent Time it took to sync internal bgp peers
	ControllerBGPadvertisementsSent = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "controller_bgp_advertisements_sent",
		Help:      "BGP advertisements sent",
	})
	// ControllerIpvsMetricsExportTime Time it took to export metrics
	ControllerIpvsMetricsExportTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "controller_ipvs_metrics_export_time",
		Help:      "Time it took to export metrics",
	})
	// ControllerPolicyChainsSyncTime Time it took for controller to sync policys
	ControllerPolicyChainsSyncTime = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "controller_policy_chains_sync_time",
		Help:      "Time it took for controller to sync policy chains",
	})
)

// Controller Holds settings for the metrics controller
type Controller struct {
	MetricsPath string
	MetricsPort uint16
}

// Run prometheus metrics controller
func (mc *Controller) Run(healthChan chan<- *healthcheck.ControllerHeartbeat, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	t := time.NewTicker(3 * time.Second)
	defer wg.Done()
	klog.Info("Starting metrics controller")

	// register metrics for this controller
	BuildInfo.WithLabelValues(runtime.Version(), version.Version).Set(1)
	prometheus.MustRegister(BuildInfo)
	prometheus.MustRegister(ControllerIpvsMetricsExportTime)

	srv := &http.Server{Addr: ":" + strconv.Itoa(int(mc.MetricsPort)), Handler: http.DefaultServeMux}

	// add prometheus handler on metrics path
	http.Handle(mc.MetricsPath, promhttp.Handler())

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			// cannot panic, because this probably is an intentional close
			klog.Errorf("Metrics controller error: %s", err)
		}
	}()
	for {
		healthcheck.SendHeartBeat(healthChan, "MC")
		select {
		case <-stopCh:
			klog.Infof("Shutting down metrics controller")
			if err := srv.Shutdown(context.Background()); err != nil {
				klog.Errorf("could not shutdown: %v", err)
			}
			return
		case <-t.C:
			klog.V(4).Info("Metrics controller tick")
		}
	}
}

// NewMetricsController returns new MetricController object
func NewMetricsController(config *options.KubeRouterConfig) (*Controller, error) {
	mc := Controller{}
	mc.MetricsPath = config.MetricsPath
	mc.MetricsPort = config.MetricsPort
	return &mc, nil
}
