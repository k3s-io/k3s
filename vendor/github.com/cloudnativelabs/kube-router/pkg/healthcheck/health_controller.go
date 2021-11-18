package healthcheck

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/cloudnativelabs/kube-router/pkg/options"
	"golang.org/x/net/context"
	"k8s.io/klog/v2"
)

//ControllerHeartbeat is the structure to hold the heartbeats sent by controllers
type ControllerHeartbeat struct {
	Component     string
	LastHeartBeat time.Time
}

//HealthController reports the health of the controller loops as a http endpoint
type HealthController struct {
	HealthPort  uint16
	HTTPEnabled bool
	Status      HealthStats
	Config      *options.KubeRouterConfig
}

//HealthStats is holds the latest heartbeats
type HealthStats struct {
	sync.Mutex
	Healthy                           bool
	MetricsControllerAlive            time.Time
	NetworkPolicyControllerAlive      time.Time
	NetworkPolicyControllerAliveTTL   time.Duration
	NetworkRoutingControllerAlive     time.Time
	NetworkRoutingControllerAliveTTL  time.Duration
	NetworkServicesControllerAlive    time.Time
	NetworkServicesControllerAliveTTL time.Duration
}

//SendHeartBeat sends a heartbeat on the passed channel
func SendHeartBeat(channel chan<- *ControllerHeartbeat, controller string) {
	heartbeat := ControllerHeartbeat{
		Component:     controller,
		LastHeartBeat: time.Now(),
	}
	channel <- &heartbeat
}

//Handler writes HTTP responses to the health path
func (hc *HealthController) Handler(w http.ResponseWriter, _ *http.Request) {
	if hc.Status.Healthy {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK\n"))
		if err != nil {
			klog.Errorf("Failed to write body: %s", err)
		}
	} else {
		w.WriteHeader(http.StatusInternalServerError)
		/*
			statusText := fmt.Sprintf("Service controller last alive %s\n ago"+
				"Routing controller last alive: %s\n ago"+
				"Policy controller last alive: %s\n ago"+
				"Metrics controller last alive: %s\n ago",
				time.Since(hc.Status.NetworkServicesControllerAlive),
				time.Since(hc.Status.NetworkRoutingControllerAlive),
				time.Since(hc.Status.NetworkPolicyControllerAlive),
				time.Since(hc.Status.MetricsControllerAlive))
			w.Write([]byte(statusText))
		*/
		_, err := w.Write([]byte("Unhealthy"))
		if err != nil {
			klog.Errorf("Failed to write body: %s", err)
		}
	}
}

//HandleHeartbeat handles received heartbeats on the health channel
func (hc *HealthController) HandleHeartbeat(beat *ControllerHeartbeat) {
	klog.V(3).Infof("Received heartbeat from %s", beat.Component)

	hc.Status.Lock()
	defer hc.Status.Unlock()

	switch {
	// The first heartbeat will set the initial gracetime the controller has to report in, A static time is added as well when checking to allow for load variation in sync time
	case beat.Component == "NSC":
		if hc.Status.NetworkServicesControllerAliveTTL == 0 {
			hc.Status.NetworkServicesControllerAliveTTL = time.Since(hc.Status.NetworkServicesControllerAlive)
		}
		hc.Status.NetworkServicesControllerAlive = beat.LastHeartBeat

	case beat.Component == "NRC":
		if hc.Status.NetworkRoutingControllerAliveTTL == 0 {
			hc.Status.NetworkRoutingControllerAliveTTL = time.Since(hc.Status.NetworkRoutingControllerAlive)
		}
		hc.Status.NetworkRoutingControllerAlive = beat.LastHeartBeat

	case beat.Component == "NPC":
		if hc.Status.NetworkPolicyControllerAliveTTL == 0 {
			hc.Status.NetworkPolicyControllerAliveTTL = time.Since(hc.Status.NetworkPolicyControllerAlive)
		}
		hc.Status.NetworkPolicyControllerAlive = beat.LastHeartBeat

	case beat.Component == "MC":
		hc.Status.MetricsControllerAlive = beat.LastHeartBeat
	}
}

