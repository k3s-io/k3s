/*
Copyright 2015 The Kubernetes Authors.

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

package job

import (
	"context"
	"fmt"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/api/pod"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/batch/validation"
	"k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/features"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

// jobStrategy implements verification logic for Replication Controllers.
type jobStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Replication Controller objects.
var Strategy = jobStrategy{legacyscheme.Scheme, names.SimpleNameGenerator}

// DefaultGarbageCollectionPolicy returns OrphanDependents for batch/v1 for backwards compatibility,
// and DeleteDependents for all other versions.
func (jobStrategy) DefaultGarbageCollectionPolicy(ctx context.Context) rest.GarbageCollectionPolicy {
	var groupVersion schema.GroupVersion
	if requestInfo, found := genericapirequest.RequestInfoFrom(ctx); found {
		groupVersion = schema.GroupVersion{Group: requestInfo.APIGroup, Version: requestInfo.APIVersion}
	}
	switch groupVersion {
	case batchv1.SchemeGroupVersion:
		// for back compatibility
		return rest.OrphanDependents
	default:
		return rest.DeleteDependents
	}
}

// NamespaceScoped returns true because all jobs need to be within a namespace.
func (jobStrategy) NamespaceScoped() bool {
	return true
}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (jobStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	fields := map[fieldpath.APIVersion]*fieldpath.Set{
		"batch/v1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}

	return fields
}

// PrepareForCreate clears the status of a job before creation.
func (jobStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	job := obj.(*batch.Job)
	job.Status = batch.JobStatus{}

	job.Generation = 1

	if !utilfeature.DefaultFeatureGate.Enabled(features.TTLAfterFinished) {
		job.Spec.TTLSecondsAfterFinished = nil
	}

	if !utilfeature.DefaultFeatureGate.Enabled(features.IndexedJob) {
		job.Spec.CompletionMode = nil
	}

	if !utilfeature.DefaultFeatureGate.Enabled(features.SuspendJob) {
		job.Spec.Suspend = nil
	}

	if utilfeature.DefaultFeatureGate.Enabled(features.JobTrackingWithFinalizers) {
		// Until this feature graduates to GA and soaks in clusters, we use an
		// annotation to mark whether jobs are tracked with it.
		addJobTrackingAnnotation(job)
	} else {
		dropJobTrackingAnnotation(job)
	}

	pod.DropDisabledTemplateFields(&job.Spec.Template, nil)
}

func addJobTrackingAnnotation(job *batch.Job) {
	if job.Annotations == nil {
		job.Annotations = map[string]string{}
	}
	job.Annotations[batchv1.JobTrackingFinalizer] = ""
}

func hasJobTrackingAnnotation(job *batch.Job) bool {
	if job.Annotations == nil {
		return false
	}
	_, ok := job.Annotations[batchv1.JobTrackingFinalizer]
	return ok
}

func dropJobTrackingAnnotation(job *batch.Job) {
	delete(job.Annotations, batchv1.JobTrackingFinalizer)
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (jobStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newJob := obj.(*batch.Job)
	oldJob := old.(*batch.Job)
	newJob.Status = oldJob.Status

	if !utilfeature.DefaultFeatureGate.Enabled(features.TTLAfterFinished) && oldJob.Spec.TTLSecondsAfterFinished == nil {
		newJob.Spec.TTLSecondsAfterFinished = nil
	}

	if !utilfeature.DefaultFeatureGate.Enabled(features.IndexedJob) && oldJob.Spec.CompletionMode == nil {
		newJob.Spec.CompletionMode = nil
	}

	if !utilfeature.DefaultFeatureGate.Enabled(features.SuspendJob) {
		// There are 3 possible values (nil, true, false) for each flag, so 9
		// combinations. We want to disallow everything except true->false and
		// true->nil when the feature gate is disabled. Or, basically allow this
		// only when oldJob is true.
		if oldJob.Spec.Suspend == nil || !*oldJob.Spec.Suspend {
			newJob.Spec.Suspend = oldJob.Spec.Suspend
		}
	}
	if !utilfeature.DefaultFeatureGate.Enabled(features.JobTrackingWithFinalizers) && !hasJobTrackingAnnotation(oldJob) {
		dropJobTrackingAnnotation(newJob)
	}

	pod.DropDisabledTemplateFields(&newJob.Spec.Template, &oldJob.Spec.Template)

	// Any changes to the spec increment the generation number.
	// See metav1.ObjectMeta description for more information on Generation.
	if !apiequality.Semantic.DeepEqual(newJob.Spec, oldJob.Spec) {
		newJob.Generation = oldJob.Generation + 1
	}
}

// Validate validates a new job.
func (jobStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	job := obj.(*batch.Job)
	// TODO: move UID generation earlier and do this in defaulting logic?
	if job.Spec.ManualSelector == nil || *job.Spec.ManualSelector == false {
		generateSelector(job)
	}
	opts := validationOptionsForJob(job, nil)
	return validation.ValidateJob(job, opts)
}

func validationOptionsForJob(newJob, oldJob *batch.Job) validation.JobValidationOptions {
	var newPodTemplate, oldPodTemplate *core.PodTemplateSpec
	if newJob != nil {
		newPodTemplate = &newJob.Spec.Template
	}
	if oldJob != nil {
		oldPodTemplate = &oldJob.Spec.Template
	}
	opts := validation.JobValidationOptions{
		PodValidationOptions:    pod.GetValidationOptionsFromPodTemplate(newPodTemplate, oldPodTemplate),
		AllowTrackingAnnotation: utilfeature.DefaultFeatureGate.Enabled(features.JobTrackingWithFinalizers),
	}
	if oldJob != nil {
		// Because we don't support the tracking with finalizers for already
		// existing jobs, we allow the annotation only if the Job already had it,
		// regardless of the feature gate.
		opts.AllowTrackingAnnotation = hasJobTrackingAnnotation(oldJob)
	}
	return opts
}

// WarningsOnCreate returns warnings for the creation of the given object.
func (jobStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	newJob := obj.(*batch.Job)
	return pod.GetWarningsForPodTemplate(ctx, field.NewPath("spec", "template"), &newJob.Spec.Template, nil)
}

// generateSelector adds a selector to a job and labels to its template
// which can be used to uniquely identify the pods created by that job,
// if the user has requested this behavior.
func generateSelector(obj *batch.Job) {
	if obj.Spec.Template.Labels == nil {
		obj.Spec.Template.Labels = make(map[string]string)
	}
	// The job-name label is unique except in cases that are expected to be
	// quite uncommon, and is more user friendly than uid.  So, we add it as
	// a label.
	_, found := obj.Spec.Template.Labels["job-name"]
	if found {
		// User asked us to not automatically generate a selector and labels,
		// but set a possibly conflicting value.  If there is a conflict,
		// we will reject in validation.
	} else {
		obj.Spec.Template.Labels["job-name"] = string(obj.ObjectMeta.Name)
	}
	// The controller-uid label makes the pods that belong to this job
	// only match this job.
	_, found = obj.Spec.Template.Labels["controller-uid"]
	if found {
		// User asked us to automatically generate a selector and labels,
		// but set a possibly conflicting value.  If there is a conflict,
		// we will reject in validation.
	} else {
		obj.Spec.Template.Labels["controller-uid"] = string(obj.ObjectMeta.UID)
	}
	// Select the controller-uid label.  This is sufficient for uniqueness.
	if obj.Spec.Selector == nil {
		obj.Spec.Selector = &metav1.LabelSelector{}
	}
	if obj.Spec.Selector.MatchLabels == nil {
		obj.Spec.Selector.MatchLabels = make(map[string]string)
	}
	if _, found := obj.Spec.Selector.MatchLabels["controller-uid"]; !found {
		obj.Spec.Selector.MatchLabels["controller-uid"] = string(obj.ObjectMeta.UID)
	}
	// If the user specified matchLabel controller-uid=$WRONGUID, then it should fail
	// in validation, either because the selector does not match the pod template
	// (controller-uid=$WRONGUID does not match controller-uid=$UID, which we applied
	// above, or we will reject in validation because the template has the wrong
	// labels.
}

// TODO: generalize generateSelector so it can work for other controller
// objects such as ReplicaSet.  Can use pkg/api/meta to generically get the
// UID, but need some way to generically access the selector and pod labels
// fields.

// Canonicalize normalizes the object after validation.
func (jobStrategy) Canonicalize(obj runtime.Object) {
}

func (jobStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// AllowCreateOnUpdate is false for jobs; this means a POST is needed to create one.
func (jobStrategy) AllowCreateOnUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (jobStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	job := obj.(*batch.Job)
	oldJob := old.(*batch.Job)

	opts := validationOptionsForJob(job, oldJob)
	validationErrorList := validation.ValidateJob(job, opts)
	updateErrorList := validation.ValidateJobUpdate(job, oldJob, opts.PodValidationOptions)
	return append(validationErrorList, updateErrorList...)
}

// WarningsOnUpdate returns warnings for the given update.
func (jobStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	var warnings []string
	newJob := obj.(*batch.Job)
	oldJob := old.(*batch.Job)
	if newJob.Generation != oldJob.Generation {
		warnings = pod.GetWarningsForPodTemplate(ctx, field.NewPath("spec", "template"), &newJob.Spec.Template, &oldJob.Spec.Template)
	}
	return warnings
}

type jobStatusStrategy struct {
	jobStrategy
}

var StatusStrategy = jobStatusStrategy{Strategy}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user.
func (jobStatusStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	return map[fieldpath.APIVersion]*fieldpath.Set{
		"batch/v1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("spec"),
		),
	}
}

func (jobStatusStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newJob := obj.(*batch.Job)
	oldJob := old.(*batch.Job)
	newJob.Spec = oldJob.Spec
}

func (jobStatusStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return validation.ValidateJobUpdateStatus(obj.(*batch.Job), old.(*batch.Job))
}

// WarningsOnUpdate returns warnings for the given update.
func (jobStatusStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

// JobSelectableFields returns a field set that represents the object for matching purposes.
func JobToSelectableFields(job *batch.Job) fields.Set {
	objectMetaFieldsSet := generic.ObjectMetaFieldsSet(&job.ObjectMeta, true)
	specificFieldsSet := fields.Set{
		"status.successful": strconv.Itoa(int(job.Status.Succeeded)),
	}
	return generic.MergeFieldsSets(objectMetaFieldsSet, specificFieldsSet)
}

// GetAttrs returns labels and fields of a given object for filtering purposes.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	job, ok := obj.(*batch.Job)
	if !ok {
		return nil, nil, fmt.Errorf("given object is not a job.")
	}
	return labels.Set(job.ObjectMeta.Labels), JobToSelectableFields(job), nil
}

// MatchJob is the filter used by the generic etcd backend to route
// watch events from etcd to clients of the apiserver only interested in specific
// labels/fields.
func MatchJob(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}
