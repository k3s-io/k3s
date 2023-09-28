package etcd

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/client/pkg/v3/logutil"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

const (
	maxConcurrentSnapshots = 1
	compressedExtension    = ".zip"
)

var (
	snapshotExtraMetadataConfigMapName = version.Program + "-etcd-snapshot-extra-metadata"
	snapshotConfigMapName              = version.Program + "-etcd-snapshots"

	// snapshotDataBackoff will retry at increasing steps for up to ~30 seconds.
	// If the ConfigMap update fails, the list won't be reconciled again until next time
	// the server starts, so we should be fairly persistent in retrying.
	snapshotDataBackoff = wait.Backoff{
		Steps:    9,
		Duration: 10 * time.Millisecond,
		Factor:   3.0,
		Jitter:   0.1,
	}

	// cronLogger wraps logrus's Printf output as cron-compatible logger
	cronLogger = cron.VerbosePrintfLogger(logrus.StandardLogger())
)

// snapshotDir ensures that the snapshot directory exists, and then returns its path.
func snapshotDir(config *config.Control, create bool) (string, error) {
	if config.EtcdSnapshotDir == "" {
		// we have to create the snapshot dir if we are using
		// the default snapshot dir if it doesn't exist
		defaultSnapshotDir := filepath.Join(config.DataDir, "db", "snapshots")
		s, err := os.Stat(defaultSnapshotDir)
		if err != nil {
			if create && os.IsNotExist(err) {
				if err := os.MkdirAll(defaultSnapshotDir, 0700); err != nil {
					return "", err
				}
				return defaultSnapshotDir, nil
			}
			return "", err
		}
		if s.IsDir() {
			return defaultSnapshotDir, nil
		}
	}
	return config.EtcdSnapshotDir, nil
}

// preSnapshotSetup checks to see if the necessary components are in place
// to perform an Etcd snapshot. This is necessary primarily for on-demand
// snapshots since they're performed before normal Etcd setup is completed.
func (e *ETCD) preSnapshotSetup(ctx context.Context) error {
	if e.snapshotSem == nil {
		e.snapshotSem = semaphore.NewWeighted(maxConcurrentSnapshots)
	}
	return nil
}

// compressSnapshot compresses the given snapshot and provides the
// caller with the path to the file.
func (e *ETCD) compressSnapshot(snapshotDir, snapshotName, snapshotPath string) (string, error) {
	logrus.Info("Compressing etcd snapshot file: " + snapshotName)

	zippedSnapshotName := snapshotName + compressedExtension
	zipPath := filepath.Join(snapshotDir, zippedSnapshotName)

	zf, err := os.Create(zipPath)
	if err != nil {
		return "", err
	}
	defer zf.Close()

	zipWriter := zip.NewWriter(zf)
	defer zipWriter.Close()

	uncompressedPath := filepath.Join(snapshotDir, snapshotName)
	fileToZip, err := os.Open(uncompressedPath)
	if err != nil {
		os.Remove(zipPath)
		return "", err
	}
	defer fileToZip.Close()

	info, err := fileToZip.Stat()
	if err != nil {
		os.Remove(zipPath)
		return "", err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		os.Remove(zipPath)
		return "", err
	}

	header.Name = snapshotName
	header.Method = zip.Deflate
	header.Modified = time.Now()

	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		os.Remove(zipPath)
		return "", err
	}
	_, err = io.Copy(writer, fileToZip)

	return zipPath, err
}

// decompressSnapshot decompresses the given snapshot and provides the caller
// with the full path to the uncompressed snapshot.
func (e *ETCD) decompressSnapshot(snapshotDir, snapshotFile string) (string, error) {
	logrus.Info("Decompressing etcd snapshot file: " + snapshotFile)

	r, err := zip.OpenReader(snapshotFile)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var decompressed *os.File
	for _, sf := range r.File {
		decompressed, err = os.OpenFile(strings.Replace(sf.Name, compressedExtension, "", -1), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, sf.Mode())
		if err != nil {
			return "", err
		}
		defer decompressed.Close()

		ss, err := sf.Open()
		if err != nil {
			return "", err
		}
		defer ss.Close()

		if _, err := io.Copy(decompressed, ss); err != nil {
			os.Remove("")
			return "", err
		}
	}

	return decompressed.Name(), nil
}

