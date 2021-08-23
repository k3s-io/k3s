/*
Copyright 2017 The Kubernetes Authors.

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
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"

	authenticationv1 "k8s.io/api/authentication/v1"
	api "k8s.io/api/core/v1"
	storage "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/features"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/util"
	volumetypes "k8s.io/kubernetes/pkg/volume/util/types"
	"k8s.io/mount-utils"
	utilstrings "k8s.io/utils/strings"
)

//TODO (vladimirvivien) move this in a central loc later
var (
	volDataKey = struct {
		specVolID,
		volHandle,
		driverName,
		nodeName,
		attachmentID,
		volumeLifecycleMode string
	}{
		"specVolID",
		"volumeHandle",
		"driverName",
		"nodeName",
		"attachmentID",
		"volumeLifecycleMode",
	}
)

type csiMountMgr struct {
	csiClientGetter
	k8s                 kubernetes.Interface
	plugin              *csiPlugin
	driverName          csiDriverName
	volumeLifecycleMode storage.VolumeLifecycleMode
	fsGroupPolicy       storage.FSGroupPolicy
	volumeID            string
	specVolumeID        string
	readOnly            bool
	supportsSELinux     bool
	spec                *volume.Spec
	pod                 *api.Pod
	podUID              types.UID
	publishContext      map[string]string
	kubeVolHost         volume.KubeletVolumeHost
	volume.MetricsProvider
}

// volume.Volume methods
var _ volume.Volume = &csiMountMgr{}

func (c *csiMountMgr) GetPath() string {
	dir := GetCSIMounterPath(filepath.Join(getTargetPath(c.podUID, c.specVolumeID, c.plugin.host)))
	klog.V(4).Info(log("mounter.GetPath generated [%s]", dir))
	return dir
}

func getTargetPath(uid types.UID, specVolumeID string, host volume.VolumeHost) string {
	specVolID := utilstrings.EscapeQualifiedName(specVolumeID)
	return host.GetPodVolumeDir(uid, utilstrings.EscapeQualifiedName(CSIPluginName), specVolID)
}

// volume.Mounter methods
var _ volume.Mounter = &csiMountMgr{}

func (c *csiMountMgr) CanMount() error {
	return nil
}

func (c *csiMountMgr) SetUp(mounterArgs volume.MounterArgs) error {
	return c.SetUpAt(c.GetPath(), mounterArgs)
}

func (c *csiMountMgr) SetUpAt(dir string, mounterArgs volume.MounterArgs) error {
	klog.V(4).Infof(log("Mounter.SetUpAt(%s)", dir))

	csi, err := c.csiClientGetter.Get()
	if err != nil {
		return volumetypes.NewTransientOperationFailure(log("mounter.SetUpAt failed to get CSI client: %v", err))

	}
	ctx, cancel := createCSIOperationContext(c.spec, csiTimeout)
	defer cancel()

	volSrc, pvSrc, err := getSourceFromSpec(c.spec)
	if err != nil {
		return errors.New(log("mounter.SetupAt failed to get CSI persistent source: %v", err))
	}

	driverName := c.driverName
	volumeHandle := c.volumeID
	readOnly := c.readOnly
	accessMode := api.ReadWriteOnce

	var (
		fsType             string
		volAttribs         map[string]string
		nodePublishSecrets map[string]string
		publishContext     map[string]string
		mountOptions       []string
		deviceMountPath    string
		secretRef          *api.SecretReference
	)

	switch {
	case volSrc != nil:
		if !utilfeature.DefaultFeatureGate.Enabled(features.CSIInlineVolume) {
			return fmt.Errorf("CSIInlineVolume feature required")
		}
		if c.volumeLifecycleMode != storage.VolumeLifecycleEphemeral {
			return fmt.Errorf("unexpected volume mode: %s", c.volumeLifecycleMode)
		}
		if volSrc.FSType != nil {
			fsType = *volSrc.FSType
		}

		volAttribs = volSrc.VolumeAttributes

		if volSrc.NodePublishSecretRef != nil {
			secretName := volSrc.NodePublishSecretRef.Name
			ns := c.pod.Namespace
			secretRef = &api.SecretReference{Name: secretName, Namespace: ns}
		}
	case pvSrc != nil:
		if c.volumeLifecycleMode != storage.VolumeLifecyclePersistent {
			return fmt.Errorf("unexpected driver mode: %s", c.volumeLifecycleMode)
		}

		fsType = pvSrc.FSType

		volAttribs = pvSrc.VolumeAttributes

		if pvSrc.NodePublishSecretRef != nil {
			secretRef = pvSrc.NodePublishSecretRef
		}

		//TODO (vladimirvivien) implement better AccessModes mapping between k8s and CSI
		if c.spec.PersistentVolume.Spec.AccessModes != nil {
			accessMode = c.spec.PersistentVolume.Spec.AccessModes[0]
		}

		mountOptions = c.spec.PersistentVolume.Spec.MountOptions

		// Check for STAGE_UNSTAGE_VOLUME set and populate deviceMountPath if so
		stageUnstageSet, err := csi.NodeSupportsStageUnstage(ctx)
		if err != nil {
			return errors.New(log("mounter.SetUpAt failed to check for STAGE_UNSTAGE_VOLUME capability: %v", err))
		}

		if stageUnstageSet {
			deviceMountPath, err = makeDeviceMountPath(c.plugin, c.spec)
			if err != nil {
				return errors.New(log("mounter.SetUpAt failed to make device mount path: %v", err))
			}
		}

		// search for attachment by VolumeAttachment.Spec.Source.PersistentVolumeName
		if c.publishContext == nil {
			nodeName := string(c.plugin.host.GetNodeName())
			c.publishContext, err = c.plugin.getPublishContext(c.k8s, volumeHandle, string(driverName), nodeName)
			if err != nil {
				// we could have a transient error associated with fetching publish context
				return volumetypes.NewTransientOperationFailure(log("mounter.SetUpAt failed to fetch publishContext: %v", err))
			}
			publishContext = c.publishContext
		}

	default:
		return fmt.Errorf("volume source not found in volume.Spec")
	}

	// create target_dir before call to NodePublish
	parentDir := filepath.Dir(dir)
	if err := os.MkdirAll(parentDir, 0750); err != nil {
		return errors.New(log("mounter.SetUpAt failed to create dir %#v:  %v", parentDir, err))
	}
	klog.V(4).Info(log("created target path successfully [%s]", parentDir))

	nodePublishSecrets = map[string]string{}
	if secretRef != nil {
		nodePublishSecrets, err = getCredentialsFromSecret(c.k8s, secretRef)
		if err != nil {
			return volumetypes.NewTransientOperationFailure(fmt.Sprintf("fetching NodePublishSecretRef %s/%s failed: %v",
				secretRef.Namespace, secretRef.Name, err))
		}

	}

	// Inject pod information into volume_attributes
	podInfoEnabled, err := c.plugin.podInfoEnabled(string(c.driverName))
	if err != nil {
		return volumetypes.NewTransientOperationFailure(log("mounter.SetUpAt failed to assemble volume attributes: %v", err))
	}
	if podInfoEnabled {
		volAttribs = mergeMap(volAttribs, getPodInfoAttrs(c.pod, c.volumeLifecycleMode))
	}

	// Inject pod service account token into volume attributes
	serviceAccountTokenAttrs, err := c.podServiceAccountTokenAttrs()
	if err != nil {
		return volumetypes.NewTransientOperationFailure(log("mounter.SetUpAt failed to get service accoount token attributes: %v", err))
	}
	volAttribs = mergeMap(volAttribs, serviceAccountTokenAttrs)

	driverSupportsCSIVolumeMountGroup := false
	var nodePublishFSGroupArg *int64
	if utilfeature.DefaultFeatureGate.Enabled(features.DelegateFSGroupToCSIDriver) {
		driverSupportsCSIVolumeMountGroup, err = csi.NodeSupportsVolumeMountGroup(ctx)
		if err != nil {
			return volumetypes.NewTransientOperationFailure(log("mounter.SetUpAt failed to determine if the node service has VOLUME_MOUNT_GROUP capability: %v", err))
		}

		if driverSupportsCSIVolumeMountGroup {
			klog.V(3).Infof("Driver %s supports applying FSGroup (has VOLUME_MOUNT_GROUP node capability). Delegating FSGroup application to the driver through NodePublishVolume.", c.driverName)
			nodePublishFSGroupArg = mounterArgs.FsGroup
		}
	}

	err = csi.NodePublishVolume(
		ctx,
		volumeHandle,
		readOnly,
		deviceMountPath,
		dir,
		accessMode,
		publishContext,
		volAttribs,
		nodePublishSecrets,
		fsType,
		mountOptions,
		nodePublishFSGroupArg,
	)

	if err != nil {
		// If operation finished with error then we can remove the mount directory.
		if volumetypes.IsOperationFinishedError(err) {
			if removeMountDirErr := removeMountDir(c.plugin, dir); removeMountDirErr != nil {
				klog.Error(log("mounter.SetupAt failed to remove mount dir after a NodePublish() error [%s]: %v", dir, removeMountDirErr))
			}
		}
		return err
	}

	c.supportsSELinux, err = c.kubeVolHost.GetHostUtil().GetSELinuxSupport(dir)
	if err != nil {
		klog.V(2).Info(log("error checking for SELinux support: %s", err))
	}

	if !driverSupportsCSIVolumeMountGroup && c.supportsFSGroup(fsType, mounterArgs.FsGroup, c.fsGroupPolicy) {
		// Driver doesn't support applying FSGroup. Kubelet must apply it instead.

		// fullPluginName helps to distinguish different driver from csi plugin
		err := volume.SetVolumeOwnership(c, mounterArgs.FsGroup, mounterArgs.FSGroupChangePolicy, util.FSGroupCompleteHook(c.plugin, c.spec))
		if err != nil {
			// At this point mount operation is successful:
			//   1. Since volume can not be used by the pod because of invalid permissions, we must return error
			//   2. Since mount is successful, we must record volume as mounted in uncertain state, so it can be
			//      cleaned up.
			return volumetypes.NewUncertainProgressError(fmt.Sprintf("applyFSGroup failed for vol %s: %v", c.volumeID, err))
		}
		klog.V(4).Info(log("mounter.SetupAt fsGroup [%d] applied successfully to %s", *mounterArgs.FsGroup, c.volumeID))
	}

	klog.V(4).Infof(log("mounter.SetUp successfully requested NodePublish [%s]", dir))
	return nil
}

func (c *csiMountMgr) podServiceAccountTokenAttrs() (map[string]string, error) {
	if c.plugin.serviceAccountTokenGetter == nil {
		return nil, errors.New("ServiceAccountTokenGetter is nil")
	}

	csiDriver, err := c.plugin.csiDriverLister.Get(string(c.driverName))
	if err != nil {
		if apierrors.IsNotFound(err) {
			klog.V(5).Infof(log("CSIDriver %q not found, not adding service account token information", c.driverName))
			return nil, nil
		}
		return nil, err
	}

	if len(csiDriver.Spec.TokenRequests) == 0 {
		return nil, nil
	}

	outputs := map[string]authenticationv1.TokenRequestStatus{}
	for _, tokenRequest := range csiDriver.Spec.TokenRequests {
		audience := tokenRequest.Audience
		audiences := []string{audience}
		if audience == "" {
			audiences = []string{}
		}
		tr, err := c.plugin.serviceAccountTokenGetter(c.pod.Namespace, c.pod.Spec.ServiceAccountName, &authenticationv1.TokenRequest{
			Spec: authenticationv1.TokenRequestSpec{
				Audiences:         audiences,
				ExpirationSeconds: tokenRequest.ExpirationSeconds,
				BoundObjectRef: &authenticationv1.BoundObjectReference{
					APIVersion: "v1",
					Kind:       "Pod",
					Name:       c.pod.Name,
					UID:        c.pod.UID,
				},
			},
		})
		if err != nil {
			return nil, err
		}

		outputs[audience] = tr.Status
	}

	klog.V(4).Infof(log("Fetched service account token attrs for CSIDriver %q", c.driverName))
	tokens, _ := json.Marshal(outputs)
	return map[string]string{
		"csi.storage.k8s.io/serviceAccount.tokens": string(tokens),
	}, nil
}

func (c *csiMountMgr) GetAttributes() volume.Attributes {
	return volume.Attributes{
		ReadOnly:        c.readOnly,
		Managed:         !c.readOnly,
		SupportsSELinux: c.supportsSELinux,
	}
}

// volume.Unmounter methods
var _ volume.Unmounter = &csiMountMgr{}

func (c *csiMountMgr) TearDown() error {
	return c.TearDownAt(c.GetPath())
}
func (c *csiMountMgr) TearDownAt(dir string) error {
	klog.V(4).Infof(log("Unmounter.TearDown(%s)", dir))

	volID := c.volumeID
	csi, err := c.csiClientGetter.Get()
	if err != nil {
		return errors.New(log("mounter.SetUpAt failed to get CSI client: %v", err))
	}

	// Could not get spec info on whether this is a migrated operation because c.spec is nil
	ctx, cancel := createCSIOperationContext(c.spec, csiTimeout)
	defer cancel()

	if err := csi.NodeUnpublishVolume(ctx, volID, dir); err != nil {
		return errors.New(log("mounter.TearDownAt failed: %v", err))
	}

	// Deprecation: Removal of target_path provided in the NodePublish RPC call
	// (in this case location `dir`) MUST be done by the CSI plugin according
	// to the spec. This will no longer be done directly as part of TearDown
	// by the kubelet in the future. Kubelet will only be responsible for
	// removal of json data files it creates and parent directories.
	if err := removeMountDir(c.plugin, dir); err != nil {
		return errors.New(log("mounter.TearDownAt failed to clean mount dir [%s]: %v", dir, err))
	}
	klog.V(4).Infof(log("mounter.TearDownAt successfully unmounted dir [%s]", dir))

	return nil
}

func (c *csiMountMgr) supportsFSGroup(fsType string, fsGroup *int64, driverPolicy storage.FSGroupPolicy) bool {
	if fsGroup == nil || driverPolicy == storage.NoneFSGroupPolicy || c.readOnly {
		return false
	}

	if driverPolicy == storage.FileFSGroupPolicy {
		return true
	}

	if fsType == "" {
		klog.V(4).Info(log("mounter.SetupAt WARNING: skipping fsGroup, fsType not provided"))
		return false
	}

	if c.spec.PersistentVolume == nil {
		klog.V(4).Info(log("mounter.SetupAt Warning: skipping fsGroup permission change, no access mode available. The volume may only be accessible to root users."))
		return false
	}
	if c.spec.PersistentVolume.Spec.AccessModes == nil {
		klog.V(4).Info(log("mounter.SetupAt WARNING: skipping fsGroup, access modes not provided"))
		return false
	}
	if !hasReadWriteOnce(c.spec.PersistentVolume.Spec.AccessModes) {
		klog.V(4).Info(log("mounter.SetupAt WARNING: skipping fsGroup, only support ReadWriteOnce access mode"))
		return false
	}
	return true
}

// isDirMounted returns the !notMounted result from IsLikelyNotMountPoint check
func isDirMounted(plug *csiPlugin, dir string) (bool, error) {
	mounter := plug.host.GetMounter(plug.GetPluginName())
	notMnt, err := mounter.IsLikelyNotMountPoint(dir)
	if err != nil && !os.IsNotExist(err) {
		klog.Error(log("isDirMounted IsLikelyNotMountPoint test failed for dir [%v]", dir))
		return false, err
	}
	return !notMnt, nil
}

func isCorruptedDir(dir string) bool {
	_, pathErr := mount.PathExists(dir)
	return pathErr != nil && mount.IsCorruptedMnt(pathErr)
}

// removeMountDir cleans the mount dir when dir is not mounted and removed the volume data file in dir
func removeMountDir(plug *csiPlugin, mountPath string) error {
	klog.V(4).Info(log("removing mount path [%s]", mountPath))

	mnt, err := isDirMounted(plug, mountPath)
	if err != nil {
		return err
	}
	if !mnt {
		klog.V(4).Info(log("dir not mounted, deleting it [%s]", mountPath))
		if err := os.Remove(mountPath); err != nil && !os.IsNotExist(err) {
			return errors.New(log("failed to remove dir [%s]: %v", mountPath, err))
		}
		// remove volume data file as well
		volPath := filepath.Dir(mountPath)
		dataFile := filepath.Join(volPath, volDataFileName)
		klog.V(4).Info(log("also deleting volume info data file [%s]", dataFile))
		if err := os.Remove(dataFile); err != nil && !os.IsNotExist(err) {
			return errors.New(log("failed to delete volume data file [%s]: %v", dataFile, err))
		}
		// remove volume path
		klog.V(4).Info(log("deleting volume path [%s]", volPath))
		if err := os.Remove(volPath); err != nil && !os.IsNotExist(err) {
			return errors.New(log("failed to delete volume path [%s]: %v", volPath, err))
		}
	}
	return nil
}

// makeVolumeHandle returns csi-<sha256(podUID,volSourceSpecName)>
func makeVolumeHandle(podUID, volSourceSpecName string) string {
	result := sha256.Sum256([]byte(fmt.Sprintf("%s%s", podUID, volSourceSpecName)))
	return fmt.Sprintf("csi-%x", result)
}

func mergeMap(first, second map[string]string) map[string]string {
	if first == nil && second != nil {
		return second
	}
	for k, v := range second {
		first[k] = v
	}
	return first
}
