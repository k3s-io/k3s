package v1

import (
	"github.com/rancher/norman/pkg/dynamiclistener"
	"github.com/rancher/norman/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ListenerConfig struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status dynamiclistener.ListenerStatus `json:"status,omitempty"`
}

type Addon struct {
	types.Namespaced

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
