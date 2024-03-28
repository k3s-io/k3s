package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	SnapshotOperationSave  = "save"
	SnapshotOperationList  = "list"
	SnapshotOperationPrune = "prune"
)

type SnapshotRequestS3 struct {
	s3Config
	Timeout   metav1.Duration `json:"timeout"`
	AccessKey string          `json:"accessKey"`
	SecretKey string          `json:"secretKey"`
}

type SnapshotRequest struct {
	Operation string  `json:"operation"`
	Dir       *string `json:dir,omitempty"`
	Name      *string `json:"name,omitempty"`
	Compress  *bool   `json:"compress,omitempty"`
	Retention *int    `json:"retention,omitempty"`

	S3 *SnapshotRequestS3 `json:"s3,omitempty"`

	ctx context.Context
}

func (sr *SnapshotRequest) context() context.Context {
	if sr.ctx != nil {
		return sr.ctx
	}
	return context.Background()
}

type SnapshotResponse struct {
}

// snapshotHandler handles snapshot save/list/prune requests from the CLI.
func (e *ETCD) snapshotHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		sr, err := getSnapshotRequest(req)
		if err != nil {
			util.SendError(err, rw, req, http.StatusInternalServerError)
		}
		switch sr.Operation {
		case SnapshotOperationSave:
			e.withRequest(sr).handleSave(rw, req)
		case SnapshotOperationList:
			e.withRequest(sr).handleList(rw, req)
		case SnapshotOperationPrune:
			e.withRequest(sr).handlePrune(rw, req)
		default:
			util.SendError(fmt.Errorf("invalid snapshot operation"), rw, req, http.StatusBadRequest)
		}
	})
}

func (e *ETCD) handleSave(rw http.ResponseWriter, req *http.Request) {
	err := e.Snapshot(req.Context())
	if err != nil {
		util.SendError(err, rw, req, http.StatusInternalServerError)
		return
	}
}

func (e *ETCD) handleList(rw http.ResponseWriter, req *http.Request) {
	_, err := e.ListSnapshots(req.Context())
	if err != nil {
		util.SendError(err, rw, req, http.StatusInternalServerError)
		return
	}
}

func (e *ETCD) handlePrune(rw http.ResponseWriter, req *http.Request) {
	err := e.PruneSnapshots(req.Context())
	if err != nil {
		util.SendError(err, rw, req, http.StatusInternalServerError)
		return
	}
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
		name:        e.name,
		address:     e.address,
		cron:        e.cron,
		cancel:      e.cancel,
		snapshotSem: e.snapshotSem,
	}
	if sr.Compress != nil {
		re.config.EtcdSnapshotCompress = *sr.Compress
	}
	if sr.Dir != nil {
		re.config.EtcdSnapshotDir = *sr.Dir
	}
	if sr.Name != nil {
		re.config.EtcdSnapshotName = *sr.Name
	}
	if sr.Retention != nil {
		re.config.EtcdSnapshotRetention = *sr.Retention
	}
	if sr.S3 != nil {
		re.config.EtcdS3 = true
		re.config.EtcdS3BucketName = sr.S3.Bucket
		re.config.EtcdS3AccessKey = sr.S3.AccessKey
		re.config.EtcdS3SecretKey = sr.S3.SecretKey
		re.config.EtcdS3EndpointCA = sr.S3.EndpointCA
		re.config.EtcdS3SkipSSLVerify = sr.S3.SkipSSLVerify
		re.config.EtcdS3Insecure = sr.S3.Insecure
		re.config.EtcdS3Region = sr.S3.Region
		re.config.EtcdS3Timeout = sr.S3.Timeout.Duration
	}
	return re
}

// getSnapshotRequest unmarshalls the snapshot operation request from a client.
func getSnapshotRequest(req *http.Request) (*SnapshotRequest, error) {
	b, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	sr := &SnapshotRequest{}
	err = json.Unmarshal(b, &sr)
	sr.ctx = req.Context()
	return sr, err
}

func sendSnapshotResponse(rw http.ResponseWriter, req *http.Request, sr *SnapshotResponse) {
	b, err := json.Marshal(sr)
	if err != nil {
		util.SendError(err, rw, req, http.StatusInternalServerError)
		return
	}
	rw.Header().Set("Content-Type", "application/json")
	rw.Write(b)
}