// Snapshot attempts to save a new snapshot to the configured directory, and then clean up any old and failed
// snapshots in excess of the retention limits. This method is used in the internal cron snapshot
// system as well as used to do on-demand snapshots.
func (e *ETCD) Snapshot(ctx context.Context) error {
	if err := e.preSnapshotSetup(ctx); err != nil {
		return err
	}
	if !e.snapshotSem.TryAcquire(maxConcurrentSnapshots) {
		return fmt.Errorf("%d snapshots already in progress", maxConcurrentSnapshots)
	}
	defer e.snapshotSem.Release(maxConcurrentSnapshots)

	// make sure the core.Factory is initialized before attempting to add snapshot metadata
	var extraMetadata string
	if e.config.Runtime.Core == nil {
		logrus.Debugf("Cannot retrieve extra metadata from %s ConfigMap: runtime core not ready", snapshotExtraMetadataConfigMapName)
	} else {
		logrus.Debugf("Attempting to retrieve extra metadata from %s ConfigMap", snapshotExtraMetadataConfigMapName)
		if snapshotExtraMetadataConfigMap, err := e.config.Runtime.Core.Core().V1().ConfigMap().Get(metav1.NamespaceSystem, snapshotExtraMetadataConfigMapName, metav1.GetOptions{}); err != nil {
			logrus.Debugf("Error encountered attempting to retrieve extra metadata from %s ConfigMap, error: %v", snapshotExtraMetadataConfigMapName, err)
		} else {
			if m, err := json.Marshal(snapshotExtraMetadataConfigMap.Data); err != nil {
				logrus.Debugf("Error attempting to marshal extra metadata contained in %s ConfigMap, error: %v", snapshotExtraMetadataConfigMapName, err)
			} else {
				logrus.Debugf("Setting extra metadata from %s ConfigMap", snapshotExtraMetadataConfigMapName)
				logrus.Tracef("Marshalled extra metadata in %s ConfigMap was: %s", snapshotExtraMetadataConfigMapName, string(m))
				extraMetadata = base64.StdEncoding.EncodeToString(m)
			}
		}
	}

	endpoints := getEndpoints(e.config)
	var client *clientv3.Client
	var err error

	// Use the internal client if possible, or create a new one
	// if run from the CLI.
	if e.client != nil {
		client = e.client
	} else {
		client, err = getClient(ctx, e.config, endpoints...)
		if err != nil {
			return err
		}
		defer client.Close()
	}

	status, err := client.Status(ctx, endpoints[0])
	if err != nil {
		return errors.Wrap(err, "failed to check etcd status for snapshot")
	}

	if status.IsLearner {
		logrus.Warnf("Unable to take snapshot: not supported for learner")
		return nil
	}

	snapshotDir, err := snapshotDir(e.config, true)
	if err != nil {
		return errors.Wrap(err, "failed to get the snapshot dir")
	}

	cfg, err := getClientConfig(ctx, e.config)
	if err != nil {
		return errors.Wrap(err, "failed to get config for etcd snapshot")
	}

	nodeName := os.Getenv("NODE_NAME")
	now := time.Now()
	snapshotName := fmt.Sprintf("%s-%s-%d", e.config.EtcdSnapshotName, nodeName, now.Unix())
	snapshotPath := filepath.Join(snapshotDir, snapshotName)

	logrus.Infof("Saving etcd snapshot to %s", snapshotPath)

	var sf *snapshotFile

	lg, err := logutil.CreateDefaultZapLogger(zap.InfoLevel)
	if err != nil {
		return err
	}

	if err := snapshot.NewV3(lg).Save(ctx, *cfg, snapshotPath); err != nil {
		sf = &snapshotFile{
			Name:     snapshotName,
			Location: "",
			Metadata: extraMetadata,
			NodeName: nodeName,
			CreatedAt: &metav1.Time{
				Time: now,
			},
			Status:     failedSnapshotStatus,
			Message:    base64.StdEncoding.EncodeToString([]byte(err.Error())),
			Size:       0,
			Compressed: e.config.EtcdSnapshotCompress,
		}
		logrus.Errorf("Failed to take etcd snapshot: %v", err)
		if err := e.addSnapshotData(*sf); err != nil {
			return errors.Wrap(err, "failed to save local snapshot failure data to configmap")
		}
	}

	if e.config.EtcdSnapshotCompress {
		zipPath, err := e.compressSnapshot(snapshotDir, snapshotName, snapshotPath)
		if err != nil {
			return err
		}
		if err := os.Remove(snapshotPath); err != nil {
			return err
		}
		snapshotPath = zipPath
		logrus.Info("Compressed snapshot: " + snapshotPath)
	}

	// If the snapshot attempt was successful, sf will be nil as we did not set it.
	if sf == nil {
		f, err := os.Stat(snapshotPath)
		if err != nil {
			return errors.Wrap(err, "unable to retrieve snapshot information from local snapshot")
		}
		sf = &snapshotFile{
			Name:     f.Name(),
			Metadata: extraMetadata,
			Location: "file://" + snapshotPath,
			NodeName: nodeName,
			CreatedAt: &metav1.Time{
				Time: f.ModTime(),
			},
			Status:     successfulSnapshotStatus,
			Size:       f.Size(),
			Compressed: e.config.EtcdSnapshotCompress,
		}

		if err := e.addSnapshotData(*sf); err != nil {
			return errors.Wrap(err, "failed to save local snapshot data to configmap")
		}
		if err := snapshotRetention(e.config.EtcdSnapshotRetention, e.config.EtcdSnapshotName, snapshotDir); err != nil {
			return errors.Wrap(err, "failed to apply local snapshot retention policy")
		}

		if e.config.EtcdS3 {
			logrus.Infof("Saving etcd snapshot %s to S3", snapshotName)
			// Set sf to nil so that we can attempt to now upload the snapshot to S3 if needed
			sf = nil
			if err := e.initS3IfNil(ctx); err != nil {
				logrus.Warnf("Unable to initialize S3 client: %v", err)
				sf = &snapshotFile{
					Name:     filepath.Base(snapshotPath),
					Metadata: extraMetadata,
					NodeName: "s3",
					CreatedAt: &metav1.Time{
						Time: now,
					},
					Message: base64.StdEncoding.EncodeToString([]byte(err.Error())),
					Size:    0,
					Status:  failedSnapshotStatus,
					S3: &s3Config{
						Endpoint:      e.config.EtcdS3Endpoint,
						EndpointCA:    e.config.EtcdS3EndpointCA,
						SkipSSLVerify: e.config.EtcdS3SkipSSLVerify,
						Bucket:        e.config.EtcdS3BucketName,
						Region:        e.config.EtcdS3Region,
						Folder:        e.config.EtcdS3Folder,
						Insecure:      e.config.EtcdS3Insecure,
					},
				}
			}
			// sf should be nil if we were able to successfully initialize the S3 client.
			if sf == nil {
				sf, err = e.s3.upload(ctx, snapshotPath, extraMetadata, now)
				if err != nil {
					return err
				}
				logrus.Infof("S3 upload complete for %s", snapshotName)
				if err := e.s3.snapshotRetention(ctx); err != nil {
					return errors.Wrap(err, "failed to apply s3 snapshot retention policy")
				}
			}
			if err := e.addSnapshotData(*sf); err != nil {
				return errors.Wrap(err, "failed to save snapshot data to configmap")
			}
		}
	}

	return e.ReconcileSnapshotData(ctx)
}

