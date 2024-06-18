package etcd

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
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

	k3s "github.com/k3s-io/k3s/pkg/apis/k3s.cattle.io/v1"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/etcdutl/v3/snapshot"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"
)

const (
	compressedExtension = ".zip"
	metadataDir         = ".metadata"
	errorTTL            = 24 * time.Hour
)

var (
	snapshotExtraMetadataConfigMapName = version.Program + "-etcd-snapshot-extra-metadata"
	labelStorageNode                   = "etcd." + version.Program + ".cattle.io/snapshot-storage-node"
	annotationLocalReconciled          = "etcd." + version.Program + ".cattle.io/local-snapshots-timestamp"
	annotationS3Reconciled             = "etcd." + version.Program + ".cattle.io/s3-snapshots-timestamp"
	annotationTokenHash                = "etcd." + version.Program + ".cattle.io/snapshot-token-hash"

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
// Only the default snapshot directory will be created; user-specified non-default
// snapshot directories must already exist.
func snapshotDir(config *config.Control, create bool) (string, error) {
	defaultSnapshotDir := filepath.Join(config.DataDir, "db", "snapshots")
	snapshotDir := config.EtcdSnapshotDir

	if snapshotDir == "" {
		snapshotDir = defaultSnapshotDir
	}

	// Disable creation if not using the default snapshot dir.
	// Non-default snapshot dirs must be created by the user.
	if snapshotDir != defaultSnapshotDir {
		create = false
	}

	s, err := os.Stat(snapshotDir)
	if err != nil {
		if os.IsNotExist(err) && create {
			if err := os.MkdirAll(snapshotDir, 0700); err != nil {
				return "", err
			}
			return snapshotDir, nil
		}
		return "", err
	}

	if !s.IsDir() {
		return "", fmt.Errorf("%s is not a directory", snapshotDir)
	}

	return snapshotDir, nil
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
			os.Remove(decompressed.Name())
			return "", err
		}
	}

	return decompressed.Name(), nil
}

