package etcd

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	apisv1 "github.com/k3s-io/k3s/pkg/apis/k3s.cattle.io/v1"
	controllersv1 "github.com/k3s-io/k3s/pkg/generated/controllers/k3s.cattle.io/v1"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	controllerv1 "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	"github.com/sirupsen/logrus"
)

const (
	pruneStepSize     = 4
	reconcileKey      = "_reconcile_"
	reconcileInterval = 600 * time.Minute
)

var (
	snapshotConfigMapName = version.Program + "-etcd-snapshots"
	errNotReconciled      = errors.New("no nodes have reconciled ETCDSnapshotFile resources")
)

type etcdSnapshotHandler struct {
	ctx        context.Context
	etcd       *ETCD
	snapshots  controllersv1.ETCDSnapshotFileController
	configmaps controllerv1.ConfigMapController
}

func registerSnapshotHandlers(ctx context.Context, etcd *ETCD) {
	snapshots := etcd.config.Runtime.K3s.K3s().V1().ETCDSnapshotFile()
	e := &etcdSnapshotHandler{
		ctx:        ctx,
		etcd:       etcd,
		snapshots:  snapshots,
		configmaps: etcd.config.Runtime.Core.Core().V1().ConfigMap(),
	}

	logrus.Infof("Starting managed etcd snapshot ConfigMap controller")
	snapshots.OnChange(ctx, "managed-etcd-snapshots-controller", e.sync)
	snapshots.OnRemove(ctx, "managed-etcd-snapshots-controller", e.onRemove)
	go wait.JitterUntil(func() { snapshots.Enqueue(reconcileKey) }, reconcileInterval, 0.04, false, ctx.Done())
}

func (e *etcdSnapshotHandler) sync(key string, esf *apisv1.ETCDSnapshotFile) (*apisv1.ETCDSnapshotFile, error) {
	if key == reconcileKey {
		err := e.reconcile()
		if err == errNotReconciled {
			logrus.Debugf("Failed to reconcile snapshot ConfigMap: %v, requeuing", err)
			e.snapshots.Enqueue(key)
			return nil, nil
		}
		return nil, err
	}
	if esf == nil || !esf.DeletionTimestamp.IsZero() {
		return nil, nil
	}

	sf := snapshotFile{}
	sf.fromETCDSnapshotFile(esf)
	sfKey := generateSnapshotConfigMapKey(sf)
	m, err := marshalSnapshotFile(sf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal snapshot ConfigMap data")
	}
	marshalledSnapshot := string(m)

	snapshotConfigMap, err := e.configmaps.Get(metav1.NamespaceSystem, snapshotConfigMapName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, errors.Wrap(err, "failed to get snapshot ConfigMap")
		}
		snapshotConfigMap = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapshotConfigMapName,
				Namespace: metav1.NamespaceSystem,
			},
		}
	}

	if snapshotConfigMap.Data[sfKey] != marshalledSnapshot {
		if snapshotConfigMap.Data == nil {
			snapshotConfigMap.Data = map[string]string{}
		}
		snapshotConfigMap.Data[sfKey] = marshalledSnapshot

		// Try to create or update the ConfigMap. If it is too large, prune old entries
		// until it fits, or until it cannot be pruned any further.
		pruneCount := pruneStepSize
		err = retry.OnError(snapshotDataBackoff, isTooLargeError, func() (err error) {
			if snapshotConfigMap.CreationTimestamp.IsZero() {
				_, err = e.configmaps.Create(snapshotConfigMap)
			} else {
				_, err = e.configmaps.Update(snapshotConfigMap)
			}

			if isTooLargeError(err) {
				logrus.Warnf("Snapshot ConfigMap is too large, attempting to elide %d of %d entries to reduce size", pruneCount, len(snapshotConfigMap.Data))
				if perr := pruneConfigMap(snapshotConfigMap, pruneCount); perr != nil {
					err = perr
				}
				// if the entry we're trying to add just got pruned, give up on adding it,
				// as it is always going to get pushed off due to being too old to keep.
				if _, ok := snapshotConfigMap.Data[sfKey]; !ok {
					logrus.Warnf("Snapshot %s has been elided from ConfigMap to reduce size; not requeuing", key)
					return nil
				}

				pruneCount += pruneStepSize
			}
			return err
		})
	}

	if err != nil {
		err = errors.Wrap(err, "failed to sync snapshot to ConfigMap")
	}

	return nil, err
}

func (e *etcdSnapshotHandler) onRemove(key string, esf *apisv1.ETCDSnapshotFile) (*apisv1.ETCDSnapshotFile, error) {
	if esf == nil {
		return nil, nil
	}
	snapshotConfigMap, err := e.configmaps.Get(metav1.NamespaceSystem, snapshotConfigMapName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, errors.Wrap(err, "failed to get snapshot ConfigMap")
	}

	sfKey := generateETCDSnapshotFileConfigMapKey(*esf)
	if _, ok := snapshotConfigMap.Data[sfKey]; ok {
		delete(snapshotConfigMap.Data, sfKey)
		if _, err := e.configmaps.Update(snapshotConfigMap); err != nil {
			return nil, errors.Wrap(err, "failed to remove snapshot from ConfigMap")
		}
	}
	e.etcd.emitEvent(esf)
	return nil, nil
}

