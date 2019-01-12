[![Build Status](https://travis-ci.org/containerd/go-cni.svg?branch=master)](https://travis-ci.org/containerd/go-cni)

# go-cni

A generic CNI library to provide APIs for CNI plugin interactions. The library provides APIs to:

- Load CNI network config from different sources  
- Setup networks for container namespace
- Remove networks from container namespace
- Query status of CNI network plugin initialization

go-cni aims to support plugins that implement [Container Network Interface](https://github.com/containernetworking/cni)

## Usage
```
func main() {
	id := "123456"
	netns := "/proc/9999/ns/net"
	defaultIfName := "eth0"
	// Initialize library
	l = gocni.New(gocni.WithMinNetworkCount(2),
		gocni.WithPluginConfDir("/etc/mycni/net.d"),
		gocni.WithPluginDir([]string{"/opt/mycni/bin", "/opt/cni/bin"}),
		gocni.WithDefaultIfName(defaultIfName))
	
	// Load the cni configuration
	err:= l.Load(gocni.WithLoNetwork,gocni.WithDefaultConf)
        if err != nil{
		log.Errorf("failed to load cni configuration: %v", err)
		return 
	}
	
	// Setup network for namespace.
	labels := map[string]string{
		"K8S_POD_NAMESPACE":          "namespace1",
		"K8S_POD_NAME":               "pod1",
		"K8S_POD_INFRA_CONTAINER_ID": id,
	}
	result, err := l.Setup(id, netns, gocni.WithLabels(labels))
	if err != nil {
		log.Errorf("failed to setup network for namespace %q: %v",id, err)
		return 
	}
	
	// Get IP of the default interface
	IP := result.Interfaces[defaultIfName].IPConfigs[0].IP.String()
	fmt.Printf("IP of the default interface %s:%s", defaultIfName, IP)
}
```
