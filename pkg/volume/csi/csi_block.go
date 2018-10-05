/*
Copyright 2018 The Kubernetes Authors.

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

package csi

import (
	"errors"
	"path/filepath"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/volume"
)

type csiBlockMapper struct {
	k8s        kubernetes.Interface
	csiClient  csiClient
	plugin     *csiPlugin
	driverName string
	specName   string
	volumeID   string
	readOnly   bool
	spec       *volume.Spec
	podUID     types.UID
	volumeInfo map[string]string
}

var _ volume.BlockVolumeMapper = &csiBlockMapper{}

// GetGlobalMapPath returns a path (on the node) to a device file which will be symlinked to
// Example: plugins/kubernetes.io/csi/volumeDevices/{volumeID}/dev
func (m *csiBlockMapper) GetGlobalMapPath(spec *volume.Spec) (string, error) {
	dir := getVolumeDevicePluginDir(spec.Name(), m.plugin.host)
	glog.V(4).Infof(log("blockMapper.GetGlobalMapPath = %s", dir))
	return dir, nil
}

// GetPodDeviceMapPath returns pod's device file which will be mapped to a volume
// returns: pods/{podUid}/volumeDevices/kubernetes.io~csi/{volumeID}/dev, {volumeID}
func (m *csiBlockMapper) GetPodDeviceMapPath() (string, string) {
	path := filepath.Join(m.plugin.host.GetPodVolumeDeviceDir(m.podUID, csiPluginName), m.specName, "dev")
	specName := m.specName
	glog.V(4).Infof(log("blockMapper.GetPodDeviceMapPath [path=%s; name=%s]", path, specName))
	return path, specName
}

// SetUpDevice ensures the device is attached returns path where the device is located.
func (m *csiBlockMapper) SetUpDevice() (string, error) {
	return "", errors.New("CSIBlockVolume feature not enabled")
}

func (m *csiBlockMapper) MapDevice(devicePath, globalMapPath, volumeMapPath, volumeMapName string, podUID types.UID) error {
	return errors.New("CSIBlockVolume feature not enabled")
}

var _ volume.BlockVolumeUnmapper = &csiBlockMapper{}

// TearDownDevice removes traces of the SetUpDevice.
func (m *csiBlockMapper) TearDownDevice(globalMapPath, devicePath string) error {
	return errors.New("CSIBlockVolume feature not enabled")
}
