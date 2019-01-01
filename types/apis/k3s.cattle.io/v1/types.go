package v1

import (
	"github.com/rancher/norman/pkg/dynamiclistener"
	"github.com/rancher/norman/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ListenerConfig struct {
	types.Namespaced

	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status dynamiclistener.ListenerStatus `json:"status,omitempty"`
}
