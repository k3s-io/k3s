// +build !providerless

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

package azure

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	azcache "k8s.io/legacy-cloud-providers/azure/cache"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
)

const (
	vmPowerStatePrefix       = "PowerState/"
	vmPowerStateStopped      = "stopped"
	vmPowerStateDeallocated  = "deallocated"
	vmPowerStateDeallocating = "deallocating"

	// nodeNameEnvironmentName is the environment variable name for getting node name.
	// It is only used for out-of-tree cloud provider.
	nodeNameEnvironmentName = "NODE_NAME"
)

var (
	errNodeNotInitialized = fmt.Errorf("providerID is empty, the node is not initialized yet")
)

func (az *Cloud) addressGetter(nodeName types.NodeName) ([]v1.NodeAddress, error) {
	ip, publicIP, err := az.getIPForMachine(nodeName)
	if err != nil {
		klog.V(2).Infof("NodeAddresses(%s) abort backoff: %v", nodeName, err)
		return nil, err
	}

	addresses := []v1.NodeAddress{
		{Type: v1.NodeInternalIP, Address: ip},
		{Type: v1.NodeHostName, Address: string(nodeName)},
	}
	if len(publicIP) > 0 {
		addresses = append(addresses, v1.NodeAddress{
			Type:    v1.NodeExternalIP,
			Address: publicIP,
		})
	}
	return addresses, nil
}

// NodeAddresses returns the addresses of the specified instance.
func (az *Cloud) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	// Returns nil for unmanaged nodes because azure cloud provider couldn't fetch information for them.
	unmanaged, err := az.IsNodeUnmanaged(string(name))
	if err != nil {
		return nil, err
	}
	if unmanaged {
		klog.V(4).Infof("NodeAddresses: omitting unmanaged node %q", name)
		return nil, nil
	}

	if az.UseInstanceMetadata {
		metadata, err := az.metadata.GetMetadata(azcache.CacheReadTypeDefault)
		if err != nil {
			return nil, err
		}

		if metadata.Compute == nil || metadata.Network == nil {
			return nil, fmt.Errorf("failure of getting instance metadata")
		}

		isLocalInstance, err := az.isCurrentInstance(name, metadata.Compute.Name)
		if err != nil {
			return nil, err
		}

		// Not local instance, get addresses from Azure ARM API.
		if !isLocalInstance {
			if az.VMSet != nil {
				return az.addressGetter(name)
			}

			// vmSet == nil indicates credentials are not provided.
			return nil, fmt.Errorf("no credentials provided for Azure cloud provider")
		}

		return az.getLocalInstanceNodeAddresses(metadata.Network.Interface, string(name))
	}

	return az.addressGetter(name)
}

func (az *Cloud) getLocalInstanceNodeAddresses(netInterfaces []NetworkInterface, nodeName string) ([]v1.NodeAddress, error) {
	if len(netInterfaces) == 0 {
		return nil, fmt.Errorf("no interface is found for the instance")
	}

	// Use ip address got from instance metadata.
	netInterface := netInterfaces[0]
	addresses := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: nodeName},
	}
	if len(netInterface.IPV4.IPAddress) > 0 && len(netInterface.IPV4.IPAddress[0].PrivateIP) > 0 {
		address := netInterface.IPV4.IPAddress[0]
		addresses = append(addresses, v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: address.PrivateIP,
		})
		if len(address.PublicIP) > 0 {
			addresses = append(addresses, v1.NodeAddress{
				Type:    v1.NodeExternalIP,
				Address: address.PublicIP,
			})
		}
	}
	if len(netInterface.IPV6.IPAddress) > 0 && len(netInterface.IPV6.IPAddress[0].PrivateIP) > 0 {
		address := netInterface.IPV6.IPAddress[0]
		addresses = append(addresses, v1.NodeAddress{
			Type:    v1.NodeInternalIP,
			Address: address.PrivateIP,
		})
		if len(address.PublicIP) > 0 {
			addresses = append(addresses, v1.NodeAddress{
				Type:    v1.NodeExternalIP,
				Address: address.PublicIP,
			})
		}
	}

	if len(addresses) == 1 {
		// No IP addresses is got from instance metadata service, clean up cache and report errors.
		az.metadata.imsCache.Delete(metadataCacheKey)
		return nil, fmt.Errorf("get empty IP addresses from instance metadata service")
	}
	return addresses, nil
}

