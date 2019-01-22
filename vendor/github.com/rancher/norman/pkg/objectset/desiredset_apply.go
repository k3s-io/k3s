package objectset

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"

	"github.com/pkg/errors"
	errors2 "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/util/flowcontrol"
)

const (
	LabelID        = "objectset.rio.cattle.io/id"
	LabelGVK       = "objectset.rio.cattle.io/owner-gvk"
	LabelName      = "objectset.rio.cattle.io/owner-name"
	LabelNamespace = "objectset.rio.cattle.io/owner-namespace"
	LabelHash      = "objectset.rio.cattle.io/hash"
)

var (
	hashOrder = []string{
		LabelID,
		LabelGVK,
		LabelName,
		LabelNamespace,
	}
	rls     = map[string]flowcontrol.RateLimiter{}
	rlsLock sync.Mutex
)

func (o *DesiredSet) getRateLimit(inputID string) flowcontrol.RateLimiter {
	var rl flowcontrol.RateLimiter

	rlsLock.Lock()
	defer rlsLock.Unlock()
	if o.remove {
		delete(rls, inputID)
	} else {
		rl = rls[inputID]
		if rl == nil {
			rl = flowcontrol.NewTokenBucketRateLimiter(4.0/60.0, 10)
			rls[inputID] = rl
		}
	}

	return rl
}

func (o *DesiredSet) Apply() error {
	if err := o.Err(); err != nil {
		return err
	}

	labelSet, annotationSet, err := o.getLabelsAndAnnotations()
	if err != nil {
		return o.err(err)
	}

	rl := o.getRateLimit(labelSet[LabelHash])
	if rl != nil && !rl.TryAccept() {
		return errors2.NewConflict(schema.GroupResource{}, o.setID, errors.New("delaying object set"))
	}

	inputID := o.inputID(labelSet[LabelHash])

	objList, err := o.injectLabelsAndAnnotations(labelSet, annotationSet)
	if err != nil {
		return o.err(err)
	}

	objList, err = o.runInjectors(objList)
	if err != nil {
		return o.err(err)
	}

	objs := o.collect(objList)

	debugID := o.debugID()
	req, err := labels.NewRequirement(LabelHash, selection.Equals, []string{labelSet[LabelHash]})
	if err != nil {
		return o.err(err)
	}

	for _, gvk := range o.gvkOrder() {
		o.process(inputID, debugID, labels.NewSelector().Add(*req), gvk, objs[gvk])
	}

	return o.Err()
}

func (o *DesiredSet) gvkOrder() []schema.GroupVersionKind {
	seen := map[schema.GroupVersionKind]bool{}
	var gvkOrder []schema.GroupVersionKind

	for _, obj := range o.objs.order {
		if seen[obj.GetObjectKind().GroupVersionKind()] {
			continue
		}
		seen[obj.GetObjectKind().GroupVersionKind()] = true
		gvkOrder = append(gvkOrder, obj.GetObjectKind().GroupVersionKind())
	}

	var rest []schema.GroupVersionKind

	for gvk := range o.clients {
		if seen[gvk] {
			continue
		}

		seen[gvk] = true
		rest = append(rest, gvk)
	}

	sort.Slice(rest, func(i, j int) bool {
		return rest[i].String() < rest[j].String()
	})

	return append(gvkOrder, rest...)
}

func (o *DesiredSet) inputID(labelHash string) string {
	sort.Slice(o.objs.inputs, func(i, j int) bool {
		left, lErr := meta.Accessor(o.objs.inputs[i])
		right, rErr := meta.Accessor(o.objs.inputs[j])
		if lErr != nil || rErr != nil {
			return true
		}

		lKey := o.objs.inputs[i].GetObjectKind().GroupVersionKind().String() + "/" + newObjectKey(left).String()
		rKey := o.objs.inputs[j].GetObjectKind().GroupVersionKind().String() + "/" + newObjectKey(right).String()
		return lKey < rKey
	})

	dig := sha1.New()
	dig.Write([]byte(o.codeVersion))
	dig.Write([]byte(labelHash))

	inputs := o.objs.inputs
	if o.owner != nil {
		inputs = append([]runtime.Object{o.owner}, o.objs.inputs...)
	}

	for _, obj := range inputs {
		metadata, err := meta.Accessor(obj)
		if err != nil {
			dig.Write([]byte(obj.GetObjectKind().GroupVersionKind().String()))
			continue
		}

		key := newObjectKey(metadata)
		dig.Write([]byte(key.String()))
		dig.Write([]byte(metadata.GetResourceVersion()))
	}

	return hex.EncodeToString(dig.Sum(nil))
}

func (o *DesiredSet) debugID() string {
	if o.owner == nil {
		return o.setID
	}
	metadata, err := meta.Accessor(o.owner)
	if err != nil {
		return o.setID
	}

	return fmt.Sprintf("%s %s", o.setID, objectKey{
		namespace: metadata.GetNamespace(),
		name:      metadata.GetName(),
	})
}

func (o *DesiredSet) collect(objList []runtime.Object) objectCollection {
	result := objectCollection{}
	for _, obj := range objList {
		result.add(obj)
	}
	return result
}

func (o *DesiredSet) runInjectors(objList []runtime.Object) ([]runtime.Object, error) {
	var err error

	for _, inj := range o.injectors {
		if inj == nil {
			continue
		}

		objList, err = inj(objList)
		if err != nil {
			return nil, err
		}
	}

	return objList, nil
}

func (o *DesiredSet) getLabelsAndAnnotations() (map[string]string, map[string]string, error) {
	annotations := map[string]string{
		LabelID: o.setID,
	}

	if o.owner != nil {
		annotations[LabelGVK] = o.owner.GetObjectKind().GroupVersionKind().String()
		metadata, err := meta.Accessor(o.owner)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get metadata for %s", o.owner.GetObjectKind().GroupVersionKind())
		}
		annotations[LabelName] = metadata.GetName()
		annotations[LabelNamespace] = metadata.GetNamespace()
	}

	labels := map[string]string{
		LabelHash: objectSetHash(annotations),
	}

	return labels, annotations, nil
}

func (o *DesiredSet) injectLabelsAndAnnotations(labels, annotations map[string]string) ([]runtime.Object, error) {
	var result []runtime.Object

	for _, objMap := range o.objs.objects {
		for key, obj := range objMap {
			obj = obj.DeepCopyObject()
			meta, err := meta.Accessor(obj)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get metadata for %s", key)
			}

			setLabels(meta, labels)
			setAnnotations(meta, annotations)

			result = append(result, obj)
		}
	}

	return result, nil
}

func setAnnotations(meta metav1.Object, annotations map[string]string) {
	objAnn := meta.GetAnnotations()
	if objAnn == nil {
		objAnn = map[string]string{}
	}
	delete(objAnn, LabelInputID)
	delete(objAnn, LabelApplied)
	for k, v := range annotations {
		objAnn[k] = v
	}
	meta.SetAnnotations(objAnn)
}

func setLabels(meta metav1.Object, labels map[string]string) {
	objLabels := meta.GetLabels()
	if objLabels == nil {
		objLabels = map[string]string{}
	}
	for k, v := range labels {
		objLabels[k] = v
	}
	meta.SetLabels(objLabels)
}

func objectSetHash(labels map[string]string) string {
	dig := sha1.New()
	for _, key := range hashOrder {
		dig.Write([]byte(labels[key]))
	}
	return hex.EncodeToString(dig.Sum(nil))
}
