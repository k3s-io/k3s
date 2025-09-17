package util

import (
	"encoding/json"
	"time"

	"github.com/k3s-io/k3s/pkg/daemons/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetNodeCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func GetNodeCondition(status *corev1.NodeStatus, conditionType corev1.NodeConditionType) (int, *corev1.NodeCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

// SetNodeCondition updates specific node condition with patch operation.
func SetNodeCondition(core config.CoreFactory, nodeName string, condition corev1.NodeCondition) error {
	if core == nil {
		return ErrCoreNotReady
	}
	condition.LastHeartbeatTime = metav1.NewTime(time.Now())
	patch, err := json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []corev1.NodeCondition{condition},
		},
	})
	if err != nil {
		return err
	}
	_, err = core.Core().V1().Node().Patch(nodeName, types.StrategicMergePatchType, patch, "status")
	return err
}
