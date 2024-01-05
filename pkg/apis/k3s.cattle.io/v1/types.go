package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Addon is used to track application of a manifest file on disk. It mostly exists so that the wrangler DesiredSet
// Apply controller has an object to track as the owner, and ensure that all created resources are tracked when the
// manifest is modified or removed.
type Addon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec provides information about the on-disk manifest backing this resource.
	Spec AddonSpec `json:"spec,omitempty"`
}

type AddonSpec struct {
	// Source is the Path on disk to the manifest file that this Addon tracks.
	Source string `json:"source,omitempty" column:""`
	// Checksum is the SHA256 checksum of the most recently successfully applied manifest file.
	Checksum string `json:"checksum,omitempty" column:""`
}

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ETCDSnapshot tracks a point-in-time snapshot of the etcd datastore.
type ETCDSnapshotFile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines properties of an etcd snapshot file
	Spec ETCDSnapshotSpec `json:"spec,omitempty"`
	// Status represents current information about a snapshot.
	Status ETCDSnapshotStatus `json:"status,omitempty"`
}

// ETCDSnapshotSpec desribes an etcd snapshot file
type ETCDSnapshotSpec struct {
	// SnapshotName contains the base name of the snapshot file. CLI actions that act
	// on snapshots stored locally or within a pre-configured S3 bucket and
	// prefix usually take the snapshot name as their argument.
	SnapshotName string `json:"snapshotName" column:""`
	// NodeName contains the name of the node that took the snapshot.
	NodeName string `json:"nodeName" column:"name=Node"`
	// Location is the absolute file:// or s3:// URI address of the snapshot.
	Location string `json:"location" column:""`
	// Metadata contains point-in-time snapshot of the contents of the
	// k3s-etcd-snapshot-extra-metadata ConfigMap's data field, at the time the
	// snapshot was taken. This is intended to contain data about cluster state
	// that may be important for an external system to have available when restoring
	// the snapshot.
	Metadata map[string]string `json:"metadata,omitempty"`
	// S3 contains extra metadata about the S3 storage system holding the
	// snapshot. This is guaranteed to be set for all snapshots uploaded to S3.
	// If not specified, the snapshot was not uploaded to S3.
	S3 *ETCDSnapshotS3 `json:"s3,omitempty"`
}

// ETCDSnapshotS3 holds information about the S3 storage system holding the snapshot.
type ETCDSnapshotS3 struct {
	// Endpoint is the host or host:port of the S3 service
	Endpoint string `json:"endpoint,omitempty"`
	// EndpointCA is the path on disk to the S3 service's trusted CA list. Leave empty to use the OS CA bundle.
	EndpointCA string `json:"endpointCA,omitempty"`
	// SkipSSLVerify is true if TLS certificate verification is disabled
	SkipSSLVerify bool `json:"skipSSLVerify,omitempty"`
	// Bucket is the bucket holding the snapshot
	Bucket string `json:"bucket,omitempty"`
	// Region is the region of the S3 service
	Region string `json:"region,omitempty"`
	// Prefix is the prefix in which the snapshot file is stored.
	Prefix string `json:"prefix,omitempty"`
	// Insecure is true if the S3 service uses HTTP instead of HTTPS
	Insecure bool `json:"insecure,omitempty"`
}

// ETCDSnapshotStatus is the status of the ETCDSnapshotFile object.
type ETCDSnapshotStatus struct {
	// Size is the size of the snapshot file, in bytes. If not specified, the snapshot failed.
	Size *resource.Quantity `json:"size,omitempty" column:""`
	// CreationTime is the timestamp when the snapshot was taken by etcd.
	CreationTime *metav1.Time `json:"creationTime,omitempty" column:""`
	// ReadyToUse indicates that the snapshot is available to be restored.
	ReadyToUse *bool `json:"readyToUse,omitempty"`
	// Error is the last observed error during snapshot creation, if any.
	// If the snapshot is retried, this field will be cleared on success.
	Error *ETCDSnapshotError `json:"error,omitempty"`
}

// ETCDSnapshotError describes an error encountered during snapshot creation.
type ETCDSnapshotError struct {
	// Time is the timestamp when the error was encountered.
	Time *metav1.Time `json:"time,omitempty"`
	// Message is a string detailing the encountered error during snapshot creation if specified.
	// NOTE: message may be logged, and it should not contain sensitive information.
	Message *string `json:"message,omitempty"`
}