// Snapshot attempts to save a new snapshot to the configured directory, and then clean up any old and failed
// snapshots in excess of the retention limits. Note that one snapshot request may result in creation and pruning
// of multiple snapshots, if S3 is enabled.
// Note that the prune step is generally disabled when snapshotting from the CLI, as there is a separate
// subcommand for prune that can be run manually if the user wants to remove old snapshots.
// Returns metadata about the new and pruned snapshots.
func (e *ETCD) Snapshot(ctx context.Context) (*managed.SnapshotResult, error) {
	if !e.snapshotMu.TryLock() {
		return nil, errors.New("snapshot save already in progress")
	}
	defer e.snapshotMu.Unlock()
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
	status, err := e.client.Status(ctx, endpoints[0])
	if err != nil {
		return nil, errors.Wrap(err, "failed to check etcd status for snapshot")
	}

	if status.IsLearner {
		logrus.Warnf("Unable to take snapshot: not supported for learner")
		return nil, nil
	}

	snapshotDir, err := snapshotDir(e.config, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get etcd-snapshot-dir")
	}

	cfg, err := getClientConfig(ctx, e.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get config for etcd snapshot")
	}

	tokenHash, err := util.GetTokenHash(e.config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get server token hash for etcd snapshot")
	}

	nodeName := os.Getenv("NODE_NAME")
	now := time.Now().Round(time.Second)
	snapshotName := fmt.Sprintf("%s-%s-%d", e.config.EtcdSnapshotName, nodeName, now.Unix())
	snapshotPath := filepath.Join(snapshotDir, snapshotName)
	logrus.Infof("Saving etcd snapshot to %s", snapshotPath)

	var sf *snapshotFile

	if err := snapshot.NewV3(e.client.GetLogger()).Save(ctx, *cfg, snapshotPath); err != nil {
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
			metadataSource: extraMetadata,
		}
		logrus.Errorf("Failed to take etcd snapshot: %v", err)
		if err := e.addSnapshotData(*sf); err != nil {
			return nil, errors.Wrap(err, "failed to sync ETCDSnapshotFile")
		}
	}

	res := &managed.SnapshotResult{}
	// If the snapshot attempt was successful, sf will be nil as we did not set it to store the error message.
	if sf == nil {
		if e.config.EtcdSnapshotCompress {
			zipPath, err := e.compressSnapshot(snapshotDir, snapshotName, snapshotPath, now)

			// ensure that the unncompressed snapshot is cleaned up even if compression fails
			if err := os.Remove(snapshotPath); err != nil && !os.IsNotExist(err) {
				logrus.Warnf("Failed to remove uncompress snapshot file: %v", err)
			}

			if err != nil {
				return nil, errors.Wrap(err, "failed to compress snapshot")
			}
			snapshotPath = zipPath
			logrus.Info("Compressed snapshot: " + snapshotPath)
		}

		f, err := os.Stat(snapshotPath)
		if err != nil {
			return nil, errors.Wrap(err, "unable to retrieve snapshot information from local snapshot")
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
			tokenHash:      tokenHash,
		}
		res.Created = append(res.Created, sf.Name)

		// Failing to save snapshot metadata is not fatal, the snapshot can still be used without it.
		if err := saveSnapshotMetadata(snapshotPath, extraMetadata); err != nil {
			logrus.Warnf("Failed to save local snapshot metadata: %v", err)
		}

		// If this fails, just log an error - the snapshot file will remain on disk
		// and will be recorded next time the snapshot list is reconciled.
		if err := e.addSnapshotData(*sf); err != nil {
			logrus.Warnf("Failed to sync ETCDSnapshotFile: %v", err)
		}

		// Snapshot retention may prune some files before returning an error. Failing to prune is not fatal.
		deleted, err := snapshotRetention(e.config.EtcdSnapshotRetention, e.config.EtcdSnapshotName, snapshotDir)
		if err != nil {
			logrus.Warnf("Failed to apply local snapshot retention policy: %v", err)
		}
		res.Deleted = append(res.Deleted, deleted...)

		if e.config.EtcdS3 {
			if err := e.initS3IfNil(ctx); err != nil {
				logrus.Warnf("Unable to initialize S3 client: %v", err)
				sf = &snapshotFile{
					Name:     f.Name(),
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
					res.Created = append(res.Created, sf.Name)
					logrus.Infof("S3 upload complete for %s", snapshotName)
				}
				// Attempt to apply retention even if the upload failed; failure may be due to bucket
				// being full or some other condition that retention policy would resolve.
				// Snapshot retention may prune some files before returning an error. Failing to prune is not fatal.
				deleted, err := e.s3.snapshotRetention(ctx)
				res.Deleted = append(res.Deleted, deleted...)
				if err != nil {
					logrus.Warnf("Failed to apply s3 snapshot retention policy: %v", err)
				}
			}
			// sf is either s3 snapshot metadata, or s3 init/upload failure record.
			// If this fails, just log an error - the snapshot file will remain on s3
			// and will be recorded next time the snapshot list is reconciled.
			if err := e.addSnapshotData(*sf); err != nil {
				logrus.Warnf("Failed to sync ETCDSnapshotFile: %v", err)
			}
		}
	}

	return res, e.ReconcileSnapshotData(ctx)
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

	// these fields are used for the internal representation of the snapshot
	// to populate other fields before serialization to the legacy configmap.
	metadataSource *v1.ConfigMap `json:"-"`
	nodeSource     string        `json:"-"`
	tokenHash      string        `json:"-"`
}

