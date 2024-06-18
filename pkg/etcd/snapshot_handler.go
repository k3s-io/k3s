package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	k3s "github.com/k3s-io/k3s/pkg/apis/k3s.cattle.io/v1"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SnapshotOperation string

const (
	SnapshotOperationSave   SnapshotOperation = "save"
	SnapshotOperationList   SnapshotOperation = "list"
	SnapshotOperationPrune  SnapshotOperation = "prune"
	SnapshotOperationDelete SnapshotOperation = "delete"
)

type SnapshotRequestS3 struct {
	s3Config
	Timeout   metav1.Duration `json:"timeout"`
	AccessKey string          `json:"accessKey"`
	SecretKey string          `json:"secretKey"`
}

type SnapshotRequest struct {
	Operation SnapshotOperation `json:"operation"`
	Name      []string          `json:"name,omitempty"`
	Dir       *string           `json:"dir,omitempty"`
	Compress  *bool             `json:"compress,omitempty"`
	Retention *int              `json:"retention,omitempty"`

	S3 *SnapshotRequestS3 `json:"s3,omitempty"`

	ctx context.Context
}

func (sr *SnapshotRequest) context() context.Context {
	if sr.ctx != nil {
		return sr.ctx
	}
	return context.Background()
}

// snapshotHandler handles snapshot save/list/prune requests from the CLI.
func (e *ETCD) snapshotHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		sr, err := getSnapshotRequest(req)
		if err != nil {
			util.SendErrorWithID(err, "etcd-snapshot", rw, req, http.StatusInternalServerError)
		}
		switch sr.Operation {
		case SnapshotOperationList:
			err = e.withRequest(sr).handleList(rw, req)
		case SnapshotOperationSave:
			err = e.withRequest(sr).handleSave(rw, req)
		case SnapshotOperationPrune:
			err = e.withRequest(sr).handlePrune(rw, req)
		case SnapshotOperationDelete:
			err = e.withRequest(sr).handleDelete(rw, req, sr.Name)
		default:
			err = e.handleInvalid(rw, req)
		}
		if err != nil {
			logrus.Warnf("Error in etcd-snapshot handler: %v", err)
		}
	})
}

func (e *ETCD) handleList(rw http.ResponseWriter, req *http.Request) error {
	if err := e.initS3IfNil(req.Context()); err != nil {
		util.SendError(err, rw, req, http.StatusBadRequest)
		return nil
	}
	sf, err := e.ListSnapshots(req.Context())
	if sf == nil {
		util.SendErrorWithID(err, "etcd-snapshot", rw, req, http.StatusInternalServerError)
		return nil
	}
	sendSnapshotList(rw, req, sf)
	return err
}

func (e *ETCD) handleSave(rw http.ResponseWriter, req *http.Request) error {
	if err := e.initS3IfNil(req.Context()); err != nil {
		util.SendError(err, rw, req, http.StatusBadRequest)
		return nil
	}
	sr, err := e.Snapshot(req.Context())
	if sr == nil {
		util.SendErrorWithID(err, "etcd-snapshot", rw, req, http.StatusInternalServerError)
		return nil
	}
	sendSnapshotResponse(rw, req, sr)
	return err
}

func (e *ETCD) handlePrune(rw http.ResponseWriter, req *http.Request) error {
	if err := e.initS3IfNil(req.Context()); err != nil {
		util.SendError(err, rw, req, http.StatusBadRequest)
		return nil
	}
	sr, err := e.PruneSnapshots(req.Context())
	if sr == nil {
		util.SendError(err, rw, req, http.StatusInternalServerError)
		return nil
	}
	sendSnapshotResponse(rw, req, sr)
	return err
}

func (e *ETCD) handleDelete(rw http.ResponseWriter, req *http.Request, snapshots []string) error {
	if err := e.initS3IfNil(req.Context()); err != nil {
		util.SendError(err, rw, req, http.StatusBadRequest)
		return nil
	}
	sr, err := e.DeleteSnapshots(req.Context(), snapshots)
	if sr == nil {
		util.SendError(err, rw, req, http.StatusInternalServerError)
		return nil
	}
	sendSnapshotResponse(rw, req, sr)
	return err
}

func (e *ETCD) handleInvalid(rw http.ResponseWriter, req *http.Request) error {
	util.SendErrorWithID(fmt.Errorf("invalid snapshot operation"), "etcd-snapshot", rw, req, http.StatusBadRequest)
	return nil
}

// withRequest returns a modified ETCD struct that is overriden
// with etcd snapshot config from the snapshot request.
func (e *ETCD) withRequest(sr *SnapshotRequest) *ETCD {
	re := &ETCD{
		client: e.client,
		config: &config.Control{
			CriticalControlArgs:   e.config.CriticalControlArgs,
			Runtime:               e.config.Runtime,
			DataDir:               e.config.DataDir,
			Datastore:             e.config.Datastore,
			EtcdSnapshotCompress:  e.config.EtcdSnapshotCompress,
			EtcdSnapshotName:      e.config.EtcdSnapshotName,
			EtcdSnapshotRetention: e.config.EtcdSnapshotRetention,
		},
		name:       e.name,
		address:    e.address,
		cron:       e.cron,
		cancel:     e.cancel,
		snapshotMu: e.snapshotMu,
	}
	if len(sr.Name) > 0 {
		re.config.EtcdSnapshotName = sr.Name[0]
	}
	if sr.Compress != nil {
		re.config.EtcdSnapshotCompress = *sr.Compress
	}
	if sr.Dir != nil {
		re.config.EtcdSnapshotDir = *sr.Dir
	}
	if sr.Retention != nil {
		re.config.EtcdSnapshotRetention = *sr.Retention
	}
	if sr.S3 != nil {
		re.config.EtcdS3 = true
		re.config.EtcdS3AccessKey = sr.S3.AccessKey
		re.config.EtcdS3BucketName = sr.S3.Bucket
		re.config.EtcdS3Endpoint = sr.S3.Endpoint
		re.config.EtcdS3EndpointCA = sr.S3.EndpointCA
		re.config.EtcdS3Folder = sr.S3.Folder
		re.config.EtcdS3Insecure = sr.S3.Insecure
		re.config.EtcdS3Region = sr.S3.Region
		re.config.EtcdS3SecretKey = sr.S3.SecretKey
		re.config.EtcdS3SkipSSLVerify = sr.S3.SkipSSLVerify
		re.config.EtcdS3Timeout = sr.S3.Timeout.Duration
	}
	return re
}

// getSnapshotRequest unmarshalls the snapshot operation request from a client.
func getSnapshotRequest(req *http.Request) (*SnapshotRequest, error) {
	if req.Method != http.MethodPost {
		return nil, http.ErrNotSupported
	}
	sr := &SnapshotRequest{}
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &sr); err != nil {
		return nil, err
	}
	sr.ctx = req.Context()
	return sr, nil
}

func sendSnapshotResponse(rw http.ResponseWriter, req *http.Request, sr *managed.SnapshotResult) {
	b, err := json.Marshal(sr)
	if err != nil {
		util.SendErrorWithID(err, "etcd-snapshot", rw, req, http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(b)
}

func sendSnapshotList(rw http.ResponseWriter, req *http.Request, sf *k3s.ETCDSnapshotFileList) {
	b, err := json.Marshal(sf)
	if err != nil {
		util.SendErrorWithID(err, "etcd-snapshot", rw, req, http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(b)
}
