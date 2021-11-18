package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/plugins/plugins/ipam/host-local/backend/allocator"
	"k8s.io/client-go/kubernetes"
)

const (
	podCIDRAnnotation = "kube-router.io/pod-cidr"
)

// GetPodCidrFromCniSpec gets pod CIDR allocated to the node from CNI spec file and returns it
func GetPodCidrFromCniSpec(cniConfFilePath string) (net.IPNet, error) {
	var podCidr = net.IPNet{}
	var err error
	var ipamConfig *allocator.IPAMConfig

	if strings.HasSuffix(cniConfFilePath, ".conflist") {
		var confList *libcni.NetworkConfigList
		confList, err = libcni.ConfListFromFile(cniConfFilePath)
		if err != nil {
			return net.IPNet{}, fmt.Errorf("failed to load CNI config list file: %s", err.Error())
		}
		for _, conf := range confList.Plugins {
			if conf.Network.IPAM.Type != "" {
				ipamConfig, _, err = allocator.LoadIPAMConfig(conf.Bytes, "")
				if err != nil {
					if err.Error() != "no IP ranges specified" {
						return net.IPNet{}, fmt.Errorf("failed to get IPAM details from the CNI conf file: %s", err.Error())
					}
				}
				break
			}
		}
	} else {
		netconfig, err := libcni.ConfFromFile(cniConfFilePath)
		if err != nil {
			return net.IPNet{}, fmt.Errorf("failed to load CNI conf file: %s", err.Error())
		}
		ipamConfig, _, err = allocator.LoadIPAMConfig(netconfig.Bytes, "")
		if err != nil {
			// TODO: Handle this error properly in controllers, if no subnet is specified
			if err.Error() != "no IP ranges specified" {
				return net.IPNet{}, fmt.Errorf("failed to get IPAM details from the CNI conf file: %s", err.Error())
			}
			return net.IPNet{}, nil
		}
	}
	// TODO: Support multiple subnet definitions in CNI conf
	if ipamConfig != nil && len(ipamConfig.Ranges) > 0 {
		for _, rangeset := range ipamConfig.Ranges {
			for _, item := range rangeset {
				if item.Subnet.IP != nil {
					podCidr = net.IPNet(item.Subnet)
					break
				}
			}
		}
	}
	return podCidr, nil
}

// InsertPodCidrInCniSpec inserts the pod CIDR allocated to the node by kubernetes controller manager
// and stored it in the CNI specification
func InsertPodCidrInCniSpec(cniConfFilePath string, cidr string) error {
	file, err := ioutil.ReadFile(cniConfFilePath)
	if err != nil {
		return fmt.Errorf("failed to load CNI conf file: %s", err.Error())
	}
	var config interface{}
	if strings.HasSuffix(cniConfFilePath, ".conflist") {
		err = json.Unmarshal(file, &config)
		if err != nil {
			return fmt.Errorf("failed to parse JSON from CNI conf file: %s", err.Error())
		}
		updatedCidr := false
		configMap := config.(map[string]interface{})
		for key := range configMap {
			if key != "plugins" {
				continue
			}
			// .conflist file has array of plug-in config. Find the one with ipam key
			// and insert the CIDR for the node
			pluginConfigs := configMap["plugins"].([]interface{})
			for _, pluginConfig := range pluginConfigs {
				pluginConfigMap := pluginConfig.(map[string]interface{})
				if val, ok := pluginConfigMap["ipam"]; ok {
					valObj := val.(map[string]interface{})
					valObj["subnet"] = cidr
					updatedCidr = true
					break
				}
			}
		}

		if !updatedCidr {
			return fmt.Errorf("failed to insert subnet cidr into CNI conf file: %s as CNI file is invalid", cniConfFilePath)
		}

	} else {
		err = json.Unmarshal(file, &config)
		if err != nil {
			return fmt.Errorf("failed to parse JSON from CNI conf file: %s", err.Error())
		}
		pluginConfig := config.(map[string]interface{})
		pluginConfig["ipam"].(map[string]interface{})["subnet"] = cidr
	}
	configJSON, _ := json.Marshal(config)
	err = ioutil.WriteFile(cniConfFilePath, configJSON, 0644)
	if err != nil {
		return fmt.Errorf("failed to insert subnet cidr into CNI conf file: %s", err.Error())
	}
	return nil
}

// GetPodCidrFromNodeSpec reads the pod CIDR allocated to the node from API node object and returns it
func GetPodCidrFromNodeSpec(clientset kubernetes.Interface, hostnameOverride string) (string, error) {
	node, err := GetNodeObject(clientset, hostnameOverride)
	if err != nil {
		return "", fmt.Errorf("Failed to get pod CIDR allocated for the node due to: " + err.Error())
	}

	if cidr, ok := node.Annotations[podCIDRAnnotation]; ok {
		_, _, err = net.ParseCIDR(cidr)
		if err != nil {
			return "", fmt.Errorf("error parsing pod CIDR in node annotation: %v", err)
		}

		return cidr, nil
	}

	if node.Spec.PodCIDR == "" {
		return "", fmt.Errorf("node.Spec.PodCIDR not set for node: %v", node.Name)
	}

	return node.Spec.PodCIDR, nil
}