// listLocalSnapshots provides a list of the currently stored
// snapshots on disk along with their relevant
// metadata.
func (e *ETCD) listLocalSnapshots() (map[string]snapshotFile, error) {
	nodeName := os.Getenv("NODE_NAME")
	snapshots := make(map[string]snapshotFile)
	snapshotDir, err := snapshotDir(e.config, true)
	if err != nil {
		return snapshots, errors.Wrap(err, "failed to get etcd-snapshot-dir")
	}

	if err := filepath.Walk(snapshotDir, func(path string, file os.FileInfo, err error) error {
		if err != nil || file.IsDir() {
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
	if e.config.EtcdS3 && e.s3 == nil {
		s3, err := NewS3(ctx, e.config)
		if err != nil {
			return err
		}
		e.s3 = s3
	}

	return nil
}

// PruneSnapshots deleted old snapshots in excess of the configured retention count.
// Returns a list of deleted snapshots. Note that snapshots may be deleted
// with a non-nil error return.
func (e *ETCD) PruneSnapshots(ctx context.Context) (*managed.SnapshotResult, error) {
	snapshotDir, err := snapshotDir(e.config, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get etcd-snapshot-dir")
	}

	res := &managed.SnapshotResult{}
	// Note that snapshotRetention functions may return a list of deleted files, as well as
	// an error, if some snapshots are deleted before the error is encountered.
	res.Deleted, err = snapshotRetention(e.config.EtcdSnapshotRetention, e.config.EtcdSnapshotName, snapshotDir)
	if err != nil {
		logrus.Errorf("Error applying snapshot retention policy: %v", err)
	}

	if e.config.EtcdS3 {
		if err := e.initS3IfNil(ctx); err != nil {
			logrus.Warnf("Unable to initialize S3 client: %v", err)
		} else {
			deleted, err := e.s3.snapshotRetention(ctx)
			if err != nil {
				logrus.Errorf("Error applying S3 snapshot retention policy: %v", err)
			}
			res.Deleted = append(res.Deleted, deleted...)
		}
	}
	return res, e.ReconcileSnapshotData(ctx)
}

// ListSnapshots returns a list of snapshots. Local snapshots are always listed,
// s3 snapshots are listed if s3 is enabled.
// Snapshots are listed locally, not listed from the apiserver, so results
// are guaranteed to be in sync with what is on disk.
func (e *ETCD) ListSnapshots(ctx context.Context) (*k3s.ETCDSnapshotFileList, error) {
	snapshotFiles := &k3s.ETCDSnapshotFileList{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "List"},
	}
	if e.config.EtcdS3 {
		if err := e.initS3IfNil(ctx); err != nil {
			logrus.Warnf("Unable to initialize S3 client: %v", err)
			return nil, err
		}
		sfs, err := e.s3.listSnapshots(ctx)
		if err != nil {
			return nil, err
		}
		for k, sf := range sfs {
			esf := k3s.NewETCDSnapshotFile("", k, k3s.ETCDSnapshotFile{})
			sf.toETCDSnapshotFile(esf)
			snapshotFiles.Items = append(snapshotFiles.Items, *esf)
		}
	}

	sfs, err := e.listLocalSnapshots()
	if err != nil {
		return nil, err
	}
	for k, sf := range sfs {
		esf := k3s.NewETCDSnapshotFile("", k, k3s.ETCDSnapshotFile{})
		sf.toETCDSnapshotFile(esf)
		snapshotFiles.Items = append(snapshotFiles.Items, *esf)
	}

	return snapshotFiles, nil
}

// DeleteSnapshots removes the given snapshots from local storage and S3.
// Returns a list of deleted snapshots. Note that snapshots may be deleted
// with a non-nil error return.
func (e *ETCD) DeleteSnapshots(ctx context.Context, snapshots []string) (*managed.SnapshotResult, error) {
	snapshotDir, err := snapshotDir(e.config, false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get etcd-snapshot-dir")
	}
	if e.config.EtcdS3 {
		if err := e.initS3IfNil(ctx); err != nil {
			logrus.Warnf("Unable to initialize S3 client: %v", err)
			return nil, err
		}
	}

	res := &managed.SnapshotResult{}
	for _, s := range snapshots {
		if err := e.deleteSnapshot(filepath.Join(snapshotDir, s)); err != nil {
			if isNotExist(err) {
				logrus.Infof("Snapshot %s not found locally", s)
			} else {
				logrus.Errorf("Failed to delete local snapshot %s: %v", s, err)
			}
		} else {
			res.Deleted = append(res.Deleted, s)
			logrus.Infof("Snapshot %s deleted locally", s)
		}

		if e.config.EtcdS3 {
			if err := e.s3.deleteSnapshot(ctx, s); err != nil {
				if isNotExist(err) {
					logrus.Infof("Snapshot %s not found in S3", s)
				} else {
					logrus.Errorf("Failed to delete S3 snapshot %s: %v", s, err)
				}
			} else {
				res.Deleted = append(res.Deleted, s)
				logrus.Infof("Snapshot %s deleted from S3", s)
			}
		}
	}

	return res, e.ReconcileSnapshotData(ctx)
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

// addSnapshotData syncs an internal snapshotFile representation to an ETCDSnapshotFile resource
// of the same name. Resources will be created or updated as necessary.
func (e *ETCD) addSnapshotData(sf snapshotFile) error {
	// make sure the K3s factory is initialized.
	for e.config.Runtime.K3s == nil {
		runtime.Gosched()
	}

	snapshots := e.config.Runtime.K3s.K3s().V1().ETCDSnapshotFile()
	esfName := generateSnapshotName(sf)

	var esf *k3s.ETCDSnapshotFile
	return retry.OnError(snapshotDataBackoff, func(err error) bool {
		return apierrors.IsConflict(err) || apierrors.IsAlreadyExists(err)
	}, func() (err error) {
		// Get current object or create new one
		esf, err = snapshots.Get(esfName, metav1.GetOptions{})
		if err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
			esf = &k3s.ETCDSnapshotFile{
				ObjectMeta: metav1.ObjectMeta{
					Name: esfName,
				},
			}
		}

		// mutate object
		existing := esf.DeepCopyObject()
		sf.toETCDSnapshotFile(esf)

		// create or update as necessary
		if esf.CreationTimestamp.IsZero() {
			var created *k3s.ETCDSnapshotFile
			created, err = snapshots.Create(esf)
			if err == nil {
				// Only emit an event for the snapshot when creating the resource
				e.emitEvent(created)
			}
		} else if !equality.Semantic.DeepEqual(existing, esf) {
			_, err = snapshots.Update(esf)
		}
		return err
	})
}

// generateSnapshotConfigMapKey generates a derived name for the snapshot that is safe for use
// as a configmap key.
func generateSnapshotConfigMapKey(sf snapshotFile) string {
	name := invalidKeyChars.ReplaceAllString(sf.Name, "_")
	if sf.NodeName == "s3" {
		return "s3-" + name
	}
	return "local-" + name
}

// generateSnapshotName generates a derived name for the snapshot that is safe for use
// as a resource name.
func generateSnapshotName(sf snapshotFile) string {
	name := strings.ToLower(sf.Name)
	nodename := sf.nodeSource
	if nodename == "" {
		nodename = sf.NodeName
	}
	// Include a digest of the hostname and location to ensure unique resource
	// names. Snapshots should already include the hostname, but this ensures we
	// don't accidentally hide records if a snapshot with the same name somehow
	// exists on multiple nodes.
	digest := sha256.Sum256([]byte(nodename + sf.Location))
	// If the lowercase filename isn't usable as a resource name, and short enough that we can include a prefix and suffix,
	// generate a safe name derived from the hostname and timestamp.
	if errs := validation.IsDNS1123Subdomain(name); len(errs) != 0 || len(name)+13 > validation.DNS1123SubdomainMaxLength {
		nodename, _, _ := strings.Cut(nodename, ".")
		name = fmt.Sprintf("etcd-snapshot-%s-%d", nodename, sf.CreatedAt.Unix())
		if sf.Compressed {
			name += compressedExtension
		}
	}
	if sf.NodeName == "s3" {
		return "s3-" + name + "-" + hex.EncodeToString(digest[0:])[0:6]
	}
	return "local-" + name + "-" + hex.EncodeToString(digest[0:])[0:6]
}

// generateETCDSnapshotFileConfigMapKey generates a key that the corresponding
// snapshotFile would be stored under in the legacy configmap
func generateETCDSnapshotFileConfigMapKey(esf k3s.ETCDSnapshotFile) string {
	name := invalidKeyChars.ReplaceAllString(esf.Spec.SnapshotName, "_")
	if esf.Spec.S3 != nil {
		return "s3-" + name
	}
	return "local-" + name
}

func (e *ETCD) emitEvent(esf *k3s.ETCDSnapshotFile) {
	switch {
	case e.config.Runtime.Event == nil:
	case !esf.DeletionTimestamp.IsZero():
		e.config.Runtime.Event.Eventf(esf, v1.EventTypeNormal, "ETCDSnapshotDeleted", "Snapshot %s deleted", esf.Spec.SnapshotName)
	case esf.Status.Error != nil:
		message := fmt.Sprintf("Failed to save snapshot %s on %s", esf.Spec.SnapshotName, esf.Spec.NodeName)
		if esf.Status.Error.Message != nil {
			message += ": " + *esf.Status.Error.Message
		}
		e.config.Runtime.Event.Event(esf, v1.EventTypeWarning, "ETCDSnapshotFailed", message)
	default:
		e.config.Runtime.Event.Eventf(esf, v1.EventTypeNormal, "ETCDSnapshotCreated", "Snapshot %s saved on %s", esf.Spec.SnapshotName, esf.Spec.NodeName)
	}
}

// ReconcileSnapshotData reconciles snapshot data in the ETCDSnapshotFile resources.
// It will reconcile snapshot data from disk locally always, and if S3 is enabled, will attempt to list S3 snapshots
// and reconcile snapshots from S3.
func (e *ETCD) ReconcileSnapshotData(ctx context.Context) error {
	// make sure the core.Factory is initialized. There can
	// be a race between this core code startup.
	for e.config.Runtime.Core == nil {
		runtime.Gosched()
	}

	logrus.Infof("Reconciling ETCDSnapshotFile resources")
	defer logrus.Infof("Reconciliation of ETCDSnapshotFile resources complete")

	// Get snapshots from local filesystem
	snapshotFiles, err := e.listLocalSnapshots()
	if err != nil {
		return err
	}

	nodeNames := []string{os.Getenv("NODE_NAME")}

	// Get snapshots from S3
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
			nodeNames = append(nodeNames, "s3")
		}
	}

	// Try to load metadata from the legacy configmap, in case any local or s3 snapshots
	// were created by an old release that does not write the metadata alongside the snapshot file.
	snapshotConfigMap, err := e.config.Runtime.Core.Core().V1().ConfigMap().Get(metav1.NamespaceSystem, snapshotConfigMapName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if snapshotConfigMap != nil {
		for sfKey, sf := range snapshotFiles {
			logrus.Debugf("Found snapshotFile for %s with key %s", sf.Name, sfKey)
			// if the configmap has data for this snapshot, and local metadata is empty,
			// deserialize the value from the configmap and attempt to load it.
			if cmSnapshotValue := snapshotConfigMap.Data[sfKey]; cmSnapshotValue != "" && sf.Metadata == "" && sf.metadataSource == nil {
				sfTemp := &snapshotFile{}
				if err := json.Unmarshal([]byte(cmSnapshotValue), sfTemp); err != nil {
					logrus.Warnf("Failed to unmarshal configmap data for snapshot %s: %v", sfKey, err)
					continue
				}
				sf.Metadata = sfTemp.Metadata
				snapshotFiles[sfKey] = sf
			}
		}
	}

	labelSelector := &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{{
			Key:      labelStorageNode,
			Operator: metav1.LabelSelectorOpIn,
			Values:   nodeNames,
		}},
	}

	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return err
	}

	// List all snapshots matching the selector
	snapshots := e.config.Runtime.K3s.K3s().V1().ETCDSnapshotFile()
	esfList, err := snapshots.List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}

	// If a snapshot from Kubernetes was found on disk/s3, it is in sync and we can remove it from the map to sync.
	// If a snapshot from Kubernetes was not found on disk/s3, is is gone and can be removed from Kubernetes.
	// The one exception to the last rule is failed snapshots - these must be retained for a period of time.
	for _, esf := range esfList.Items {
		sfKey := generateETCDSnapshotFileConfigMapKey(esf)
		logrus.Debugf("Found ETCDSnapshotFile for %s with key %s", esf.Spec.SnapshotName, sfKey)
		if sf, ok := snapshotFiles[sfKey]; ok && generateSnapshotName(sf) == esf.Name {
			// exists in both and names match, don't need to sync
			delete(snapshotFiles, sfKey)
		} else {
			// doesn't exist on disk - if it's an error that hasn't expired yet, leave it, otherwise remove it
			if esf.Status.Error != nil && esf.Status.Error.Time != nil {
				expires := esf.Status.Error.Time.Add(errorTTL)
				if time.Now().Before(expires) {
					continue
				}
			}
			if ok {
				logrus.Debugf("Name of ETCDSnapshotFile for snapshotFile with key %s does not match: %s vs %s", sfKey, generateSnapshotName(sf), esf.Name)
			} else {
				logrus.Debugf("Key %s not found in snapshotFile list", sfKey)
			}
			logrus.Infof("Deleting ETCDSnapshotFile for %s", esf.Spec.SnapshotName)
			if err := snapshots.Delete(esf.Name, &metav1.DeleteOptions{}); err != nil {
				logrus.Errorf("Failed to delete ETCDSnapshotFile: %v", err)
			}
		}
	}

	// Any snapshots remaining in the map from disk/s3 were not found in Kubernetes and need to be created
	for _, sf := range snapshotFiles {
		logrus.Infof("Creating ETCDSnapshotFile for %s", sf.Name)
		if err := e.addSnapshotData(sf); err != nil {
			logrus.Errorf("Failed to create ETCDSnapshotFile: %v", err)
		}
	}

	// Agentless servers do not have a node. If we are running agentless, return early to avoid pruning
	// snapshots for nonexistent nodes and trying to patch the reconcile annotations on our node.
	if e.config.DisableAgent {
		return nil
	}

	// List all snapshots in Kubernetes not stored on S3 or a current etcd node.
	// These snapshots are local to a node that no longer runs etcd and cannot be restored.
	// If the node rejoins later and has local snapshots, it will reconcile them itself.
	labelSelector.MatchExpressions[0].Operator = metav1.LabelSelectorOpNotIn
	labelSelector.MatchExpressions[0].Values = []string{"s3"}

	// Get a list of all etcd nodes currently in the cluster and add them to the selector
	nodes := e.config.Runtime.Core.Core().V1().Node()
	etcdSelector := labels.Set{util.ETCDRoleLabelKey: "true"}
	nodeList, err := nodes.List(metav1.ListOptions{LabelSelector: etcdSelector.String()})
	if err != nil {
		return err
	}

	for _, node := range nodeList.Items {
		labelSelector.MatchExpressions[0].Values = append(labelSelector.MatchExpressions[0].Values, node.Name)
	}

	selector, err = metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return err
	}

	// List and remove all snapshots stored on nodes that do not match the selector
	esfList, err = snapshots.List(metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}

	for _, esf := range esfList.Items {
		if err := snapshots.Delete(esf.Name, &metav1.DeleteOptions{}); err != nil {
			logrus.Errorf("Failed to delete ETCDSnapshotFile for non-etcd node %s: %v", esf.Spec.NodeName, err)
		}
	}

	// Update our Node object to note the timestamp of the snapshot storages that have been reconciled
	now := time.Now().Round(time.Second).Format(time.RFC3339)
	patch := []map[string]string{
		{
			"op":    "add",
			"value": now,
			"path":  "/metadata/annotations/" + strings.ReplaceAll(annotationLocalReconciled, "/", "~1"),
		},
	}
	if e.config.EtcdS3 {
		patch = append(patch, map[string]string{
			"op":    "add",
			"value": now,
			"path":  "/metadata/annotations/" + strings.ReplaceAll(annotationS3Reconciled, "/", "~1"),
		})
	}
	b, err := json.Marshal(patch)
	if err != nil {
		return err
	}
	_, err = nodes.Patch(nodeNames[0], types.JSONPatchType, b)
	return err
}