type s3Config struct {
	Endpoint      string `json:"endpoint,omitempty"`
	EndpointCA    string `json:"endpointCA,omitempty"`
	SkipSSLVerify bool   `json:"skipSSLVerify,omitempty"`
	Bucket        string `json:"bucket,omitempty"`
	Region        string `json:"region,omitempty"`
	Folder        string `json:"folder,omitempty"`
	Insecure      bool   `json:"insecure,omitempty"`
}

type snapshotStatus string

const (
	successfulSnapshotStatus snapshotStatus = "successful"
	failedSnapshotStatus     snapshotStatus = "failed"
)

// snapshotFile represents a single snapshot and it's
// metadata.
type snapshotFile struct {
	Name string `json:"name"`
	// Location contains the full path of the snapshot. For
	// local paths, the location will be prefixed with "file://".
	Location   string         `json:"location,omitempty"`
	Metadata   string         `json:"metadata,omitempty"`
	Message    string         `json:"message,omitempty"`
	NodeName   string         `json:"nodeName,omitempty"`
	CreatedAt  *metav1.Time   `json:"createdAt,omitempty"`
	Size       int64          `json:"size,omitempty"`
	Status     snapshotStatus `json:"status,omitempty"`
	S3         *s3Config      `json:"s3Config,omitempty"`
	Compressed bool           `json:"compressed"`
}

