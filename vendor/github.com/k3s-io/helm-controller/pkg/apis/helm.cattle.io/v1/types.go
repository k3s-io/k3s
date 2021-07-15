package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HelmChart struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmChartSpec   `json:"spec,omitempty"`
	Status HelmChartStatus `json:"status,omitempty"`
}

type HelmChartSpec struct {
	TargetNamespace string                        `json:"targetNamespace,omitempty"`
	Chart           string                        `json:"chart,omitempty"`
	Version         string                        `json:"version,omitempty"`
	Repo            string                        `json:"repo,omitempty"`
	Set             map[string]intstr.IntOrString `json:"set,omitempty"`
	ValuesContent   string                        `json:"valuesContent,omitempty"`
	HelmVersion     string                        `json:"helmVersion,omitempty"`
	Bootstrap       bool                          `json:"bootstrap,omitempty"`
	ChartContent    string                        `json:"chartContent,omitempty"`
	JobImage        string                        `json:"jobImage,omitempty"`
	Timeout         *metav1.Duration              `json:"timeout,omitempty"`
}

type HelmChartStatus struct {
	JobName string `json:"jobName,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type HelmChartConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HelmChartConfigSpec `json:"spec,omitempty"`
}

type HelmChartConfigSpec struct {
	ValuesContent string `json:"valuesContent,omitempty"`
}