// setSnapshotFunction schedules snapshots at the configured interval.
func (e *ETCD) setSnapshotFunction(ctx context.Context) {
	skipJob := cron.SkipIfStillRunning(cronLogger)
	e.cron.AddJob(e.config.EtcdSnapshotCron, skipJob(cron.FuncJob(func() {
		// Add a small amount of jitter to the actual snapshot execution. On clusters with multiple servers,
		// having all the nodes take a snapshot at the exact same time can lead to excessive retry thrashing
		// when updating the snapshot list configmap.
		time.Sleep(time.Duration(rand.Float64() * float64(snapshotJitterMax)))
		if _, err := e.Snapshot(ctx); err != nil {
			logrus.Errorf("Failed to take scheduled snapshot: %v", err)
		}
	})))
}

// snapshotRetention iterates through the snapshots and removes the oldest
// leaving the desired number of snapshots. Returns a list of pruned snapshot names.
func snapshotRetention(retention int, snapshotPrefix string, snapshotDir string) ([]string, error) {
	if retention < 1 {
		return nil, nil
	}

	logrus.Infof("Applying snapshot retention=%d to local snapshots with prefix %s in %s", retention, snapshotPrefix, snapshotDir)

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
		return nil, err
	}
	if len(snapshotFiles) <= retention {
		return nil, nil
	}

	// sort newest-first so we can prune entries past the retention count
	sort.Slice(snapshotFiles, func(i, j int) bool {
		return snapshotFiles[j].CreatedAt.Before(snapshotFiles[i].CreatedAt)
	})

	deleted := []string{}
	for _, df := range snapshotFiles[retention:] {
		snapshotPath := filepath.Join(snapshotDir, df.Name)
		metadataPath := filepath.Join(snapshotDir, "..", metadataDir, df.Name)
		logrus.Infof("Removing local snapshot %s", snapshotPath)
		if err := os.Remove(snapshotPath); err != nil {
			return deleted, err
		}
		if err := os.Remove(metadataPath); err != nil && !os.IsNotExist(err) {
			return deleted, err
		}
		deleted = append(deleted, df.Name)
	}

	return deleted, nil
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