// listLocalSnapshots provides a list of the currently stored
// snapshots on disk along with their relevant
// metadata.
func (e *ETCD) listLocalSnapshots() (map[string]snapshotFile, error) {
	snapshots := make(map[string]snapshotFile)
	snapshotDir, err := snapshotDir(e.config, true)
	if err != nil {
		return snapshots, errors.Wrap(err, "failed to get the snapshot dir")
	}

	dirEntries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return nil, err
	}

	nodeName := os.Getenv("NODE_NAME")

	for _, de := range dirEntries {
		file, err := de.Info()
		if err != nil {
			return nil, err
		}
		sf := snapshotFile{
			Name:     file.Name(),
			Location: "file://" + filepath.Join(snapshotDir, file.Name()),
			NodeName: nodeName,
			CreatedAt: &metav1.Time{
				Time: file.ModTime(),
			},
			Size:   file.Size(),
			Status: successfulSnapshotStatus,
		}
		sfKey := generateSnapshotConfigMapKey(sf)
		snapshots[sfKey] = sf
	}

	return snapshots, nil
}

// listS3Snapshots provides a list of currently stored
// snapshots in S3 along with their relevant
// metadata.
func (e *ETCD) listS3Snapshots(ctx context.Context) (map[string]snapshotFile, error) {
	snapshots := make(map[string]snapshotFile)

	if e.config.EtcdS3 {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		if err := e.initS3IfNil(ctx); err != nil {
			return nil, err
		}

		var loo minio.ListObjectsOptions
		if e.config.EtcdS3Folder != "" {
			loo = minio.ListObjectsOptions{
				Prefix:    e.config.EtcdS3Folder,
				Recursive: true,
			}
		}

		objects := e.s3.client.ListObjects(ctx, e.config.EtcdS3BucketName, loo)

		for obj := range objects {
			if obj.Err != nil {
				return nil, obj.Err
			}
			if obj.Size == 0 {
				continue
			}

			ca, err := time.Parse(time.RFC3339, obj.LastModified.Format(time.RFC3339))
			if err != nil {
				return nil, err
			}

			sf := snapshotFile{
				Name:     filepath.Base(obj.Key),
				NodeName: "s3",
				CreatedAt: &metav1.Time{
					Time: ca,
				},
				Size: obj.Size,
				S3: &s3Config{
					Endpoint:      e.config.EtcdS3Endpoint,
					EndpointCA:    e.config.EtcdS3EndpointCA,
					SkipSSLVerify: e.config.EtcdS3SkipSSLVerify,
					Bucket:        e.config.EtcdS3BucketName,
					Region:        e.config.EtcdS3Region,
					Folder:        e.config.EtcdS3Folder,
					Insecure:      e.config.EtcdS3Insecure,
				},
				Status: successfulSnapshotStatus,
			}
			sfKey := generateSnapshotConfigMapKey(sf)
			snapshots[sfKey] = sf
		}
	}
	return snapshots, nil
}