// NodeAddressesByProviderID returns the node addresses of an instances with the specified unique providerID
// This method will not be called from the node that is requesting this ID. i.e. metadata service
// and other local methods cannot be used here
func (az *Cloud) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	if providerID == "" {
		return nil, errNodeNotInitialized
	}

	// Returns nil for unmanaged nodes because azure cloud provider couldn't fetch information for them.
	if az.IsNodeUnmanagedByProviderID(providerID) {
		klog.V(4).Infof("NodeAddressesByProviderID: omitting unmanaged node %q", providerID)
		return nil, nil
	}

	name, err := az.VMSet.GetNodeNameByProviderID(providerID)
	if err != nil {
		return nil, err
	}

	return az.NodeAddresses(ctx, name)
}

// InstanceExistsByProviderID returns true if the instance with the given provider id still exists and is running.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
func (az *Cloud) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	if providerID == "" {
		return false, errNodeNotInitialized
	}

	// Returns true for unmanaged nodes because azure cloud provider always assumes them exists.
	if az.IsNodeUnmanagedByProviderID(providerID) {
		klog.V(4).Infof("InstanceExistsByProviderID: assuming unmanaged node %q exists", providerID)
		return true, nil
	}

	name, err := az.VMSet.GetNodeNameByProviderID(providerID)
	if err != nil {
		if err == cloudprovider.InstanceNotFound {
			return false, nil
		}
		return false, err
	}

	_, err = az.InstanceID(ctx, name)
	if err != nil {
		if err == cloudprovider.InstanceNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// InstanceShutdownByProviderID returns true if the instance is in safe state to detach volumes
func (az *Cloud) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	if providerID == "" {
		return false, nil
	}

	nodeName, err := az.VMSet.GetNodeNameByProviderID(providerID)
	if err != nil {
		// Returns false, so the controller manager will continue to check InstanceExistsByProviderID().
		if err == cloudprovider.InstanceNotFound {
			return false, nil
		}

		return false, err
	}

	powerStatus, err := az.VMSet.GetPowerStatusByNodeName(string(nodeName))
	if err != nil {
		// Returns false, so the controller manager will continue to check InstanceExistsByProviderID().
		if err == cloudprovider.InstanceNotFound {
			return false, nil
		}

		return false, err
	}
	klog.V(3).Infof("InstanceShutdownByProviderID gets power status %q for node %q", powerStatus, nodeName)

	provisioningState, err := az.VMSet.GetProvisioningStateByNodeName(string(nodeName))
	if err != nil {
		// Returns false, so the controller manager will continue to check InstanceExistsByProviderID().
		if errors.Is(err, cloudprovider.InstanceNotFound) {
			return false, nil
		}

		return false, err
	}
	klog.V(3).Infof("InstanceShutdownByProviderID gets provisioning state %q for node %q", provisioningState, nodeName)

	status := strings.ToLower(powerStatus)
	provisioningSucceeded := strings.EqualFold(strings.ToLower(provisioningState), strings.ToLower(string(compute.ProvisioningStateSucceeded)))
	return provisioningSucceeded && (status == vmPowerStateStopped || status == vmPowerStateDeallocated || status == vmPowerStateDeallocating), nil
}

func (az *Cloud) isCurrentInstance(name types.NodeName, metadataVMName string) (bool, error) {
	var err error
	nodeName := mapNodeNameToVMName(name)

	// VMSS vmName is not same with hostname, use hostname instead.
	if az.VMType == vmTypeVMSS {
		metadataVMName, err = os.Hostname()
		if err != nil {
			return false, err
		}

		// Use name from env variable "NODE_NAME" if it is set.
		nodeNameEnv := os.Getenv(nodeNameEnvironmentName)
		if nodeNameEnv != "" {
			metadataVMName = nodeNameEnv
		}
	}

	metadataVMName = strings.ToLower(metadataVMName)
	return metadataVMName == nodeName, nil
}

