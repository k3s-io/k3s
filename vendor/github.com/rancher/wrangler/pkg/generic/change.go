package generic

import (
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
)

func UpdateOnChange(updater Updater, handler Handler) Handler {
	return func(key string, obj runtime.Object) (runtime.Object, error) {
		if obj == nil {
			return handler(key, nil)
		}

		copyObj := obj.DeepCopyObject()
		newObj, err := handler(key, copyObj)
		if newObj != nil {
			copyObj = newObj
		}

		oldMeta, ignoreErr := meta.Accessor(obj)
		if ignoreErr != nil {
			return copyObj, err
		}

		newMeta, ignoreErr := meta.Accessor(copyObj)
		if ignoreErr != nil {
			return copyObj, err
		}

		if oldMeta.GetResourceVersion() == newMeta.GetResourceVersion() && !equality.Semantic.DeepEqual(obj, copyObj) {
			newObj, err := updater(copyObj)
			if newObj != nil && err == nil {
				copyObj = newObj
			}
		}

		return copyObj, err
	}
}