// initS3IfNil initializes the S3 client
// if it hasn't yet been initialized.
func (e *ETCD) initS3IfNil(ctx context.Context) error {
	if e.s3 == nil {
		s3, err := NewS3(ctx, e.config)
		if err != nil {
			return err
		}
		e.s3 = s3
	}

	return nil
}

// PruneSnapshots performs a retention run with the given
// retention duration and removes expired snapshots.
func (e *ETCD) PruneSnapshots(ctx context.Context) error {
	snapshotDir, err := snapshotDir(e.config, false)
	if err != nil {
		return errors.Wrap(err, "failed to get the snapshot dir")
	}
	if err := snapshotRetention(e.config.EtcdSnapshotRetention, e.config.EtcdSnapshotName, snapshotDir); err != nil {
		logrus.Errorf("Error applying snapshot retention policy: %v", err)
	}

	if e.config.EtcdS3 {
		if err := e.initS3IfNil(ctx); err != nil {
			logrus.Warnf("Unable to initialize S3 client during prune: %v", err)
		} else {
			if err := e.s3.snapshotRetention(ctx); err != nil {
				logrus.Errorf("Error applying S3 snapshot retention policy: %v", err)
			}
		}
	}

	return e.ReconcileSnapshotData(ctx)
}

// ListSnapshots is an exported wrapper method that wraps an
// unexported method of the same name.
func (e *ETCD) ListSnapshots(ctx context.Context) (map[string]snapshotFile, error) {
	if e.config.EtcdS3 {
		return e.listS3Snapshots(ctx)
	}
	return e.listLocalSnapshots()
}

// deleteSnapshots removes the given snapshots from
// either local storage or S3.
func (e *ETCD) DeleteSnapshots(ctx context.Context, snapshots []string) error {
	snapshotDir, err := snapshotDir(e.config, false)
	if err != nil {
		return errors.Wrap(err, "failed to get the snapshot dir")
	}

	if e.config.EtcdS3 {
		logrus.Info("Removing the given etcd snapshot(s) from S3")
		logrus.Debugf("Removing the given etcd snapshot(s) from S3: %v", snapshots)

		if e.initS3IfNil(ctx); err != nil {
			return err
		}

		objectsCh := make(chan minio.ObjectInfo)

		ctx, cancel := context.WithTimeout(ctx, e.config.EtcdS3Timeout)
		defer cancel()

		go func() {
			defer close(objectsCh)

			opts := minio.ListObjectsOptions{
				Recursive: true,
			}

			for obj := range e.s3.client.ListObjects(ctx, e.config.EtcdS3BucketName, opts) {
				if obj.Err != nil {
					logrus.Error(obj.Err)
					return
				}

				// iterate through the given snapshots and only
				// add them to the channel for remove if they're
				// actually found from the bucket listing.
				for _, snapshot := range snapshots {
					if snapshot == obj.Key {
						objectsCh <- obj
					}
				}
			}
		}()

		err = func() error {
			for {
				select {
				case <-ctx.Done():
					logrus.Errorf("Unable to delete snapshot: %v", ctx.Err())
					return e.ReconcileSnapshotData(ctx)
				case <-time.After(time.Millisecond * 100):
					continue
				case err, ok := <-e.s3.client.RemoveObjects(ctx, e.config.EtcdS3BucketName, objectsCh, minio.RemoveObjectsOptions{}):
					if err.Err != nil {
						logrus.Errorf("Unable to delete snapshot: %v", err.Err)
					}
					if !ok {
						return e.ReconcileSnapshotData(ctx)
					}
				}
			}
		}()
		if err != nil {
			return err
		}
	}

	logrus.Info("Removing the given locally stored etcd snapshot(s)")
	logrus.Debugf("Attempting to remove the given locally stored etcd snapshot(s): %v", snapshots)

	for _, s := range snapshots {
		// check if the given snapshot exists. If it does,
		// remove it, otherwise continue.
		sf := filepath.Join(snapshotDir, s)
		if _, err := os.Stat(sf); os.IsNotExist(err) {
			logrus.Infof("Snapshot %s, does not exist", s)
			continue
		}
		if err := os.Remove(sf); err != nil {
			return err
		}
		logrus.Debug("Removed snapshot ", s)
	}

	return e.ReconcileSnapshotData(ctx)
}