func (sf *snapshotFile) fromETCDSnapshotFile(esf *k3s.ETCDSnapshotFile) {
	if esf == nil {
		panic("cannot convert from nil ETCDSnapshotFile")
	}

	sf.Name = esf.Spec.SnapshotName
	sf.Location = esf.Spec.Location
	sf.CreatedAt = esf.Status.CreationTime
	sf.nodeSource = esf.Spec.NodeName
	sf.Compressed = strings.HasSuffix(esf.Spec.SnapshotName, compressedExtension)

	if esf.Status.ReadyToUse != nil && *esf.Status.ReadyToUse {
		sf.Status = successfulSnapshotStatus
	} else {
		sf.Status = failedSnapshotStatus
	}

	if esf.Status.Size != nil {
		sf.Size = esf.Status.Size.Value()
	}

	if esf.Status.Error != nil {
		if esf.Status.Error.Time != nil {
			sf.CreatedAt = esf.Status.Error.Time
		}
		message := "etcd snapshot failed"
		if esf.Status.Error.Message != nil {
			message = *esf.Status.Error.Message
		}
		sf.Message = base64.StdEncoding.EncodeToString([]byte(message))
	}

	if len(esf.Spec.Metadata) > 0 {
		if b, err := json.Marshal(esf.Spec.Metadata); err != nil {
			logrus.Warnf("Failed to marshal metadata for %s: %v", esf.Name, err)
		} else {
			sf.Metadata = base64.StdEncoding.EncodeToString(b)
		}
	}

	if tokenHash := esf.Annotations[annotationTokenHash]; tokenHash != "" {
		sf.tokenHash = tokenHash
	}

	if esf.Spec.S3 == nil {
		sf.NodeName = esf.Spec.NodeName
	} else {
		sf.NodeName = "s3"
		sf.S3 = &s3Config{
			Endpoint:      esf.Spec.S3.Endpoint,
			EndpointCA:    esf.Spec.S3.EndpointCA,
			SkipSSLVerify: esf.Spec.S3.SkipSSLVerify,
			Bucket:        esf.Spec.S3.Bucket,
			Region:        esf.Spec.S3.Region,
			Folder:        esf.Spec.S3.Prefix,
			Insecure:      esf.Spec.S3.Insecure,
		}
	}
}

