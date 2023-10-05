package etcd

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
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
	pruneStepSize          = 5
	compressedExtension    = ".zip"
	metadataDir            = ".metadata"
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
func (e *ETCD) compressSnapshot(snapshotDir, snapshotName, snapshotPath string, now time.Time) (string, error) {
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
	header.Modified = now

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
	var extraMetadata *v1.ConfigMap
	if e.config.Runtime.Core == nil {
		logrus.Debugf("Cannot retrieve extra metadata from %s ConfigMap: runtime core not ready", snapshotExtraMetadataConfigMapName)
	} else {
		logrus.Debugf("Attempting to retrieve extra metadata from %s ConfigMap", snapshotExtraMetadataConfigMapName)
		if snapshotExtraMetadataConfigMap, err := e.config.Runtime.Core.Core().V1().ConfigMap().Get(metav1.NamespaceSystem, snapshotExtraMetadataConfigMapName, metav1.GetOptions{}); err != nil {
			logrus.Debugf("Error encountered attempting to retrieve extra metadata from %s ConfigMap, error: %v", snapshotExtraMetadataConfigMapName, err)
		} else {
			logrus.Debugf("Setting extra metadata from %s ConfigMap", snapshotExtraMetadataConfigMapName)
			extraMetadata = snapshotExtraMetadataConfigMap
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
	now := time.Now().Round(time.Second)
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
			NodeName: nodeName,
			CreatedAt: &metav1.Time{
				Time: now,
			},
			Status:         failedSnapshotStatus,
			Message:        base64.StdEncoding.EncodeToString([]byte(err.Error())),
			Size:           0,
			Compressed:     e.config.EtcdSnapshotCompress,
			metadataSource: extraMetadata,
		}
		logrus.Errorf("Failed to take etcd snapshot: %v", err)
		if err := e.addSnapshotData(*sf); err != nil {
			return errors.Wrap(err, "failed to save local snapshot failure data to configmap")
		}
	}

	// If the snapshot attempt was successful, sf will be nil as we did not set it to store the error message.
	if sf == nil {
		if e.config.EtcdSnapshotCompress {
			zipPath, err := e.compressSnapshot(snapshotDir, snapshotName, snapshotPath, now)
			if err != nil {
				return errors.Wrap(err, "failed to compress snapshot")
			}
			if err := os.Remove(snapshotPath); err != nil {
				return errors.Wrap(err, "failed to remove uncompressed snapshot")
			}
			snapshotPath = zipPath
			logrus.Info("Compressed snapshot: " + snapshotPath)
		}

		f, err := os.Stat(snapshotPath)
		if err != nil {
			return errors.Wrap(err, "unable to retrieve snapshot information from local snapshot")
		}
		sf = &snapshotFile{
			Name:     f.Name(),
			Location: "file://" + snapshotPath,
			NodeName: nodeName,
			CreatedAt: &metav1.Time{
				Time: now,
			},
			Status:         successfulSnapshotStatus,
			Size:           f.Size(),
			Compressed:     e.config.EtcdSnapshotCompress,
			metadataSource: extraMetadata,
		}

		if err := saveSnapshotMetadata(snapshotPath, extraMetadata); err != nil {
			return errors.Wrap(err, "failed to save local snapshot metadata")
		}

		if err := e.addSnapshotData(*sf); err != nil {
			return errors.Wrap(err, "failed to save local snapshot data to configmap")
		}

		if err := snapshotRetention(e.config.EtcdSnapshotRetention, e.config.EtcdSnapshotName, snapshotDir); err != nil {
			return errors.Wrap(err, "failed to apply local snapshot retention policy")
		}

		if e.config.EtcdS3 {
			if err := e.initS3IfNil(ctx); err != nil {
				logrus.Warnf("Unable to initialize S3 client: %v", err)
				sf = &snapshotFile{
					Name:     filepath.Base(snapshotPath),
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
					metadataSource: extraMetadata,
				}
			} else {
				logrus.Infof("Saving etcd snapshot %s to S3", snapshotName)
				// upload will return a snapshotFile even on error - if there was an
				// error, it will be reflected in the status and message.
				sf, err = e.s3.upload(ctx, snapshotPath, extraMetadata, now)
				if err != nil {
					logrus.Errorf("Error received during snapshot upload to S3: %s", err)
				} else {
					logrus.Infof("S3 upload complete for %s", snapshotName)
				}
			}
			if err := e.addSnapshotData(*sf); err != nil {
				return errors.Wrap(err, "failed to save snapshot data to configmap")
			}
			if err := e.s3.snapshotRetention(ctx); err != nil {
				logrus.Errorf("Failed to apply s3 snapshot retention policy: %v", err)
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

	metadataSource *v1.ConfigMap `json:"-"`
}

// listLocalSnapshots provides a list of the currently stored
// snapshots on disk along with their relevant
// metadata.
func (e *ETCD) listLocalSnapshots() (map[string]snapshotFile, error) {
	nodeName := os.Getenv("NODE_NAME")
	snapshots := make(map[string]snapshotFile)
	snapshotDir, err := snapshotDir(e.config, true)
	if err != nil {
		return snapshots, errors.Wrap(err, "failed to get the snapshot dir")
	}

	if err := filepath.Walk(snapshotDir, func(path string, file os.FileInfo, err error) error {
		if file.IsDir() || err != nil {
			return err
		}

		basename, compressed := strings.CutSuffix(file.Name(), compressedExtension)
		ts, err := strconv.ParseInt(basename[strings.LastIndexByte(basename, '-')+1:], 10, 64)
		if err != nil {
			ts = file.ModTime().Unix()
		}

		// try to read metadata from disk; don't warn if it is missing as it will not exist
		// for snapshot files from old releases or if there was no metadata provided.
		var metadata string
		metadataFile := filepath.Join(filepath.Dir(path), "..", metadataDir, file.Name())
		if m, err := os.ReadFile(metadataFile); err == nil {
			logrus.Debugf("Loading snapshot metadata from %s", metadataFile)
			metadata = base64.StdEncoding.EncodeToString(m)
		}

		sf := snapshotFile{
			Name:     file.Name(),
			Location: "file://" + filepath.Join(snapshotDir, file.Name()),
			NodeName: nodeName,
			Metadata: metadata,
			CreatedAt: &metav1.Time{
				Time: time.Unix(ts, 0),
			},
			Size:       file.Size(),
			Status:     successfulSnapshotStatus,
			Compressed: compressed,
		}
		sfKey := generateSnapshotConfigMapKey(sf)
		snapshots[sfKey] = sf
		return nil
	}); err != nil {
		return nil, err
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
			logrus.Warnf("Unable to initialize S3 client: %v", err)
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
	snapshotFiles := map[string]snapshotFile{}
	if e.config.EtcdS3 {
		if err := e.initS3IfNil(ctx); err != nil {
			logrus.Warnf("Unable to initialize S3 client: %v", err)
			return nil, err
		}
		sfs, err := e.s3.listSnapshots(ctx)
		if err != nil {
			return nil, err
		}
		snapshotFiles = sfs
	}

	sfs, err := e.listLocalSnapshots()
	if err != nil {
		return nil, err
	}
	for k, sf := range sfs {
		snapshotFiles[k] = sf
	}

	return snapshotFiles, err
}

// DeleteSnapshots removes the given snapshots from local storage and S3.
func (e *ETCD) DeleteSnapshots(ctx context.Context, snapshots []string) error {
	snapshotDir, err := snapshotDir(e.config, false)
	if err != nil {
		return errors.Wrap(err, "failed to get the snapshot dir")
	}
	if e.config.EtcdS3 {
		if err := e.initS3IfNil(ctx); err != nil {
			return err
		}
	}

	for _, s := range snapshots {
		if err := e.deleteSnapshot(filepath.Join(snapshotDir, s)); err != nil {
			if isNotExist(err) {
				logrus.Infof("Snapshot %s not found locally", s)
			} else {
				logrus.Errorf("Failed to delete local snapshot %s: %v", s, err)
			}
		} else {
			logrus.Infof("Snapshot %s deleted locally", s)
		}

		if e.config.EtcdS3 {
			if err := e.s3.deleteSnapshot(s); err != nil {
				if isNotExist(err) {
					logrus.Infof("Snapshot %s not found in S3", s)
				} else {
					logrus.Errorf("Failed to delete S3 snapshot %s: %v", s, err)
				}
			} else {
				logrus.Infof("Snapshot %s deleted from S3", s)
			}
		}
	}

	return e.ReconcileSnapshotData(ctx)
}

func (e *ETCD) deleteSnapshot(snapshotPath string) error {
	dir := filepath.Join(filepath.Dir(snapshotPath), "..", metadataDir)
	filename := filepath.Base(snapshotPath)
	metadataPath := filepath.Join(dir, filename)

	err := os.Remove(snapshotPath)
	if err == nil || os.IsNotExist(err) {
		if merr := os.Remove(metadataPath); err != nil && !isNotExist(err) {
			err = merr
		}
	}

	return err
}

func marshalSnapshotFile(sf snapshotFile) ([]byte, error) {
	if sf.metadataSource != nil {
		if m, err := json.Marshal(sf.metadataSource.Data); err != nil {
			logrus.Debugf("Error attempting to marshal extra metadata contained in %s ConfigMap, error: %v", snapshotExtraMetadataConfigMapName, err)
		} else {
			sf.Metadata = base64.StdEncoding.EncodeToString(m)
		}
	}
	return json.Marshal(sf)
}

// AddSnapshotData adds the given snapshot file information to the snapshot configmap, using the existing extra metadata
// available at the time.
func (e *ETCD) addSnapshotData(sf snapshotFile) error {
	// make sure the core.Factory is initialized. There can
	// be a race between this core code startup.
	for e.config.Runtime.Core == nil {
		runtime.Gosched()
	}

	sfKey := generateSnapshotConfigMapKey(sf)
	marshalledSnapshotFile, err := marshalSnapshotFile(sf)
	if err != nil {
		return err
	}

	pruneCount := pruneStepSize
	var lastErr error
	return retry.OnError(snapshotDataBackoff, func(err error) bool {
		return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) || isTooLargeError(err)
	}, func() error {
		snapshotConfigMap, getErr := e.config.Runtime.Core.Core().V1().ConfigMap().Get(metav1.NamespaceSystem, snapshotConfigMapName, metav1.GetOptions{})

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

		// If the configmap update was rejected due to size, drop the oldest entries from the map.
		// We will continue to remove an increasing number of old snapshots from the map until the request succeeds,
		// or the number we would attempt to remove exceeds the number stored.
		if isTooLargeError(lastErr) {
			logrus.Warnf("Snapshot configmap is too large, attempting to elide %d oldest snapshots from list", pruneCount)
			if err := pruneConfigMap(snapshotConfigMap, pruneCount); err != nil {
				return err
			}
			pruneCount += pruneStepSize
		}

		snapshotConfigMap.Data[sfKey] = string(marshalledSnapshotFile)

		_, lastErr = e.config.Runtime.Core.Core().V1().ConfigMap().Update(snapshotConfigMap)
		return lastErr
	})
}

func generateSnapshotConfigMapKey(sf snapshotFile) string {
	name := invalidKeyChars.ReplaceAllString(sf.Name, "_")
	if sf.NodeName == "s3" {
		return "s3-" + name
	}
	return "local-" + name
}

// pruneConfigMap drops the oldest entries from the configMap.
// Note that the actual snapshot files are not removed, just the entries that track them in the configmap.
func pruneConfigMap(snapshotConfigMap *v1.ConfigMap, pruneCount int) error {
	if pruneCount > len(snapshotConfigMap.Data) {
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

// ReconcileSnapshotData reconciles snapshot data in the snapshot ConfigMap.
// It will reconcile snapshot data from disk locally always, and if S3 is enabled, will attempt to list S3 snapshots
// and reconcile snapshots from S3.
func (e *ETCD) ReconcileSnapshotData(ctx context.Context) error {
	// make sure the core.Factory is initialized. There can
	// be a race between this core code startup.
	for e.config.Runtime.Core == nil {
		runtime.Gosched()
	}

	logrus.Infof("Reconciling etcd snapshot data in %s ConfigMap", snapshotConfigMapName)
	defer logrus.Infof("Reconciliation of snapshot data in %s ConfigMap complete", snapshotConfigMapName)

	pruneCount := pruneStepSize
	var lastErr error
	return retry.OnError(retry.DefaultBackoff, func(err error) bool {
		return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err) || isTooLargeError(err)
	}, func() error {
		snapshotConfigMap, getErr := e.config.Runtime.Core.Core().V1().ConfigMap().Get(metav1.NamespaceSystem, snapshotConfigMapName, metav1.GetOptions{})
		if apierrors.IsNotFound(getErr) {
			cm := &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotConfigMapName,
					Namespace: metav1.NamespaceSystem,
				},
			}
			cm, err := e.config.Runtime.Core.Core().V1().ConfigMap().Create(cm)
			if err != nil {
				return err
			}
			snapshotConfigMap = cm
		}

		logrus.Debugf("Attempting to reconcile etcd snapshot data for configmap generation %d", snapshotConfigMap.Generation)
		if snapshotConfigMap.Data == nil {
			snapshotConfigMap.Data = map[string]string{}
		}

		snapshotFiles, err := e.listLocalSnapshots()
		if err != nil {
			return err
		}

		// s3ListSuccessful is set to true if we are successful at listing snapshots from S3 to eliminate accidental
		// clobbering of S3 snapshots in the configmap due to misconfigured S3 credentials/details
		var s3ListSuccessful bool

		if e.config.EtcdS3 {
			if err := e.initS3IfNil(ctx); err != nil {
				logrus.Warnf("Unable to initialize S3 client: %v", err)
				return err
			}

			if s3Snapshots, err := e.s3.listSnapshots(ctx); err != nil {
				logrus.Errorf("Error retrieving S3 snapshots for reconciliation: %v", err)
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
			// sort newest-first so we can record only the retention count
			sort.Slice(failedSnapshots, func(i, j int) bool {
				return failedSnapshots[j].CreatedAt.Before(failedSnapshots[i].CreatedAt)
			})

			for _, dfs := range failedSnapshots[:e.config.EtcdSnapshotRetention] {
				sfKey := generateSnapshotConfigMapKey(dfs)
				marshalledSnapshot, err := marshalSnapshotFile(dfs)
				if err != nil {
					logrus.Errorf("Failed to marshal snapshot to store in configmap %v", err)
				} else {
					snapshotConfigMap.Data[sfKey] = string(marshalledSnapshot)
				}
			}
		}

		// Apply the failed snapshot retention policy to the S3 snapshots
		if len(failedS3Snapshots) > 0 && e.config.EtcdSnapshotRetention >= 1 {
			// sort newest-first so we can record only the retention count
			sort.Slice(failedS3Snapshots, func(i, j int) bool {
				return failedS3Snapshots[j].CreatedAt.Before(failedS3Snapshots[i].CreatedAt)
			})

			for _, dfs := range failedS3Snapshots[:e.config.EtcdSnapshotRetention] {
				sfKey := generateSnapshotConfigMapKey(dfs)
				marshalledSnapshot, err := marshalSnapshotFile(dfs)
				if err != nil {
					logrus.Errorf("Failed to marshal snapshot to store in configmap %v", err)
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
					logrus.Errorf("Error unmarshaling snapshot file: %v", err)
					// use the snapshot with info we sourced from disk/S3 (will be missing metadata, but something is better than nothing)
					sf = snapshot
				}
			} else {
				sf = snapshot
			}

			sf.Status = successfulSnapshotStatus // if the snapshot is on disk or in S3, it was successful.
			marshalledSnapshot, err := marshalSnapshotFile(sf)
			if err != nil {
				logrus.Warnf("Failed to marshal snapshot metadata %s to store in configmap, received error: %v", sf.Name, err)
			} else {
				snapshotConfigMap.Data[sfKey] = string(marshalledSnapshot)
			}
		}

		// If the configmap update was rejected due to size, drop the oldest entries from the map.
		// We will continue to remove an increasing number of old snapshots from the map until the request succeeds,
		// or the number we would attempt to remove exceeds the number stored.
		if isTooLargeError(lastErr) {
			logrus.Warnf("Snapshot configmap is too large, attempting to elide %d oldest snapshots from list", pruneCount)
			if err := pruneConfigMap(snapshotConfigMap, pruneCount); err != nil {
				return err
			}
			pruneCount += pruneStepSize
		}

		logrus.Debugf("Updating snapshot ConfigMap (%s) with %d entries", snapshotConfigMapName, len(snapshotConfigMap.Data))
		_, lastErr = e.config.Runtime.Core.Core().V1().ConfigMap().Update(snapshotConfigMap)
		return lastErr
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
			logrus.Errorf("Failed to take scheduled snapshot: %v", err)
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

	var snapshotFiles []snapshotFile
	if err := filepath.Walk(snapshotDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || err != nil {
			return err
		}
		if strings.HasPrefix(info.Name(), snapshotPrefix) {
			basename, compressed := strings.CutSuffix(info.Name(), compressedExtension)
			ts, err := strconv.ParseInt(basename[strings.LastIndexByte(basename, '-')+1:], 10, 64)
			if err != nil {
				ts = info.ModTime().Unix()
			}
			snapshotFiles = append(snapshotFiles, snapshotFile{Name: info.Name(), CreatedAt: &metav1.Time{Time: time.Unix(ts, 0)}, Compressed: compressed})
		}
		return nil
	}); err != nil {
		return err
	}
	if len(snapshotFiles) <= retention {
		return nil
	}

	// sort newest-first so we can prune entries past the retention count
	sort.Slice(snapshotFiles, func(i, j int) bool {
		return snapshotFiles[j].CreatedAt.Before(snapshotFiles[i].CreatedAt)
	})

	for _, df := range snapshotFiles[retention:] {
		snapshotPath := filepath.Join(snapshotDir, df.Name)
		metadataPath := filepath.Join(snapshotDir, "..", metadataDir, df.Name)
		logrus.Infof("Removing local snapshot %s", snapshotPath)
		if err := os.Remove(snapshotPath); err != nil {
			return err
		}
		if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func isTooLargeError(err error) bool {
	// There are no helpers for unpacking field validation errors, so we just check for "Too long" in the error string.
	return apierrors.IsRequestEntityTooLargeError(err) || (apierrors.IsInvalid(err) && strings.Contains(err.Error(), "Too long"))
}

func isNotExist(err error) bool {
	if resp := minio.ToErrorResponse(err); resp.StatusCode == http.StatusNotFound || os.IsNotExist(err) {
		return true
	}
	return false
}

// saveSnapshotMetadata writes extra metadata to disk.
// The upload is silently skipped if no extra metadata is provided.
func saveSnapshotMetadata(snapshotPath string, extraMetadata *v1.ConfigMap) error {
	if extraMetadata == nil || len(extraMetadata.Data) == 0 {
		return nil
	}

	dir := filepath.Join(filepath.Dir(snapshotPath), "..", metadataDir)
	filename := filepath.Base(snapshotPath)
	metadataPath := filepath.Join(dir, filename)
	logrus.Infof("Saving snapshot metadata to %s", metadataPath)
	m, err := json.Marshal(extraMetadata.Data)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(metadataPath, m, 0700)
}