// AddSnapshotData adds the given snapshot file information to the snapshot configmap, using the existing extra metadata
// available at the time.
func (e *ETCD) addSnapshotData(sf snapshotFile) error {
	return retry.OnError(snapshotDataBackoff, func(err error) bool {
		return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err)
	}, func() error {
		// make sure the core.Factory is initialized. There can
		// be a race between this core code startup.
		for e.config.Runtime.Core == nil {
			runtime.Gosched()
		}
		snapshotConfigMap, getErr := e.config.Runtime.Core.Core().V1().ConfigMap().Get(metav1.NamespaceSystem, snapshotConfigMapName, metav1.GetOptions{})

		sfKey := generateSnapshotConfigMapKey(sf)
		marshalledSnapshotFile, err := json.Marshal(sf)
		if err != nil {
			return err
		}
		if apierrors.IsNotFound(getErr) {
			cm := v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotConfigMapName,
					Namespace: metav1.NamespaceSystem,
				},
				Data: map[string]string{sfKey: string(marshalledSnapshotFile)},
			}
			_, err := e.config.Runtime.Core.Core().V1().ConfigMap().Create(&cm)
			return err
		}

		if snapshotConfigMap.Data == nil {
			snapshotConfigMap.Data = make(map[string]string)
		}

		snapshotConfigMap.Data[sfKey] = string(marshalledSnapshotFile)

		_, err = e.config.Runtime.Core.Core().V1().ConfigMap().Update(snapshotConfigMap)
		return err
	})
}

func generateSnapshotConfigMapKey(sf snapshotFile) string {
	name := invalidKeyChars.ReplaceAllString(sf.Name, "_")
	if sf.NodeName == "s3" {
		return "s3-" + name
	}
	return "local-" + name
}

