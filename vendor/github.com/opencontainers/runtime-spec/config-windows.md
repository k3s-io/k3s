# <a name="windowsSpecificContainerConfiguration" />Windows-specific Container Configuration

This document describes the schema for the [Windows-specific section](config.md#platform-specific-configuration) of the [container configuration](config.md).
The Windows container specification uses APIs provided by the Windows Host Compute Service (HCS) to fulfill the spec.

## <a name="configWindowsLayerFolders" />LayerFolders

**`layerFolders`** (array of strings, REQUIRED) specifies a list of layer folders the container image relies on. The list is ordered from topmost layer to base layer.
  `layerFolders` MUST contain at least one entry.

### Example

```json
    "windows": {
        "layerFolders": [
            "C:\\Layers\\layer1",
            "C:\\Layers\\layer2"
        ]
    }
```

## <a name="configWindowsDevices" />Devices

**`devices`** (array of objects, OPTIONAL) lists devices that MUST be available in the container.

Each entry has the following structure:

* **`id`** *(string, REQUIRED)* - specifies the device which the runtime MUST make available in the container.
* **`idType`** *(string, REQUIRED)* - tells the runtime how to interpret `id`. Today, Windows only supports a value of `class`, which identifies `id` as a [device interface class GUID][interfaceGUID].

[interfaceGUID]: https://docs.microsoft.com/en-us/windows-hardware/drivers/install/overview-of-device-interface-classes

### Example

```json
    "windows": {
        "devices": [
            {
                "id": "24E552D7-6523-47F7-A647-D3465BF1F5CA",
                "idType": "class"
            },
            {
                "id": "5175d334-c371-4806-b3ba-71fd53c9258d",
                "idType": "class"
            }
        ]
    }
```

## <a name="configWindowsResources" />Resources

You can configure a container's resource limits via the OPTIONAL `resources` field of the Windows configuration.

### <a name="configWindowsMemory" />Memory

`memory` is an OPTIONAL configuration for the container's memory usage.

The following parameters can be specified:

* **`limit`** *(uint64, OPTIONAL)* - sets limit of memory usage in bytes.

#### Example

```json
    "windows": {
        "resources": {
            "memory": {
                "limit": 2097152
            }
        }
    }
```

### <a name="configWindowsCpu" />CPU

`cpu` is an OPTIONAL configuration for the container's CPU usage.

The following parameters can be specified:

* **`count`** *(uint64, OPTIONAL)* - specifies the number of CPUs available to the container.
* **`shares`** *(uint16, OPTIONAL)* - specifies the relative weight to other containers with CPU shares.
* **`maximum`** *(uint16, OPTIONAL)* - specifies the portion of processor cycles that this container can use as a percentage times 100.

#### Example

```json
    "windows": {
        "resources": {
            "cpu": {
                "maximum": 5000
            }
        }
    }
```

### <a name="configWindowsStorage" />Storage

`storage` is an OPTIONAL configuration for the container's storage usage.

The following parameters can be specified:

* **`iops`** *(uint64, OPTIONAL)* - specifies the maximum IO operations per second for the system drive of the container.
* **`bps`** *(uint64, OPTIONAL)* - specifies the maximum bytes per second for the system drive of the container.
* **`sandboxSize`** *(uint64, OPTIONAL)* - specifies the minimum size of the system drive in bytes.

#### Example

```json
    "windows": {
        "resources": {
            "storage": {
                "iops": 50
            }
        }
    }
```

## <a name="configWindowsNetwork" />Network

You can configure a container's networking options via the OPTIONAL `network` field of the Windows configuration.

The following parameters can be specified:

* **`endpointList`** *(array of strings, OPTIONAL)* - list of HNS (Host Network Service) endpoints that the container should connect to.
* **`allowUnqualifiedDNSQuery`** *(bool, OPTIONAL)* - specifies if unqualified DNS name resolution is allowed.
* **`DNSSearchList`** *(array of strings, OPTIONAL)* - comma separated list of DNS suffixes to use for name resolution.
* **`networkSharedContainerName`** *(string, OPTIONAL)* - name (ID) of the container that we will share with the network stack.
* **`networkNamespace`** *(string, OPTIONAL)* - name (ID) of the network namespace that will be used for the container. If a network namespace is specified no other parameter must be specified.

### Example

```json
    "windows": {
        "network": {
            "endpointList": [
                "7a010682-17e0-4455-a838-02e5d9655fe6"
            ],
            "allowUnqualifiedDNSQuery": true,
            "DNSSearchList": [
                "a.com",
                "b.com"
            ],
            "networkSharedContainerName": "containerName",
            "networkNamespace": "168f3daf-efc6-4377-b20a-2c86764ba892"
        }
   }
```

## <a name="configWindowsCredentialSpec" />Credential Spec

You can configure a container's group Managed Service Account (gMSA) via the OPTIONAL `credentialSpec` field of the Windows configuration.
The `credentialSpec` is a JSON object whose properties are implementation-defined.
For more information about gMSAs, see [Active Directory Service Accounts for Windows Containers][gMSAOverview].
For more information about tooling to generate a gMSA, see [Deployment Overview][gMSATooling].


[gMSAOverview]: https://aka.ms/windowscontainers/manage-serviceaccounts
[gMSATooling]: https://aka.ms/windowscontainers/credentialspec-tools

## <a name="configWindowsServicing" />Servicing

When a container terminates, the Host Compute Service indicates if a Windows update servicing operation is pending.
You can indicate that a container should be started in a mode to apply pending servicing operations via the OPTIONAL `servicing` field of the Windows configuration.

### Example

```json
    "windows": {
        "servicing": true
    }
```

## <a name="configWindowsIgnoreFlushesDuringBoot" />IgnoreFlushesDuringBoot

You can indicate that a container should be started in an a mode where disk flushes are not performed during container boot via the OPTIONAL `ignoreFlushesDuringBoot` field of the Windows configuration.

### Example

```json
    "windows": {
        "ignoreFlushesDuringBoot": true
    }
```

## <a name="configWindowsHyperV" />HyperV

`hyperv` is an OPTIONAL field of the Windows configuration.
If present, the container MUST be run with Hyper-V isolation.
If omitted, the container MUST be run as a Windows Server container.

The following parameters can be specified:

* **`utilityVMPath`** *(string, OPTIONAL)* - specifies the path to the image used for the utility VM.
    This would be specified if using a base image which does not contain a utility VM image.
    If not supplied, the runtime will search the container filesystem layers from the bottom-most layer upwards, until it locates "UtilityVM", and default to that path.

### Example

```json
    "windows": {
        "hyperv": {
            "utilityVMPath": "C:\\path\\to\\utilityvm"
        }
    }
```
