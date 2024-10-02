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
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	k3s "github.com/k3s-io/k3s/pkg/apis/k3s.cattle.io/v1"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd/s3"
	"github.com/k3s-io/k3s/pkg/etcd/snapshot"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/pkg/errors"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	snapshotv3 "go.etcd.io/etcd/etcdutl/v3/snapshot"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/pager"
	"k8s.io/client-go/util/retry"
)

const (
	errorTTL             = 24 * time.Hour
	s3ReconcileTTL       = time.Minute
	snapshotListPageSize = 20
)

var (
	annotationLocalReconciled = "etcd." + version.Program + ".cattle.io/local-snapshots-timestamp"
	annotationS3Reconciled    = "etcd." + version.Program + ".cattle.io/s3-snapshots-timestamp"

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

	zippedSnapshotName := snapshotName + snapshot.CompressedExtension
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
		decompressed, err = os.OpenFile(strings.Replace(sf.Name, snapshot.CompressedExtension, "", -1), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, sf.Mode())
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
		logrus.Debugf("Cannot retrieve extra metadata from %s ConfigMap: runtime core not ready", snapshot.ExtraMetadataConfigMapName)
	} else {
		logrus.Debugf("Attempting to retrieve extra metadata from %s ConfigMap", snapshot.ExtraMetadataConfigMapName)
		if snapshotExtraMetadataConfigMap, err := e.config.Runtime.Core.Core().V1().ConfigMap().Get(metav1.NamespaceSystem, snapshot.ExtraMetadataConfigMapName, metav1.GetOptions{}); err != nil {
			logrus.Debugf("Error encountered attempting to retrieve extra metadata from %s ConfigMap, error: %v", snapshot.ExtraMetadataConfigMapName, err)
		} else {
			logrus.Debugf("Setting extra metadata from %s ConfigMap", snapshot.ExtraMetadataConfigMapName)
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

	var sf *snapshot.File

	if err := snapshotv3.NewV3(e.client.GetLogger()).Save(ctx, *cfg, snapshotPath); err != nil {
		sf = &snapshot.File{
			Name:     snapshotName,
			Location: "",
			NodeName: nodeName,
			CreatedAt: &metav1.Time{
				Time: now,
			},
			Status:         snapshot.FailedStatus,
			Message:        base64.StdEncoding.EncodeToString([]byte(err.Error())),
			Size:           0,
			MetadataSource: extraMetadata,
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

		sf = &snapshot.File{
			Name:     f.Name(),
			Location: "file://" + snapshotPath,
			NodeName: nodeName,
			CreatedAt: &metav1.Time{
				Time: now,
			},
			Status:         snapshot.SuccessfulStatus,
			Size:           f.Size(),
			Compressed:     e.config.EtcdSnapshotCompress,
			MetadataSource: extraMetadata,
			TokenHash:      tokenHash,
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

		if e.config.EtcdS3 != nil {
			if s3client, err := e.getS3Client(ctx); err != nil {
				logrus.Warnf("Unable to initialize S3 client: %v", err)
				if !errors.Is(err, s3.ErrNoConfigSecret) {
					err = errors.Wrap(err, "failed to initialize S3 client")
					sf = &snapshot.File{
						Name:     f.Name(),
						NodeName: "s3",
						CreatedAt: &metav1.Time{
							Time: now,
						},
						Message:        base64.StdEncoding.EncodeToString([]byte(err.Error())),
						Size:           0,
						Status:         snapshot.FailedStatus,
						S3:             &snapshot.S3Config{EtcdS3: *e.config.EtcdS3},
						MetadataSource: extraMetadata,
					}
				}
			} else {
				logrus.Infof("Saving etcd snapshot %s to S3", snapshotName)
				// upload will return a snapshot.File even on error - if there was an
				// error, it will be reflected in the status and message.
				sf, err = s3client.Upload(ctx, snapshotPath, extraMetadata, now)
				if err != nil {
					logrus.Errorf("Error received during snapshot upload to S3: %s", err)
				} else {
					res.Created = append(res.Created, sf.Name)
					logrus.Infof("S3 upload complete for %s", snapshotName)
				}
				// Attempt to apply retention even if the upload failed; failure may be due to bucket
				// being full or some other condition that retention policy would resolve.
				// Snapshot retention may prune some files before returning an error. Failing to prune is not fatal.
				deleted, err := s3client.SnapshotRetention(ctx, e.config.EtcdSnapshotRetention, e.config.EtcdSnapshotName)
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

	return res, e.reconcileSnapshotData(ctx, res)
}

// listLocalSnapshots provides a list of the currently stored
// snapshots on disk along with their relevant
// metadata.
func (e *ETCD) listLocalSnapshots() (map[string]snapshot.File, error) {
	nodeName := os.Getenv("NODE_NAME")
	snapshots := make(map[string]snapshot.File)
	snapshotDir, err := snapshotDir(e.config, true)
	if err != nil {
		return snapshots, errors.Wrap(err, "failed to get etcd-snapshot-dir")
	}

	if err := filepath.Walk(snapshotDir, func(path string, file os.FileInfo, err error) error {
		if err != nil || file.IsDir() {
			return err
		}

		basename, compressed := strings.CutSuffix(file.Name(), snapshot.CompressedExtension)
		ts, err := strconv.ParseInt(basename[strings.LastIndexByte(basename, '-')+1:], 10, 64)
		if err != nil {
			ts = file.ModTime().Unix()
		}

		// try to read metadata from disk; don't warn if it is missing as it will not exist
		// for snapshot files from old releases or if there was no metadata provided.
		var metadata string
		metadataFile := filepath.Join(filepath.Dir(path), "..", snapshot.MetadataDir, file.Name())
		if m, err := os.ReadFile(metadataFile); err == nil {
			logrus.Debugf("Loading snapshot metadata from %s", metadataFile)
			metadata = base64.StdEncoding.EncodeToString(m)
		}

		sf := snapshot.File{
			Name:     file.Name(),
			Location: "file://" + filepath.Join(snapshotDir, file.Name()),
			NodeName: nodeName,
			Metadata: metadata,
			CreatedAt: &metav1.Time{
				Time: time.Unix(ts, 0),
			},
			Size:       file.Size(),
			Status:     snapshot.SuccessfulStatus,
			Compressed: compressed,
		}
		sfKey := sf.GenerateConfigMapKey()
		snapshots[sfKey] = sf
		return nil
	}); err != nil {
		return nil, err
	}

	return snapshots, nil
}

// getS3Client initializes the S3 controller if it hasn't yet been initialized.
// If S3 is or can be initialized successfully, and valid S3 configuration is
// present, a client for the current S3 configuration is returned.
// The context passed here is only used to validate the configuration,
// it does not need to continue to remain uncancelled after the call returns.
func (e *ETCD) getS3Client(ctx context.Context) (*s3.Client, error) {
	if e.s3 == nil {
		s3, err := s3.Start(ctx, e.config)
		if err != nil {
			return nil, err
		}
		e.s3 = s3
	}

	return e.s3.GetClient(ctx, e.config.EtcdS3)
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

	if e.config.EtcdS3 != nil {
		if s3client, err := e.getS3Client(ctx); err != nil {
			logrus.Warnf("Unable to initialize S3 client: %v", err)
		} else {
			deleted, err := s3client.SnapshotRetention(ctx, e.config.EtcdSnapshotRetention, e.config.EtcdSnapshotName)
			if err != nil {
				logrus.Errorf("Error applying S3 snapshot retention policy: %v", err)
			}
			res.Deleted = append(res.Deleted, deleted...)
		}
	}
	return res, e.reconcileSnapshotData(ctx, res)
}

// ListSnapshots returns a list of snapshots. Local snapshots are always listed,
// s3 snapshots are listed if s3 is enabled.
// Snapshots are listed locally, not listed from the apiserver, so results
// are guaranteed to be in sync with what is on disk.
func (e *ETCD) ListSnapshots(ctx context.Context) (*k3s.ETCDSnapshotFileList, error) {
	snapshotFiles := &k3s.ETCDSnapshotFileList{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "List"},
	}

	if e.config.EtcdS3 != nil {
		if s3client, err := e.getS3Client(ctx); err != nil {
			logrus.Warnf("Unable to initialize S3 client: %v", err)
			if !errors.Is(err, s3.ErrNoConfigSecret) {
				return nil, errors.Wrap(err, "failed to initialize S3 client")
			}
		} else {
			sfs, err := s3client.ListSnapshots(ctx)
			if err != nil {
				return nil, err
			}
			for k, sf := range sfs {
				esf := k3s.NewETCDSnapshotFile("", k, k3s.ETCDSnapshotFile{})
				sf.ToETCDSnapshotFile(esf)
				snapshotFiles.Items = append(snapshotFiles.Items, *esf)
			}
		}
	}

	sfs, err := e.listLocalSnapshots()
	if err != nil {
		return nil, err
	}
	for k, sf := range sfs {
		esf := k3s.NewETCDSnapshotFile("", k, k3s.ETCDSnapshotFile{})
		sf.ToETCDSnapshotFile(esf)
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

	var s3client *s3.Client
	if e.config.EtcdS3 != nil {
		s3client, err = e.getS3Client(ctx)
		if err != nil {
			logrus.Warnf("Unable to initialize S3 client: %v", err)
			if !errors.Is(err, s3.ErrNoConfigSecret) {
				return nil, errors.Wrap(err, "failed to initialize S3 client")
			}
		}
	}

	res := &managed.SnapshotResult{}
	for _, s := range snapshots {
		if err := e.deleteSnapshot(filepath.Join(snapshotDir, s)); err != nil {
			if snapshot.IsNotExist(err) {
				logrus.Infof("Snapshot %s not found locally", s)
			} else {
				logrus.Errorf("Failed to delete local snapshot %s: %v", s, err)
			}
		} else {
			res.Deleted = append(res.Deleted, s)
			logrus.Infof("Snapshot %s deleted locally", s)
		}

		if s3client != nil {
			if err := s3client.DeleteSnapshot(ctx, s); err != nil {
				if snapshot.IsNotExist(err) {
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

	return res, e.reconcileSnapshotData(ctx, res)
}

func (e *ETCD) deleteSnapshot(snapshotPath string) error {
	dir := filepath.Join(filepath.Dir(snapshotPath), "..", snapshot.MetadataDir)
	filename := filepath.Base(snapshotPath)
	metadataPath := filepath.Join(dir, filename)

	err := os.Remove(snapshotPath)
	if err == nil || os.IsNotExist(err) {
		if merr := os.Remove(metadataPath); err != nil && !snapshot.IsNotExist(err) {
			err = merr
		}
	}

	return err
}

// addSnapshotData syncs an internal snapshotFile representation to an ETCDSnapshotFile resource
// of the same name. Resources will be created or updated as necessary.
func (e *ETCD) addSnapshotData(sf snapshot.File) error {
	// make sure the K3s factory is initialized.
	for e.config.Runtime.K3s == nil {
		runtime.Gosched()
	}

	snapshots := e.config.Runtime.K3s.K3s().V1().ETCDSnapshotFile()
	esfName := sf.GenerateName()

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
		sf.ToETCDSnapshotFile(esf)

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

// generateETCDSnapshotFileConfigMapKey generates a key that the corresponding
// snapshotFile would be stored under in the legacy configmap
func generateETCDSnapshotFileConfigMapKey(esf k3s.ETCDSnapshotFile) string {
	name := snapshot.InvalidKeyChars.ReplaceAllString(esf.Spec.SnapshotName, "_")
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
// It will reconcile snapshot data from disk locally always, and if S3 is enabled, will attempt to
// list S3 snapshots and reconcile snapshots from S3.
func (e *ETCD) ReconcileSnapshotData(ctx context.Context) error {
	return e.reconcileSnapshotData(ctx, nil)
}

// reconcileSnapshotData reconciles snapshot data in the ETCDSnapshotFile resources.
// It will reconcile snapshot data from disk locally always, and if S3 is enabled, will attempt to
// list S3 snapshots and reconcile snapshots from S3. Any snapshots listed in the Deleted field of
// the provided SnapshotResult are deleted, even if they are within a retention window.
func (e *ETCD) reconcileSnapshotData(ctx context.Context, res *managed.SnapshotResult) error {
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
	if e.config.EtcdS3 != nil {
		if s3client, err := e.getS3Client(ctx); err != nil {
			logrus.Warnf("Unable to initialize S3 client: %v", err)
			if !errors.Is(err, s3.ErrNoConfigSecret) {
				return errors.Wrap(err, "failed to initialize S3 client")
			}
		} else {
			if s3Snapshots, err := s3client.ListSnapshots(ctx); err != nil {
				logrus.Errorf("Error retrieving S3 snapshots for reconciliation: %v", err)
			} else {
				for k, v := range s3Snapshots {
					snapshotFiles[k] = v
				}
				nodeNames = append(nodeNames, "s3")
			}
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
			// deserialize the value from the configmap and attempt to load iM.
			if cmSnapshotValue := snapshotConfigMap.Data[sfKey]; cmSnapshotValue != "" && sf.Metadata == "" && sf.MetadataSource == nil {
				sfTemp := &snapshot.File{}
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
			Key:      snapshot.LabelStorageNode,
			Operator: metav1.LabelSelectorOpIn,
			Values:   nodeNames,
		}},
	}

	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return err
	}

	snapshots := e.config.Runtime.K3s.K3s().V1().ETCDSnapshotFile()
	snapshotPager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (k8sruntime.Object, error) { return snapshots.List(opts) }))
	snapshotPager.PageSize = snapshotListPageSize
	now := time.Now().Round(time.Second)

	// List all snapshots matching the selector
	// If a snapshot from Kubernetes was found on disk/s3, it is in sync and we can remove it from the map to sync.
	// If a snapshot from Kubernetes was not found on disk/s3, is is gone and can be removed from Kubernetes.
	// The one exception to the last rule is failed snapshots - these must be retained for a period of time.
	if err := snapshotPager.EachListItem(ctx, metav1.ListOptions{LabelSelector: selector.String()}, func(obj k8sruntime.Object) error {
		esf, ok := obj.(*k3s.ETCDSnapshotFile)
		if !ok {
			return errors.New("failed to convert object to ETCDSnapshotFile")
		}
		sfKey := generateETCDSnapshotFileConfigMapKey(*esf)
		logrus.Debugf("Found ETCDSnapshotFile for %s with key %s", esf.Spec.SnapshotName, sfKey)
		if sf, ok := snapshotFiles[sfKey]; ok && sf.GenerateName() == esf.Name {
			// exists in both and names match, don't need to sync
			delete(snapshotFiles, sfKey)
		} else {
			// doesn't exist on disk/s3
			if res != nil && slices.Contains(res.Deleted, esf.Spec.SnapshotName) {
				// snapshot has been intentionally deleted, skip checking for expiration
			} else if esf.Status.Error != nil && esf.Status.Error.Time != nil {
				expires := esf.Status.Error.Time.Add(errorTTL)
				if now.Before(expires) {
					// it's an error that hasn't expired yet, leave it
					return nil
				}
			} else if esf.Spec.S3 != nil {
				expires := esf.ObjectMeta.CreationTimestamp.Add(s3ReconcileTTL)
				if now.Before(expires) {
					// it's an s3 snapshot that's only just been created, leave it to prevent a race condition
					// when multiple nodes are uploading snapshots at the same time.
					return nil
				}
			}
			if ok {
				logrus.Debugf("Name of ETCDSnapshotFile for snapshotFile with key %s does not match: %s vs %s", sfKey, sf.GenerateName(), esf.Name)
			} else {
				logrus.Debugf("Key %s not found in snapshotFile list", sfKey)
			}
			// otherwise remove it
			logrus.Infof("Deleting ETCDSnapshotFile for %s", esf.Spec.SnapshotName)
			if err := snapshots.Delete(esf.Name, &metav1.DeleteOptions{}); err != nil {
				logrus.Errorf("Failed to delete ETCDSnapshotFile: %v", err)
			}
		}
		return nil
	}); err != nil {
		return err
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
	if err := snapshotPager.EachListItem(ctx, metav1.ListOptions{LabelSelector: selector.String()}, func(obj k8sruntime.Object) error {
		esf, ok := obj.(*k3s.ETCDSnapshotFile)
		if !ok {
			return errors.New("failed to convert object to ETCDSnapshotFile")
		}

		if err := snapshots.Delete(esf.Name, &metav1.DeleteOptions{}); err != nil {
			logrus.Errorf("Failed to delete ETCDSnapshotFile for non-etcd node %s: %v", esf.Spec.NodeName, err)
		}
		return nil
	}); err != nil {
		return err
	}

	// Update our Node object to note the timestamp of the snapshot storages that have been reconciled
	patch := []map[string]string{
		{
			"op":    "add",
			"value": now.Format(time.RFC3339),
			"path":  "/metadata/annotations/" + strings.ReplaceAll(annotationLocalReconciled, "/", "~1"),
		},
	}
	if e.config.EtcdS3 != nil {
		patch = append(patch, map[string]string{
			"op":    "add",
			"value": now.Format(time.RFC3339),
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

	var snapshotFiles []snapshot.File
	if err := filepath.Walk(snapshotDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || err != nil {
			return err
		}
		if strings.HasPrefix(info.Name(), snapshotPrefix) {
			basename, compressed := strings.CutSuffix(info.Name(), snapshot.CompressedExtension)
			ts, err := strconv.ParseInt(basename[strings.LastIndexByte(basename, '-')+1:], 10, 64)
			if err != nil {
				ts = info.ModTime().Unix()
			}
			snapshotFiles = append(snapshotFiles, snapshot.File{Name: info.Name(), CreatedAt: &metav1.Time{Time: time.Unix(ts, 0)}, Compressed: compressed})
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
		metadataPath := filepath.Join(snapshotDir, "..", snapshot.MetadataDir, df.Name)
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

// saveSnapshotMetadata writes extra metadata to disk.
// The upload is silently skipped if no extra metadata is provided.
func saveSnapshotMetadata(snapshotPath string, extraMetadata *v1.ConfigMap) error {
	if extraMetadata == nil || len(extraMetadata.Data) == 0 {
		return nil
	}

	dir := filepath.Join(filepath.Dir(snapshotPath), "..", snapshot.MetadataDir)
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