// ReconcileSnapshotData reconciles snapshot data in the snapshot ConfigMap.
// It will reconcile snapshot data from disk locally always, and if S3 is enabled, will attempt to list S3 snapshots
// and reconcile snapshots from S3. Notably,
func (e *ETCD) ReconcileSnapshotData(ctx context.Context) error {
	logrus.Infof("Reconciling etcd snapshot data in %s ConfigMap", snapshotConfigMapName)
	defer logrus.Infof("Reconciliation of snapshot data in %s ConfigMap complete", snapshotConfigMapName)
	return retry.OnError(retry.DefaultBackoff, func(err error) bool {
		return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err)
	}, func() error {
		// make sure the core.Factory is initialize. There can
		// be a race between this core code startup.
		for e.config.Runtime.Core == nil {
			runtime.Gosched()
		}

		logrus.Debug("core.Factory is initialized")

		snapshotConfigMap, getErr := e.config.Runtime.Core.Core().V1().ConfigMap().Get(metav1.NamespaceSystem, snapshotConfigMapName, metav1.GetOptions{})
		if apierrors.IsNotFound(getErr) {
			// Can't reconcile what doesn't exist.
			return errors.New("No snapshot configmap found")
		}

		logrus.Debugf("Attempting to reconcile etcd snapshot data for configmap generation %d", snapshotConfigMap.Generation)

		// if the snapshot config map data is nil, no need to reconcile.
		if snapshotConfigMap.Data == nil {
			return nil
		}

		snapshotFiles, err := e.listLocalSnapshots()
		if err != nil {
			return err
		}

		// s3ListSuccessful is set to true if we are successful at listing snapshots from S3 to eliminate accidental
		// clobbering of S3 snapshots in the configmap due to misconfigured S3 credentials/details
		s3ListSuccessful := false

		if e.config.EtcdS3 {
			if s3Snapshots, err := e.listS3Snapshots(ctx); err != nil {
				logrus.Errorf("error retrieving S3 snapshots for reconciliation: %v", err)
			} else {
				for k, v := range s3Snapshots {
					snapshotFiles[k] = v
				}
				s3ListSuccessful = true
			}
		}

		nodeName := os.Getenv("NODE_NAME")

		// deletedSnapshots is a map[string]string where key is the configmap key and the value is the marshalled snapshot file
		// it will be populated below with snapshots that are either from S3 or on the local node. Notably, deletedSnapshots will
		// not contain snapshots that are in the "failed" status
		deletedSnapshots := make(map[string]string)
		// failedSnapshots is a slice of unmarshaled snapshot files sourced from the configmap
		// These are stored unmarshaled so we can sort based on name.
		var failedSnapshots []snapshotFile
		var failedS3Snapshots []snapshotFile

		// remove entries for this node and s3 (if S3 is enabled) only
		for k, v := range snapshotConfigMap.Data {
			var sf snapshotFile
			if err := json.Unmarshal([]byte(v), &sf); err != nil {
				return err
			}
			if (sf.NodeName == nodeName || (sf.NodeName == "s3" && s3ListSuccessful)) && sf.Status != failedSnapshotStatus {
				// Only delete the snapshot if the snapshot was not failed
				// sf.Status != FailedSnapshotStatus is intentional, as it is possible we are reconciling snapshots stored from older versions that did not set status
				deletedSnapshots[generateSnapshotConfigMapKey(sf)] = v // store a copy of the snapshot
				delete(snapshotConfigMap.Data, k)
			} else if sf.Status == failedSnapshotStatus && sf.NodeName == nodeName && e.config.EtcdSnapshotRetention >= 1 {
				// Handle locally failed snapshots.
				failedSnapshots = append(failedSnapshots, sf)
				delete(snapshotConfigMap.Data, k)
			} else if sf.Status == failedSnapshotStatus && e.config.EtcdS3 && sf.NodeName == "s3" && strings.HasPrefix(sf.Name, e.config.EtcdSnapshotName+"-"+nodeName) && e.config.EtcdSnapshotRetention >= 1 {
				// If we're operating against S3, we can clean up failed S3 snapshots that failed on this node.
				failedS3Snapshots = append(failedS3Snapshots, sf)
				delete(snapshotConfigMap.Data, k)
			}
		}

		// Apply the failed snapshot retention policy to locally failed snapshots
		if len(failedSnapshots) > 0 && e.config.EtcdSnapshotRetention >= 1 {
			sort.Slice(failedSnapshots, func(i, j int) bool {
				return failedSnapshots[i].Name > failedSnapshots[j].Name
			})

			var keepCount int
			if e.config.EtcdSnapshotRetention >= len(failedSnapshots) {
				keepCount = len(failedSnapshots)
			} else {
				keepCount = e.config.EtcdSnapshotRetention
			}
			for _, dfs := range failedSnapshots[:keepCount] {
				sfKey := generateSnapshotConfigMapKey(dfs)
				marshalledSnapshot, err := json.Marshal(dfs)
				if err != nil {
					logrus.Errorf("unable to marshal snapshot to store in configmap %v", err)
				} else {
					snapshotConfigMap.Data[sfKey] = string(marshalledSnapshot)
				}
			}
		}

		// Apply the failed snapshot retention policy to the S3 snapshots
		if len(failedS3Snapshots) > 0 && e.config.EtcdSnapshotRetention >= 1 {
			sort.Slice(failedS3Snapshots, func(i, j int) bool {
				return failedS3Snapshots[i].Name > failedS3Snapshots[j].Name
			})

			var keepCount int
			if e.config.EtcdSnapshotRetention >= len(failedS3Snapshots) {
				keepCount = len(failedS3Snapshots)
			} else {
				keepCount = e.config.EtcdSnapshotRetention
			}
			for _, dfs := range failedS3Snapshots[:keepCount] {
				sfKey := generateSnapshotConfigMapKey(dfs)
				marshalledSnapshot, err := json.Marshal(dfs)
				if err != nil {
					logrus.Errorf("unable to marshal snapshot to store in configmap %v", err)
				} else {
					snapshotConfigMap.Data[sfKey] = string(marshalledSnapshot)
				}
			}
		}

		// save the local entries to the ConfigMap if they are still on disk or in S3.
		for _, snapshot := range snapshotFiles {
			var sf snapshotFile
			sfKey := generateSnapshotConfigMapKey(snapshot)
			if v, ok := deletedSnapshots[sfKey]; ok {
				// use the snapshot file we have from the existing configmap, and unmarshal it so we can manipulate it
				if err := json.Unmarshal([]byte(v), &sf); err != nil {
					logrus.Errorf("error unmarshaling snapshot file: %v", err)
					// use the snapshot with info we sourced from disk/S3 (will be missing metadata, but something is better than nothing)
					sf = snapshot
				}
			} else {
				sf = snapshot
			}

			sf.Status = successfulSnapshotStatus // if the snapshot is on disk or in S3, it was successful.

			marshalledSnapshot, err := json.Marshal(sf)
			if err != nil {
				logrus.Warnf("unable to marshal snapshot metadata %s to store in configmap, received error: %v", sf.Name, err)
			} else {
				snapshotConfigMap.Data[sfKey] = string(marshalledSnapshot)
			}
		}

		logrus.Debugf("Updating snapshot ConfigMap (%s) with %d entries", snapshotConfigMapName, len(snapshotConfigMap.Data))
		_, err = e.config.Runtime.Core.Core().V1().ConfigMap().Update(snapshotConfigMap)
		return err
	})
}