func (sf *snapshotFile) toETCDSnapshotFile(esf *k3s.ETCDSnapshotFile) {
	if esf == nil {
		panic("cannot convert to nil ETCDSnapshotFile")
	}
	esf.Spec.SnapshotName = sf.Name
	esf.Spec.Location = sf.Location
	esf.Status.CreationTime = sf.CreatedAt
	esf.Status.ReadyToUse = ptr.To(sf.Status == successfulSnapshotStatus)
	esf.Status.Size = resource.NewQuantity(sf.Size, resource.DecimalSI)

	if sf.nodeSource != "" {
		esf.Spec.NodeName = sf.nodeSource
	} else {
		esf.Spec.NodeName = sf.NodeName
	}

	if sf.Message != "" {
		var message string
		b, err := base64.StdEncoding.DecodeString(sf.Message)
		if err != nil {
			logrus.Warnf("Failed to decode error message for %s: %v", sf.Name, err)
			message = "etcd snapshot failed"
		} else {
			message = string(b)
		}
		esf.Status.Error = &k3s.ETCDSnapshotError{
			Time:    sf.CreatedAt,
			Message: &message,
		}
	}

	if sf.metadataSource != nil {
		esf.Spec.Metadata = sf.metadataSource.Data
	} else if sf.Metadata != "" {
		metadata, err := base64.StdEncoding.DecodeString(sf.Metadata)
		if err != nil {
			logrus.Warnf("Failed to decode metadata for %s: %v", sf.Name, err)
		} else {
			if err := json.Unmarshal(metadata, &esf.Spec.Metadata); err != nil {
				logrus.Warnf("Failed to unmarshal metadata for %s: %v", sf.Name, err)
			}
		}
	}

	if esf.ObjectMeta.Labels == nil {
		esf.ObjectMeta.Labels = map[string]string{}
	}

	if esf.ObjectMeta.Annotations == nil {
		esf.ObjectMeta.Annotations = map[string]string{}
	}

	if sf.tokenHash != "" {
		esf.ObjectMeta.Annotations[annotationTokenHash] = sf.tokenHash
	}

	if sf.S3 == nil {
		esf.ObjectMeta.Labels[labelStorageNode] = esf.Spec.NodeName
	} else {
		esf.ObjectMeta.Labels[labelStorageNode] = "s3"
		esf.Spec.S3 = &k3s.ETCDSnapshotS3{
			Endpoint:      sf.S3.Endpoint,
			EndpointCA:    sf.S3.EndpointCA,
			SkipSSLVerify: sf.S3.SkipSSLVerify,
			Bucket:        sf.S3.Bucket,
			Region:        sf.S3.Region,
			Prefix:        sf.S3.Folder,
			Insecure:      sf.S3.Insecure,
		}
	}
}