func (e *etcdSnapshotHandler) reconcile() error {
	logrus.Infof("Reconciling snapshot ConfigMap data")

	snapshotConfigMap, err := e.configmaps.Get(metav1.NamespaceSystem, snapshotConfigMapName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrap(err, "failed to get snapshot ConfigMap")
		}
		snapshotConfigMap = &v1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapshotConfigMapName,
				Namespace: metav1.NamespaceSystem,
			},
		}
	}

	// Get a list of all etcd nodes currently in the cluster.
	// We will use this list to prune local entries for any node that does not exist.
	nodes := e.etcd.config.Runtime.Core.Core().V1().Node()
	etcdSelector := labels.Set{util.ETCDRoleLabelKey: "true"}
	nodeList, err := nodes.List(metav1.ListOptions{LabelSelector: etcdSelector.String()})
	if err != nil {
		return err
	}

	// Once a node has set the reconcile annotation, it is considered to have
	// migrated to using ETCDSnapshotFile resources, and any old configmap
	// entries for it can be pruned. Until the annotation is set, we will leave
	// its entries alone.
	syncedNodes := map[string]bool{}
	for _, node := range nodeList.Items {
		if _, ok := node.Annotations[annotationLocalReconciled]; ok {
			syncedNodes[node.Name] = true
		}
		if _, ok := node.Annotations[annotationS3Reconciled]; ok {
			syncedNodes["s3"] = true
		}
	}

	if len(syncedNodes) == 0 {
		return errNotReconciled
	}

	// Get a list of existing snapshots
	snapshotList, err := e.snapshots.List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	snapshots := map[string]*apisv1.ETCDSnapshotFile{}
	for i := range snapshotList.Items {
		esf := &snapshotList.Items[i]
		if esf.DeletionTimestamp.IsZero() {
			sfKey := generateETCDSnapshotFileConfigMapKey(*esf)
			snapshots[sfKey] = esf
		}
	}

	// Make a copy of the configmap for change detection
	existing := snapshotConfigMap.DeepCopyObject()

	// Delete any keys missing from synced storages, or associated with missing nodes
	for key := range snapshotConfigMap.Data {
		if strings.HasPrefix(key, "s3-") {
			// If a node has syncd s3 and the key is missing then delete it
			if syncedNodes["s3"] && snapshots[key] == nil {
				delete(snapshotConfigMap.Data, key)
			}
		} else if s, ok := strings.CutPrefix(key, "local-"); ok {
			// If a matching node has synced and the key is missing then delete it
			// If a matching node does not exist, delete the key
			// A node is considered to match the snapshot if the snapshot name matches the node name
			// after trimming the leading local- prefix and trailing timestamp and extension.
			s, _ = strings.CutSuffix(s, ".zip")
			s = strings.TrimRight(s, "-012345678")
			var matchingNode bool
			for _, node := range nodeList.Items {
				if strings.HasSuffix(s, node.Name) {
					if syncedNodes[node.Name] && snapshots[key] == nil {
						delete(snapshotConfigMap.Data, key)
					}
					matchingNode = true
					break
				}
			}
			if !matchingNode {
				delete(snapshotConfigMap.Data, key)
			}
		}
	}

	// Ensure keys for existing snapshots
	for sfKey, esf := range snapshots {
		sf := snapshotFile{}
		sf.fromETCDSnapshotFile(esf)
		m, err := marshalSnapshotFile(sf)
		if err != nil {
			logrus.Warnf("Failed to marshal snapshot ConfigMap data for %s", sfKey)
			continue
		}
		marshalledSnapshot := string(m)
		snapshotConfigMap.Data[sfKey] = marshalledSnapshot
	}

	// If the configmap didn't change, don't bother updating it
	if equality.Semantic.DeepEqual(existing, snapshotConfigMap) {
		return nil
	}

	// Try to create or update the ConfigMap. If it is too large, prune old entries
	// until it fits, or until it cannot be pruned any further.
	pruneCount := pruneStepSize
	return retry.OnError(snapshotDataBackoff, isTooLargeError, func() (err error) {
		if snapshotConfigMap.CreationTimestamp.IsZero() {
			_, err = e.configmaps.Create(snapshotConfigMap)
		} else {
			_, err = e.configmaps.Update(snapshotConfigMap)
		}

		if isTooLargeError(err) {
			logrus.Warnf("Snapshot ConfigMap is too large, attempting to elide %d of %d entries to reduce size", pruneCount, len(snapshotConfigMap.Data))
			if perr := pruneConfigMap(snapshotConfigMap, pruneCount); perr != nil {
				err = perr
			}
			pruneCount += pruneStepSize
		}
		return err
	})
}

// pruneConfigMap drops the oldest entries from the configMap.
// Note that the actual snapshot files are not removed, just the entries that track them in the configmap.
func pruneConfigMap(snapshotConfigMap *v1.ConfigMap, pruneCount int) error {
	if pruneCount >= len(snapshotConfigMap.Data) {
		return errors.New("unable to reduce snapshot ConfigMap size by eliding old snapshots")
	}

	var snapshotFiles []snapshotFile
	retention := len(snapshotConfigMap.Data) - pruneCount
	for name := range snapshotConfigMap.Data {
		basename, compressed := strings.CutSuffix(name, compressedExtension)
		ts, _ := strconv.ParseInt(basename[strings.LastIndexByte(basename, '-')+1:], 10, 64)
		snapshotFiles = append(snapshotFiles, snapshotFile{Name: name, CreatedAt: &metav1.Time{Time: time.Unix(ts, 0)}, Compressed: compressed})
	}

	// sort newest-first so we can prune entries past the retention count
	sort.Slice(snapshotFiles, func(i, j int) bool {
		return snapshotFiles[j].CreatedAt.Before(snapshotFiles[i].CreatedAt)
	})

	for _, snapshotFile := range snapshotFiles[retention:] {
		delete(snapshotConfigMap.Data, snapshotFile.Name)
	}
	return nil
}

func isTooLargeError(err error) bool {
	// There are no helpers for unpacking field validation errors, so we just check for "Too long" in the error string.
	return apierrors.IsRequestEntityTooLargeError(err) || (apierrors.IsInvalid(err) && strings.Contains(err.Error(), "Too long"))
}
