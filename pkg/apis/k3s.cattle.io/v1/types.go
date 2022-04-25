package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Addon struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AddonSpec   `json:"spec,omitempty"`
	Status AddonStatus `json:"status,omitempty"`
}

type AddonSpec struct {
	Source   string `json:"source,omitempty"`
	Checksum string `json:"checksum,omitempty"`
}

type AddonStatus struct {
	GVKs []schema.GroupVersionKind `json:"gvks,omitempty"`
}
