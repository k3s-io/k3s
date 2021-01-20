package patch

import (
	"encoding/json"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

func Apply(original, patch []byte) ([]byte, error) {
	style, metadata, err := GetPatchStyle(original, patch)
	if err != nil {
		return nil, err
	}

	switch style {
	case types.JSONPatchType:
		return applyJSONPatch(original, patch)
	case types.MergePatchType:
		return applyMergePatch(original, patch)
	case types.StrategicMergePatchType:
		return applyStrategicMergePatch(original, patch, metadata)
	default:
		return nil, fmt.Errorf("invalid patch")
	}
}

func applyStrategicMergePatch(original, patch []byte, lookup strategicpatch.LookupPatchMeta) ([]byte, error) {
	originalMap := map[string]interface{}{}
	patchMap := map[string]interface{}{}
	if err := json.Unmarshal(original, &originalMap); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(patch, &patchMap); err != nil {
		return nil, err
	}
	patchedMap, err := strategicpatch.StrategicMergeMapPatchUsingLookupPatchMeta(originalMap, patchMap, lookup)
	if err != nil {
		return nil, err
	}
	return json.Marshal(patchedMap)
}

func applyMergePatch(original, patch []byte) ([]byte, error) {
	return jsonpatch.MergePatch(original, patch)
}

func applyJSONPatch(original, patch []byte) ([]byte, error) {
	jsonPatch, err := jsonpatch.DecodePatch(patch)
	if err != nil {
		return nil, err
	}

	return jsonPatch.Apply(original)
}