// setSnapshotFunction schedules snapshots at the configured interval.
func (e *ETCD) setSnapshotFunction(ctx context.Context) {
	skipJob := cron.SkipIfStillRunning(cronLogger)
	e.cron.AddJob(e.config.EtcdSnapshotCron, skipJob(cron.FuncJob(func() {
		// Add a small amount of jitter to the actual snapshot execution. On clusters with multiple servers,
		// having all the nodes take a snapshot at the exact same time can lead to excessive retry thrashing
		// when updating the snapshot list configmap.
		time.Sleep(time.Duration(rand.Float64() * float64(snapshotJitterMax)))
		if err := e.Snapshot(ctx); err != nil {
			logrus.Error(err)
		}
	})))
}

// snapshotRetention iterates through the snapshots and removes the oldest
// leaving the desired number of snapshots.
func snapshotRetention(retention int, snapshotPrefix string, snapshotDir string) error {
	if retention < 1 {
		return nil
	}

	logrus.Infof("Applying local snapshot retention policy: retention: %d, snapshotPrefix: %s, directory: %s", retention, snapshotPrefix, snapshotDir)

	var snapshotFiles []os.FileInfo
	if err := filepath.Walk(snapshotDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(info.Name(), snapshotPrefix) {
			snapshotFiles = append(snapshotFiles, info)
		}
		return nil
	}); err != nil {
		return err
	}
	if len(snapshotFiles) <= retention {
		return nil
	}
	sort.Slice(snapshotFiles, func(firstSnapshot, secondSnapshot int) bool {
		// it takes the name from the snapshot file ex: etcd-snapshot-example-{date}, makes the split using "-" to find the date, takes the date and sort by date
		firstSnapshotName, secondSnapshotName := strings.Split(snapshotFiles[firstSnapshot].Name(), "-"), strings.Split(snapshotFiles[secondSnapshot].Name(), "-")
		firstSnapshotDate, secondSnapshotDate := firstSnapshotName[len(firstSnapshotName)-1], secondSnapshotName[len(secondSnapshotName)-1]
		return firstSnapshotDate < secondSnapshotDate
	})

	delCount := len(snapshotFiles) - retention
	for _, df := range snapshotFiles[:delCount] {
		snapshotPath := filepath.Join(snapshotDir, df.Name())
		logrus.Infof("Removing local snapshot %s", snapshotPath)
		if err := os.Remove(snapshotPath); err != nil {
			return err
		}
	}

	return nil
}
