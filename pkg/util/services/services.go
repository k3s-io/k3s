package services

import (
	"fmt"
	"path/filepath"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
)

const (
	APIServer            = "api-server"
	Admin                = "admin"
	AuthProxy            = "auth-proxy"
	CertificateAuthority = "certificate-authority"
	CloudController      = "cloud-controller"
	ControllerManager    = "controller-manager"
	ETCD                 = "etcd"
	KubeProxy            = "kube-proxy"
	Kubelet              = "kubelet"
	ProgramController    = "-controller"
	ProgramServer        = "-server"
	Scheduler            = "scheduler"
	Supervisor           = "supervisor"
)

var Agent = []string{
	KubeProxy,
	Kubelet,
	version.Program + ProgramController,
}

var Server = []string{
	APIServer,
	Admin,
	AuthProxy,
	CloudController,
	ControllerManager,
	ETCD,
	Scheduler,
	Supervisor,
	version.Program + ProgramServer,
}

var All = append(Server, Agent...)

// CA is intentionally not included in agent, server, or all as it
// requires manual action by the user to rotate these certs.
var CA = []string{
	CertificateAuthority,
}

func FilesForServices(controlConfig config.Control, services []string) (map[string][]string, error) {
	agentDataDir := filepath.Join(controlConfig.DataDir, "..", "agent")
	fileMap := map[string][]string{}
	for _, service := range services {
		switch service {
		case Admin:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientAdminCert,
				controlConfig.Runtime.ClientAdminKey,
			}
		case APIServer:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientKubeAPICert,
				controlConfig.Runtime.ClientKubeAPIKey,
				controlConfig.Runtime.ServingKubeAPICert,
				controlConfig.Runtime.ServingKubeAPIKey,
			}
		case ControllerManager:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientControllerCert,
				controlConfig.Runtime.ClientControllerKey,
			}
		case Scheduler:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientSchedulerCert,
				controlConfig.Runtime.ClientSchedulerKey,
			}
		case ETCD:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientETCDCert,
				controlConfig.Runtime.ClientETCDKey,
				controlConfig.Runtime.ServerETCDCert,
				controlConfig.Runtime.ServerETCDKey,
				controlConfig.Runtime.PeerServerClientETCDCert,
				controlConfig.Runtime.PeerServerClientETCDKey,
			}
		case CloudController:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientCloudControllerCert,
				controlConfig.Runtime.ClientCloudControllerKey,
			}
		case version.Program + ProgramController:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientK3sControllerCert,
				controlConfig.Runtime.ClientK3sControllerKey,
				filepath.Join(agentDataDir, "client-"+version.Program+"-controller.crt"),
				filepath.Join(agentDataDir, "client-"+version.Program+"-controller.key"),
			}
		case Supervisor:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientSupervisorCert,
				controlConfig.Runtime.ClientSupervisorKey,
			}
		case AuthProxy:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientAuthProxyCert,
				controlConfig.Runtime.ClientAuthProxyKey,
			}
		case Kubelet:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientKubeletKey,
				controlConfig.Runtime.ServingKubeletKey,
				filepath.Join(agentDataDir, "client-kubelet.crt"),
				filepath.Join(agentDataDir, "client-kubelet.key"),
				filepath.Join(agentDataDir, "serving-kubelet.crt"),
				filepath.Join(agentDataDir, "serving-kubelet.key"),
			}
		case KubeProxy:
			fileMap[service] = []string{
				controlConfig.Runtime.ClientKubeProxyCert,
				controlConfig.Runtime.ClientKubeProxyKey,
				filepath.Join(agentDataDir, "client-kube-proxy.crt"),
				filepath.Join(agentDataDir, "client-kube-proxy.key"),
			}
		case CertificateAuthority:
			fileMap[service] = []string{
				controlConfig.Runtime.ServerCA,
				controlConfig.Runtime.ServerCAKey,
				controlConfig.Runtime.ClientCA,
				controlConfig.Runtime.ClientCAKey,
				controlConfig.Runtime.RequestHeaderCA,
				controlConfig.Runtime.RequestHeaderCAKey,
				controlConfig.Runtime.ETCDPeerCA,
				controlConfig.Runtime.ETCDPeerCAKey,
				controlConfig.Runtime.ETCDServerCA,
				controlConfig.Runtime.ETCDServerCAKey,
			}
		case version.Program + ProgramServer:
			// not handled here, as the dynamiclistener cert cache is not a standard cert
		default:
			return nil, fmt.Errorf("%s is not a recognized service", service)
		}
	}
	return fileMap, nil
}

func IsValid(svc string) bool {
	for _, service := range All {
		if svc == service {
			return true
		}
	}
	return false
}
