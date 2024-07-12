package snapshot

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	k3s "github.com/k3s-io/k3s/pkg/apis/k3s.cattle.io/v1"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/minio/minio-go/v7"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"
)

type SnapshotStatus string

const (
	SuccessfulStatus SnapshotStatus = "successful"
	FailedStatus     SnapshotStatus = "failed"

	CompressedExtension = ".zip"
	MetadataDir         = ".metadata"
)

var (
	InvalidKeyChars = regexp.MustCompile(`[^-._a-zA-Z0-9]`)

	LabelStorageNode    = "etcd." + version.Program + ".cattle.io/snapshot-storage-node"
	AnnotationTokenHash = "etcd." + version.Program + ".cattle.io/snapshot-token-hash"

	ExtraMetadataConfigMapName = version.Program + "-etcd-snapshot-extra-metadata"
)

type S3Config struct {
	config.EtcdS3
	// Mask these fields in the embedded struct to avoid serializing their values in the snapshotFile record
	AccessKey    string          `json:"accessKey,omitempty"`
	ConfigSecret string          `json:"configSecret,omitempty"`
	Proxy        string          `json:"proxy,omitempty"`
	SecretKey    string          `json:"secretKey,omitempty"`
	Timeout      metav1.Duration `json:"timeout,omitempty"`
}

// File represents a single snapshot and it's
// metadata.
type File struct {
	Name string `json:"name"`
	// Location contains the full path of the snapshot. For
	// local paths, the location will be prefixed with "file://".
	Location   string         `json:"location,omitempty"`
	Metadata   string         `json:"metadata,omitempty"`
	Message    string         `json:"message,omitempty"`
	NodeName   string         `json:"nodeName,omitempty"`
	CreatedAt  *metav1.Time   `json:"createdAt,omitempty"`
	Size       int64          `json:"size,omitempty"`
	Status     SnapshotStatus `json:"status,omitempty"`
	S3         *S3Config      `json:"s3Config,omitempty"`
	Compressed bool           `json:"compressed"`

	// these fields are used for the internal representation of the snapshot
	// to populate other fields before serialization to the legacy configmap.
	MetadataSource *v1.ConfigMap `json:"-"`
	NodeSource     string        `json:"-"`
	TokenHash      string        `json:"-"`
}

// GenerateConfigMapKey generates a derived name for the snapshot that is safe for use
// as a configmap key.
func (sf *File) GenerateConfigMapKey() string {
	name := InvalidKeyChars.ReplaceAllString(sf.Name, "_")
	if sf.NodeName == "s3" {
		return "s3-" + name
	}
	return "local-" + name
}

// GenerateName generates a derived name for the snapshot that is safe for use
// as a resource name.
func (sf *File) GenerateName() string {
	name := strings.ToLower(sf.Name)
	nodename := sf.NodeSource
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
			name += CompressedExtension
		}
	}
	if sf.NodeName == "s3" {
		return "s3-" + name + "-" + hex.EncodeToString(digest[0:])[0:6]
	}
	return "local-" + name + "-" + hex.EncodeToString(digest[0:])[0:6]
}

// FromETCDSnapshotFile translates fields to the File from the ETCDSnapshotFile
func (sf *File) FromETCDSnapshotFile(esf *k3s.ETCDSnapshotFile) {
	if esf == nil {
		panic("cannot convert from nil ETCDSnapshotFile")
	}

	sf.Name = esf.Spec.SnapshotName
	sf.Location = esf.Spec.Location
	sf.CreatedAt = esf.Status.CreationTime
	sf.NodeSource = esf.Spec.NodeName
	sf.Compressed = strings.HasSuffix(esf.Spec.SnapshotName, CompressedExtension)

	if esf.Status.ReadyToUse != nil && *esf.Status.ReadyToUse {
		sf.Status = SuccessfulStatus
	} else {
		sf.Status = FailedStatus
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

	if tokenHash := esf.Annotations[AnnotationTokenHash]; tokenHash != "" {
		sf.TokenHash = tokenHash
	}

	if esf.Spec.S3 == nil {
		sf.NodeName = esf.Spec.NodeName
	} else {
		sf.NodeName = "s3"
		sf.S3 = &S3Config{
			EtcdS3: config.EtcdS3{
				Endpoint:      esf.Spec.S3.Endpoint,
				EndpointCA:    esf.Spec.S3.EndpointCA,
				SkipSSLVerify: esf.Spec.S3.SkipSSLVerify,
				Bucket:        esf.Spec.S3.Bucket,
				Region:        esf.Spec.S3.Region,
				Folder:        esf.Spec.S3.Prefix,
				Insecure:      esf.Spec.S3.Insecure,
			},
		}
	}
}

// ToETCDSnapshotFile translates fields from the File to the ETCDSnapshotFile
func (sf *File) ToETCDSnapshotFile(esf *k3s.ETCDSnapshotFile) {
	if esf == nil {
		panic("cannot convert to nil ETCDSnapshotFile")
	}
	esf.Spec.SnapshotName = sf.Name
	esf.Spec.Location = sf.Location
	esf.Status.CreationTime = sf.CreatedAt
	esf.Status.ReadyToUse = ptr.To(sf.Status == SuccessfulStatus)
	esf.Status.Size = resource.NewQuantity(sf.Size, resource.DecimalSI)

	if sf.NodeSource != "" {
		esf.Spec.NodeName = sf.NodeSource
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

	if sf.MetadataSource != nil {
		esf.Spec.Metadata = sf.MetadataSource.Data
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

	if sf.TokenHash != "" {
		esf.ObjectMeta.Annotations[AnnotationTokenHash] = sf.TokenHash
	}

	if sf.S3 == nil {
		esf.ObjectMeta.Labels[LabelStorageNode] = esf.Spec.NodeName
	} else {
		esf.ObjectMeta.Labels[LabelStorageNode] = "s3"
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

// Marshal returns the JSON encoding of the snapshot File, with metadata inlined as base64.
func (sf *File) Marshal() ([]byte, error) {
	if sf.MetadataSource != nil {
		if m, err := json.Marshal(sf.MetadataSource.Data); err != nil {
			logrus.Debugf("Error attempting to marshal extra metadata contained in %s ConfigMap, error: %v", ExtraMetadataConfigMapName, err)
		} else {
			sf.Metadata = base64.StdEncoding.EncodeToString(m)
		}
	}
	return json.Marshal(sf)
}

// IsNotExist returns true if the error is from http.StatusNotFound or os.IsNotExist
func IsNotExist(err error) bool {
	if resp := minio.ToErrorResponse(err); resp.StatusCode == http.StatusNotFound || os.IsNotExist(err) {
		return true
	}
	return false
}
