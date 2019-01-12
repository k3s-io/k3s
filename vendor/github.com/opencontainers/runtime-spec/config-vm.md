# <a name="VirtualMachineSpecificContainerConfiguration" /> Virtual-machine-specific Container Configuration

This section describes the schema for the [virtual-machine-specific section](config.md#platform-specific-configuration) of the [container configuration](config.md).
The virtual-machine container specification provides additional configuration for the hypervisor, kernel, and image.

## <a name="HypervisorObject" /> Hypervisor Object

**`hypervisor`** (object, OPTIONAL) specifies details of the hypervisor that manages the container virtual machine.
* **`path`** (string, REQUIRED) path to the hypervisor binary that manages the container virtual machine.
    This value MUST be an absolute path in the [runtime mount namespace](glossary.md#runtime-namespace).
* **`parameters`** (array of strings, OPTIONAL) specifies an array of parameters to pass to the hypervisor.

### Example

```json
    "hypervisor": {
        "path": "/path/to/vmm",
        "parameters": ["opts1=foo", "opts2=bar"]
    }
```

## <a name="KernelObject" /> Kernel Object

**`kernel`** (object, REQUIRED) specifies details of the kernel to boot the container virtual machine with.
* **`path`** (string, REQUIRED) path to the kernel used to boot the container virtual machine.
    This value MUST be an absolute path in the [runtime mount namespace](glossary.md#runtime-namespace).
* **`parameters`** (array of strings, OPTIONAL) specifies an array of parameters to pass to the kernel.
* **`initrd`** (string, OPTIONAL) path to an initial ramdisk to be used by the container virtual machine.
    This value MUST be an absolute path in the [runtime mount namespace](glossary.md#runtime-namespace).

### Example

```json
    "kernel": {
        "path": "/path/to/vmlinuz",
        "parameters": ["foo=bar", "hello world"],
        "initrd": "/path/to/initrd.img"
    }
```

## <a name="ImageObject" /> Image Object

**`image`** (object, OPTIONAL) specifies details of the image that contains the root filesystem for the container virtual machine.
* **`path`** (string, REQUIRED) path to the container virtual machine root image.
    This value MUST be an absolute path in the [runtime mount namespace](glossary.md#runtime-namespace).
* **`format`** (string, REQUIRED) format of the container virtual machine root image. Commonly supported formats are:
    * **`raw`** [raw disk image format][raw-image-format]. Unset values for `format` will default to that format.
    * **`qcow2`** [QEMU image format][qcow2-image-format].
    * **`vdi`** [VirtualBox 1.1 compatible image format][vdi-image-format].
    * **`vmdk`** [VMware compatible image format][vmdk-image-format].
    * **`vhd`** [Virtual Hard Disk image format][vhd-image-format].

This image contains the root filesystem that the virtual machine **`kernel`** will boot into, not to be confused with the container root filesystem itself. The latter, as specified by **`path`** from the [Root Configuration](config.md#Root-Configuration) section, will be mounted inside the virtual machine at a location chosen by the virtual-machine-based runtime.

### Example

```json
    "image": {
        "path": "/path/to/vm/rootfs.img",
	"format": "raw"
    }
```

[raw-image-format]: https://en.wikipedia.org/wiki/IMG_(file_format)
[qcow2-image-format]: https://git.qemu.org/?p=qemu.git;a=blob_plain;f=docs/interop/qcow2.txt;hb=HEAD
[vdi-image-format]: https://forensicswiki.org/wiki/Virtual_Disk_Image_(VDI)
[vmdk-image-format]: http://www.vmware.com/app/vmdk/?src=vmdk
[vhd-image-format]: https://github.com/libyal/libvhdi/blob/master/documentation/Virtual%20Hard%20Disk%20(VHD)%20image%20format.asciidoc