// InstanceID returns the cloud provider ID of the specified instance.
// Note that if the instance does not exist or is no longer running, we must return ("", cloudprovider.InstanceNotFound)
func (az *Cloud) InstanceID(ctx context.Context, name types.NodeName) (string, error) {
	nodeName := mapNodeNameToVMName(name)
	unmanaged, err := az.IsNodeUnmanaged(nodeName)
	if err != nil {
		return "", err
	}
	if unmanaged {
		// InstanceID is same with nodeName for unmanaged nodes.
		klog.V(4).Infof("InstanceID: getting ID %q for unmanaged node %q", name, name)
		return nodeName, nil
	}

	if az.UseInstanceMetadata {
		metadata, err := az.metadata.GetMetadata(azcache.CacheReadTypeDefault)
		if err != nil {
			return "", err
		}

		if metadata.Compute == nil {
			return "", fmt.Errorf("failure of getting instance metadata")
		}

		isLocalInstance, err := az.isCurrentInstance(name, metadata.Compute.Name)
		if err != nil {
			return "", err
		}

		// Not local instance, get instanceID from Azure ARM API.
		if !isLocalInstance {
			if az.VMSet != nil {
				return az.VMSet.GetInstanceIDByNodeName(nodeName)
			}

			// vmSet == nil indicates credentials are not provided.
			return "", fmt.Errorf("no credentials provided for Azure cloud provider")
		}
		return az.getLocalInstanceProviderID(metadata, nodeName)
	}

	return az.VMSet.GetInstanceIDByNodeName(nodeName)
}

func (az *Cloud) getLocalInstanceProviderID(metadata *InstanceMetadata, nodeName string) (string, error) {
	// Get resource group name and subscription ID.
	resourceGroup := strings.ToLower(metadata.Compute.ResourceGroup)
	subscriptionID := strings.ToLower(metadata.Compute.SubscriptionID)

	// Compose instanceID based on nodeName for standard instance.
	if metadata.Compute.VMScaleSetName == "" {
		return az.getStandardMachineID(subscriptionID, resourceGroup, nodeName), nil
	}

	// Get scale set name and instanceID from vmName for vmss.
	ssName, instanceID, err := extractVmssVMName(metadata.Compute.Name)
	if err != nil {
		if err == ErrorNotVmssInstance {
			// Compose machineID for standard Node.
			return az.getStandardMachineID(subscriptionID, resourceGroup, nodeName), nil
		}
		return "", err
	}
	// Compose instanceID based on ssName and instanceID for vmss instance.
	return az.getVmssMachineID(subscriptionID, resourceGroup, ssName, instanceID), nil
}

// InstanceTypeByProviderID returns the cloudprovider instance type of the node with the specified unique providerID
// This method will not be called from the node that is requesting this ID. i.e. metadata service
// and other local methods cannot be used here
func (az *Cloud) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	if providerID == "" {
		return "", errNodeNotInitialized
	}

	// Returns "" for unmanaged nodes because azure cloud provider couldn't fetch information for them.
	if az.IsNodeUnmanagedByProviderID(providerID) {
		klog.V(4).Infof("InstanceTypeByProviderID: omitting unmanaged node %q", providerID)
		return "", nil
	}

	name, err := az.VMSet.GetNodeNameByProviderID(providerID)
	if err != nil {
		return "", err
	}

	return az.InstanceType(ctx, name)
}

// InstanceType returns the type of the specified instance.
// Note that if the instance does not exist or is no longer running, we must return ("", cloudprovider.InstanceNotFound)
// (Implementer Note): This is used by kubelet. Kubelet will label the node. Real log from kubelet:
//       Adding node label from cloud provider: beta.kubernetes.io/instance-type=[value]
func (az *Cloud) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	// Returns "" for unmanaged nodes because azure cloud provider couldn't fetch information for them.
	unmanaged, err := az.IsNodeUnmanaged(string(name))
	if err != nil {
		return "", err
	}
	if unmanaged {
		klog.V(4).Infof("InstanceType: omitting unmanaged node %q", name)
		return "", nil
	}

	if az.UseInstanceMetadata {
		metadata, err := az.metadata.GetMetadata(azcache.CacheReadTypeDefault)
		if err != nil {
			return "", err
		}

		if metadata.Compute == nil {
			return "", fmt.Errorf("failure of getting instance metadata")
		}

		isLocalInstance, err := az.isCurrentInstance(name, metadata.Compute.Name)
		if err != nil {
			return "", err
		}
		if !isLocalInstance {
			if az.VMSet != nil {
				return az.VMSet.GetInstanceTypeByNodeName(string(name))
			}

			// vmSet == nil indicates credentials are not provided.
			return "", fmt.Errorf("no credentials provided for Azure cloud provider")
		}

		if metadata.Compute.VMSize != "" {
			return metadata.Compute.VMSize, nil
		}
	}

	return az.VMSet.GetInstanceTypeByNodeName(string(name))
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (az *Cloud) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on.
// On Azure this is the hostname, so we just return the hostname.
func (az *Cloud) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return types.NodeName(hostname), nil
}

// mapNodeNameToVMName maps a k8s NodeName to an Azure VM Name
// This is a simple string cast.
func mapNodeNameToVMName(nodeName types.NodeName) string {
	return string(nodeName)
}
