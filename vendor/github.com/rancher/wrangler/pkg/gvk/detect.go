package gvk

import (
	"encoding/json"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func Detect(obj []byte) (schema.GroupVersionKind, bool, error) {
	partial := v1.PartialObjectMetadata{}
	if err := json.Unmarshal(obj, &partial); err != nil {
		return schema.GroupVersionKind{}, false, err
	}

	result := partial.GetObjectKind().GroupVersionKind()
	ok := result.Kind != "" && result.Version != ""
	return result, ok, nil
}