// CheckHealth evaluates the time since last heartbeat to decide if the controller is running or not
func (hc *HealthController) CheckHealth() bool {
	health := true
	graceTime := time.Duration(1500) * time.Millisecond

	if hc.Config.RunFirewall {
		if time.Since(hc.Status.NetworkPolicyControllerAlive) > hc.Config.IPTablesSyncPeriod+hc.Status.NetworkPolicyControllerAliveTTL+graceTime {
			klog.Error("Network Policy Controller heartbeat missed")
			health = false
		}
	}

	if hc.Config.RunRouter {
		if time.Since(hc.Status.NetworkRoutingControllerAlive) > hc.Config.RoutesSyncPeriod+hc.Status.NetworkRoutingControllerAliveTTL+graceTime {
			klog.Error("Network Routing Controller heartbeat missed")
			health = false
		}
	}

	if hc.Config.RunServiceProxy {
		if time.Since(hc.Status.NetworkServicesControllerAlive) > hc.Config.IpvsSyncPeriod+hc.Status.NetworkServicesControllerAliveTTL+graceTime {
			klog.Error("NetworkService Controller heartbeat missed")
			health = false
		}
	}

	if hc.Config.MetricsEnabled {
		if time.Since(hc.Status.MetricsControllerAlive) > 5*time.Second {
			klog.Error("Metrics Controller heartbeat missed")
			health = false
		}
	}

	return health
}

//RunServer starts the HealthController's server
func (hc *HealthController) RunServer(stopCh <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	srv := &http.Server{Addr: ":" + strconv.Itoa(int(hc.HealthPort)), Handler: http.DefaultServeMux}
	http.HandleFunc("/healthz", hc.Handler)
	if hc.Config.HealthPort > 0 {
		hc.HTTPEnabled = true
		go func() {
			if err := srv.ListenAndServe(); err != nil {
				// cannot panic, because this probably is an intentional close
				klog.Errorf("Health controller error: %s", err)
			}
		}()
	} else if hc.Config.MetricsPort > 65535 {
		klog.Errorf("Metrics port must be over 0 and under 65535, given port: %d", hc.Config.MetricsPort)
	} else {
		hc.HTTPEnabled = false
	}

	// block until we receive a shut down signal
	<-stopCh
	klog.Infof("Shutting down health controller")
	if hc.HTTPEnabled {
		if err := srv.Shutdown(context.Background()); err != nil {
			klog.Errorf("could not shutdown: %v", err)
		}
	}
}

//RunCheck starts the HealthController's check
func (hc *HealthController) RunCheck(healthChan <-chan *ControllerHeartbeat, stopCh <-chan struct{}, wg *sync.WaitGroup) {
	t := time.NewTicker(5000 * time.Millisecond)
	defer wg.Done()
	for {
		select {
		case <-stopCh:
			klog.Infof("Shutting down HealthController RunCheck")
			return
		case heartbeat := <-healthChan:
			hc.HandleHeartbeat(heartbeat)
		case <-t.C:
			klog.V(4).Info("Health controller tick")
		}
		hc.Status.Healthy = hc.CheckHealth()
	}
}

func (hc *HealthController) SetAlive() {

	now := time.Now()

	hc.Status.MetricsControllerAlive = now
	hc.Status.NetworkPolicyControllerAlive = now
	hc.Status.NetworkRoutingControllerAlive = now
	hc.Status.NetworkServicesControllerAlive = now
}

//NewHealthController creates a new health controller and returns a reference to it
func NewHealthController(config *options.KubeRouterConfig) (*HealthController, error) {
	hc := HealthController{
		Config:     config,
		HealthPort: config.HealthPort,
		Status: HealthStats{
			Healthy: true,
		},
	}
	return &hc, nil
}
