/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package statefulset

import (
	"context"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/features"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/api/pod"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/apps/validation"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

// statefulSetStrategy implements verification logic for Replication StatefulSets.
type statefulSetStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Replication StatefulSet objects.
var Strategy = statefulSetStrategy{legacyscheme.Scheme, names.SimpleNameGenerator}

// DefaultGarbageCollectionPolicy returns OrphanDependents for apps/v1beta1 and apps/v1beta2 for backwards compatibility,
// and DeleteDependents for all other versions.
func (statefulSetStrategy) DefaultGarbageCollectionPolicy(ctx context.Context) rest.GarbageCollectionPolicy {
	var groupVersion schema.GroupVersion
	if requestInfo, found := genericapirequest.RequestInfoFrom(ctx); found {
		groupVersion = schema.GroupVersion{Group: requestInfo.APIGroup, Version: requestInfo.APIVersion}
	}
	switch groupVersion {
	case appsv1beta1.SchemeGroupVersion, appsv1beta2.SchemeGroupVersion:
		// for back compatibility
		return rest.OrphanDependents
	default:
		return rest.DeleteDependents
	}
}

// NamespaceScoped returns true because all StatefulSet' need to be within a namespace.
func (statefulSetStrategy) NamespaceScoped() bool {
	return true
}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (statefulSetStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	fields := map[fieldpath.APIVersion]*fieldpath.Set{
		"apps/v1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}

	return fields
}

// PrepareForCreate clears the status of an StatefulSet before creation.
func (statefulSetStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	statefulSet := obj.(*apps.StatefulSet)
	// create cannot set status
	statefulSet.Status = apps.StatefulSetStatus{}

	statefulSet.Generation = 1
	dropStatefulSetDisabledFields(statefulSet, nil)
	pod.DropDisabledTemplateFields(&statefulSet.Spec.Template, nil)
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (statefulSetStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newStatefulSet := obj.(*apps.StatefulSet)
	oldStatefulSet := old.(*apps.StatefulSet)
	// Update is not allowed to set status
	newStatefulSet.Status = oldStatefulSet.Status

	dropStatefulSetDisabledFields(newStatefulSet, oldStatefulSet)
	pod.DropDisabledTemplateFields(&newStatefulSet.Spec.Template, &oldStatefulSet.Spec.Template)

	// Any changes to the spec increment the generation number, any changes to the
	// status should reflect the generation number of the corresponding object.
	// See metav1.ObjectMeta description for more information on Generation.
	if !apiequality.Semantic.DeepEqual(oldStatefulSet.Spec, newStatefulSet.Spec) {
		newStatefulSet.Generation = oldStatefulSet.Generation + 1
	}
}

// dropStatefulSetDisabledFields drops fields that are not used if their associated feature gates
// are not enabled.
// The typical pattern is:
//     if !utilfeature.DefaultFeatureGate.Enabled(features.MyFeature) && !myFeatureInUse(oldSvc) {
//         newSvc.Spec.MyFeature = nil
//     }
func dropStatefulSetDisabledFields(newSS *apps.StatefulSet, oldSS *apps.StatefulSet) {
	if !utilfeature.DefaultFeatureGate.Enabled(features.StatefulSetMinReadySeconds) {
		if !minReadySecondsFieldsInUse(oldSS) {
			newSS.Spec.MinReadySeconds = int32(0)
		}
	}
}

// minReadySecondsFieldsInUse returns true if fields related to StatefulSet minReadySeconds are set and
// are greater than 0
func minReadySecondsFieldsInUse(ss *apps.StatefulSet) bool {
	if ss == nil {
		return false
	} else if ss.Spec.MinReadySeconds >= 0 {
		return true
	}
	return false
}

// Validate validates a new StatefulSet.
func (statefulSetStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	statefulSet := obj.(*apps.StatefulSet)
	opts := pod.GetValidationOptionsFromPodTemplate(&statefulSet.Spec.Template, nil)
	return validation.ValidateStatefulSet(statefulSet, opts)
}

// WarningsOnCreate returns warnings for the creation of the given object.
func (statefulSetStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	newStatefulSet := obj.(*apps.StatefulSet)
	return pod.GetWarningsForPodTemplate(ctx, field.NewPath("spec", "template"), &newStatefulSet.Spec.Template, nil)
}

// Canonicalize normalizes the object after validation.
func (statefulSetStrategy) Canonicalize(obj runtime.Object) {
}

// AllowCreateOnUpdate is false for StatefulSet; this means POST is needed to create one.
func (statefulSetStrategy) AllowCreateOnUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (statefulSetStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	newStatefulSet := obj.(*apps.StatefulSet)
	oldStatefulSet := old.(*apps.StatefulSet)

	opts := pod.GetValidationOptionsFromPodTemplate(&newStatefulSet.Spec.Template, &oldStatefulSet.Spec.Template)
	validationErrorList := validation.ValidateStatefulSet(newStatefulSet, opts)
	updateErrorList := validation.ValidateStatefulSetUpdate(newStatefulSet, oldStatefulSet)
	return append(validationErrorList, updateErrorList...)
}

// WarningsOnUpdate returns warnings for the given update.
func (statefulSetStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	var warnings []string
	newStatefulSet := obj.(*apps.StatefulSet)
	oldStatefulSet := old.(*apps.StatefulSet)
	if newStatefulSet.Generation != oldStatefulSet.Generation {
		warnings = pod.GetWarningsForPodTemplate(ctx, field.NewPath("spec", "template"), &newStatefulSet.Spec.Template, &oldStatefulSet.Spec.Template)
	}
	return warnings
}

// AllowUnconditionalUpdate is the default update policy for StatefulSet objects.
func (statefulSetStrategy) AllowUnconditionalUpdate() bool {
	return true
}

type statefulSetStatusStrategy struct {
	statefulSetStrategy
}

// StatusStrategy is the default logic invoked when updating object status.
var StatusStrategy = statefulSetStatusStrategy{Strategy}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (statefulSetStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"apps/v1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update of status
func (statefulSetStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newStatefulSet := obj.(*apps.StatefulSet)
	oldStatefulSet := old.(*apps.StatefulSet)
	// status changes are not allowed to update spec
	newStatefulSet.Spec = oldStatefulSet.Spec
}

// ValidateUpdate is the default update validation for an end user updating status
func (statefulSetStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	// TODO: Validate status updates.
	return validation.ValidateStatefulSetStatusUpdate(obj.(*apps.StatefulSet), old.(*apps.StatefulSet))
}

// WarningsOnUpdate returns warnings for the given update.
func (statefulSetStatusStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}
