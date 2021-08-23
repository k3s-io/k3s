/*
Copyright 2016 The Kubernetes Authors.

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

package storage

import (
	"context"
	"fmt"
	"reflect"
	"time"

	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/util/dryrun"
	policyclient "k8s.io/client-go/kubernetes/typed/policy/v1"
	"k8s.io/client-go/util/retry"
	pdbhelper "k8s.io/component-helpers/apps/poddisruptionbudget"
	podutil "k8s.io/kubernetes/pkg/api/pod"
	api "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/policy"
)

const (
	// MaxDisruptedPodSize is the max size of PodDisruptionBudgetStatus.DisruptedPods. API server eviction
	// subresource handler will refuse to evict pods covered by the corresponding PDB
	// if the size of the map exceeds this value. It means a large number of
	// evictions have been approved by the API server but not noticed by the PDB controller yet.
	// This situation should self-correct because the PDB controller removes
	// entries from the map automatically after the PDB DeletionTimeout regardless.
	MaxDisruptedPodSize = 2000
)

// EvictionsRetry is the retry for a conflict where multiple clients
// are making changes to the same resource.
var EvictionsRetry = wait.Backoff{
	Steps:    20,
	Duration: 500 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
}

func newEvictionStorage(store rest.StandardStorage, podDisruptionBudgetClient policyclient.PodDisruptionBudgetsGetter) *EvictionREST {
	return &EvictionREST{store: store, podDisruptionBudgetClient: podDisruptionBudgetClient}
}

// EvictionREST implements the REST endpoint for evicting pods from nodes
type EvictionREST struct {
	store                     rest.StandardStorage
	podDisruptionBudgetClient policyclient.PodDisruptionBudgetsGetter
}

var _ = rest.NamedCreater(&EvictionREST{})
var _ = rest.GroupVersionKindProvider(&EvictionREST{})
var _ = rest.GroupVersionAcceptor(&EvictionREST{})

var v1Eviction = schema.GroupVersionKind{Group: "policy", Version: "v1", Kind: "Eviction"}

// GroupVersionKind specifies a particular GroupVersionKind to discovery
func (r *EvictionREST) GroupVersionKind(containingGV schema.GroupVersion) schema.GroupVersionKind {
	return v1Eviction
}

// AcceptsGroupVersion indicates both v1 and v1beta1 Eviction objects are acceptable
func (r *EvictionREST) AcceptsGroupVersion(gv schema.GroupVersion) bool {
	switch gv {
	case policyv1.SchemeGroupVersion, policyv1beta1.SchemeGroupVersion:
		return true
	default:
		return false
	}
}

// New creates a new eviction resource
func (r *EvictionREST) New() runtime.Object {
	return &policy.Eviction{}
}

// Propagate dry-run takes the dry-run option from the request and pushes it into the eviction object.
// It returns an error if they have non-matching dry-run options.
func propagateDryRun(eviction *policy.Eviction, options *metav1.CreateOptions) (*metav1.DeleteOptions, error) {
	if eviction.DeleteOptions == nil {
		return &metav1.DeleteOptions{DryRun: options.DryRun}, nil
	}
	if len(eviction.DeleteOptions.DryRun) == 0 {
		eviction.DeleteOptions.DryRun = options.DryRun
		return eviction.DeleteOptions, nil
	}
	if len(options.DryRun) == 0 {
		return eviction.DeleteOptions, nil
	}

	if !reflect.DeepEqual(options.DryRun, eviction.DeleteOptions.DryRun) {
		return nil, fmt.Errorf("Non-matching dry-run options in request and content: %v and %v", options.DryRun, eviction.DeleteOptions.DryRun)
	}
	return eviction.DeleteOptions, nil
}

// Create attempts to create a new eviction.  That is, it tries to evict a pod.
func (r *EvictionREST) Create(ctx context.Context, name string, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	eviction, ok := obj.(*policy.Eviction)
	if !ok {
		return nil, errors.NewBadRequest(fmt.Sprintf("not a Eviction object: %T", obj))
	}

	if name != eviction.Name {
		return nil, errors.NewBadRequest("name in URL does not match name in Eviction object")
	}

	originalDeleteOptions, err := propagateDryRun(eviction, options)
	if err != nil {
		return nil, err
	}

	if createValidation != nil {
		if err := createValidation(ctx, eviction.DeepCopyObject()); err != nil {
			return nil, err
		}
	}

	var pod *api.Pod
	deletedPod := false
	// by default, retry conflict errors
	shouldRetry := errors.IsConflict
	if !resourceVersionIsUnset(originalDeleteOptions) {
		// if the original options included a resourceVersion precondition, don't retry
		shouldRetry = func(err error) bool { return false }
	}

	err = retry.OnError(EvictionsRetry, shouldRetry, func() error {
		obj, err = r.store.Get(ctx, eviction.Name, &metav1.GetOptions{})
		if err != nil {
			return err
		}
		pod = obj.(*api.Pod)

		// Evicting a terminal pod should result in direct deletion of pod as it already caused disruption by the time we are evicting.
		// There is no need to check for pdb.
		if !canIgnorePDB(pod) {
			// Pod is not in a state where we can skip checking PDBs, exit the loop, and continue to PDB checks.
			return nil
		}

		// the PDB can be ignored, so delete the pod
		deleteOptions := originalDeleteOptions

		// We should check if resourceVersion is already set by the requestor
		// as it might be older than the pod we just fetched and should be
		// honored.
		if shouldEnforceResourceVersion(pod) && resourceVersionIsUnset(originalDeleteOptions) {
			// Set deleteOptions.Preconditions.ResourceVersion to ensure we're not
			// racing with another PDB-impacting process elsewhere.
			deleteOptions = deleteOptions.DeepCopy()
			setPreconditionsResourceVersion(deleteOptions, &pod.ResourceVersion)
		}
		_, _, err = r.store.Delete(ctx, eviction.Name, rest.ValidateAllObjectFunc, deleteOptions)
		if err != nil {
			return err
		}
		deletedPod = true
		return nil
	})
	switch {
	case err != nil:
		// this can happen in cases where the PDB can be ignored, but there was a problem issuing the pod delete:
		// maybe we conflicted too many times or we didn't have permission or something else weird.
		return nil, err

	case deletedPod:
		// this happens when we successfully deleted the pod.  In this case, we're done executing because we've evicted/deleted the pod
		return &metav1.Status{Status: metav1.StatusSuccess}, nil

	default:
		// this happens when we didn't have an error and we didn't delete the pod. The only branch that happens on is when
		// we cannot ignored the PDB for this pod, so this is the fall through case.
	}

	var rtStatus *metav1.Status
	var pdbName string
	updateDeletionOptions := false

	err = func() error {
		pdbs, err := r.getPodDisruptionBudgets(ctx, pod)
		if err != nil {
			return err
		}

		if len(pdbs) > 1 {
			rtStatus = &metav1.Status{
				Status:  metav1.StatusFailure,
				Message: "This pod has more than one PodDisruptionBudget, which the eviction subresource does not support.",
				Code:    500,
			}
			return nil
		}
		if len(pdbs) == 0 {
			return nil
		}

		pdb := &pdbs[0]
		pdbName = pdb.Name

		// If the pod is not ready, it doesn't count towards healthy and we should not decrement
		if !podutil.IsPodReady(pod) && pdb.Status.CurrentHealthy >= pdb.Status.DesiredHealthy && pdb.Status.DesiredHealthy > 0 {
			updateDeletionOptions = true
			return nil
		}

		refresh := false
		err = retry.RetryOnConflict(EvictionsRetry, func() error {
			if refresh {
				pdb, err = r.podDisruptionBudgetClient.PodDisruptionBudgets(pod.Namespace).Get(context.TODO(), pdbName, metav1.GetOptions{})
				if err != nil {
					return err
				}
			}
			// Try to verify-and-decrement

			// If it was false already, or if it becomes false during the course of our retries,
			// raise an error marked as a 429.
			if err = r.checkAndDecrement(pod.Namespace, pod.Name, *pdb, dryrun.IsDryRun(originalDeleteOptions.DryRun)); err != nil {
				refresh = true
				return err
			}
			return nil
		})
		return err
	}()
	if err == wait.ErrWaitTimeout {
		err = errors.NewTimeoutError(fmt.Sprintf("couldn't update PodDisruptionBudget %q due to conflicts", pdbName), 10)
	}
	if err != nil {
		return nil, err
	}

	if rtStatus != nil {
		return rtStatus, nil
	}

	// At this point there was either no PDB or we succeeded in decrementing or
	// the pod was unready and we have enough healthy replicas

	deleteOptions := originalDeleteOptions

	// Set deleteOptions.Preconditions.ResourceVersion to ensure
	// the pod hasn't been considered ready since we calculated
	if updateDeletionOptions {
		// Take a copy so we can compare to client-provied Options later.
		deleteOptions = deleteOptions.DeepCopy()
		setPreconditionsResourceVersion(deleteOptions, &pod.ResourceVersion)
	}

	// Try the delete
	_, _, err = r.store.Delete(ctx, eviction.Name, rest.ValidateAllObjectFunc, deleteOptions)
	if err != nil {
		if errors.IsConflict(err) && updateDeletionOptions &&
			(originalDeleteOptions.Preconditions == nil || originalDeleteOptions.Preconditions.ResourceVersion == nil) {
			// If we encounter a resource conflict error, we updated the deletion options to include them,
			// and the original deletion options did not specify ResourceVersion, we send back
			// TooManyRequests so clients will retry.
			return nil, createTooManyRequestsError(pdbName)
		}
		return nil, err
	}

	// Success!
	return &metav1.Status{Status: metav1.StatusSuccess}, nil
}

func setPreconditionsResourceVersion(deleteOptions *metav1.DeleteOptions, resourceVersion *string) {
	if deleteOptions.Preconditions == nil {
		deleteOptions.Preconditions = &metav1.Preconditions{}
	}
	deleteOptions.Preconditions.ResourceVersion = resourceVersion
}

// canIgnorePDB returns true for pod conditions that allow the pod to be deleted
// without checking PDBs.
func canIgnorePDB(pod *api.Pod) bool {
	if pod.Status.Phase == api.PodSucceeded || pod.Status.Phase == api.PodFailed ||
		pod.Status.Phase == api.PodPending || !pod.ObjectMeta.DeletionTimestamp.IsZero() {
		return true
	}
	return false
}

func shouldEnforceResourceVersion(pod *api.Pod) bool {
	// We don't need to enforce ResourceVersion for terminal pods
	if pod.Status.Phase == api.PodSucceeded || pod.Status.Phase == api.PodFailed || !pod.ObjectMeta.DeletionTimestamp.IsZero() {
		return false
	}
	// Return true for all other pods to ensure we don't race against a pod becoming
	// ready and violating PDBs.
	return true
}

func resourceVersionIsUnset(options *metav1.DeleteOptions) bool {
	return options.Preconditions == nil || options.Preconditions.ResourceVersion == nil
}

func createTooManyRequestsError(name string) error {
	// TODO: Once there are time-based
	// budgets, we can sometimes compute a sensible suggested value.  But
	// even without that, we can give a suggestion (even if small) that
	// prevents well-behaved clients from hammering us.
	err := errors.NewTooManyRequests("Cannot evict pod as it would violate the pod's disruption budget.", 10)
	err.ErrStatus.Details.Causes = append(err.ErrStatus.Details.Causes, metav1.StatusCause{Type: policyv1.DisruptionBudgetCause, Message: fmt.Sprintf("The disruption budget %s is still being processed by the server.", name)})
	return err
}

// checkAndDecrement checks if the provided PodDisruptionBudget allows any disruption.
func (r *EvictionREST) checkAndDecrement(namespace string, podName string, pdb policyv1.PodDisruptionBudget, dryRun bool) error {
	if pdb.Status.ObservedGeneration < pdb.Generation {

		return createTooManyRequestsError(pdb.Name)
	}
	if pdb.Status.DisruptionsAllowed < 0 {
		return errors.NewForbidden(policy.Resource("poddisruptionbudget"), pdb.Name, fmt.Errorf("pdb disruptions allowed is negative"))
	}
	if len(pdb.Status.DisruptedPods) > MaxDisruptedPodSize {
		return errors.NewForbidden(policy.Resource("poddisruptionbudget"), pdb.Name, fmt.Errorf("DisruptedPods map too big - too many evictions not confirmed by PDB controller"))
	}
	if pdb.Status.DisruptionsAllowed == 0 {
		err := errors.NewTooManyRequests("Cannot evict pod as it would violate the pod's disruption budget.", 0)
		err.ErrStatus.Details.Causes = append(err.ErrStatus.Details.Causes, metav1.StatusCause{Type: policyv1.DisruptionBudgetCause, Message: fmt.Sprintf("The disruption budget %s needs %d healthy pods and has %d currently", pdb.Name, pdb.Status.DesiredHealthy, pdb.Status.CurrentHealthy)})
		return err
	}

	pdb.Status.DisruptionsAllowed--
	if pdb.Status.DisruptionsAllowed == 0 {
		pdbhelper.UpdateDisruptionAllowedCondition(&pdb)
	}

	// If this is a dry-run, we don't need to go any further than that.
	if dryRun == true {
		return nil
	}

	if pdb.Status.DisruptedPods == nil {
		pdb.Status.DisruptedPods = make(map[string]metav1.Time)
	}

	// Eviction handler needs to inform the PDB controller that it is about to delete a pod
	// so it should not consider it as available in calculations when updating PodDisruptions allowed.
	// If the pod is not deleted within a reasonable time limit PDB controller will assume that it won't
	// be deleted at all and remove it from DisruptedPod map.
	pdb.Status.DisruptedPods[podName] = metav1.Time{Time: time.Now()}
	if _, err := r.podDisruptionBudgetClient.PodDisruptionBudgets(namespace).UpdateStatus(context.TODO(), &pdb, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

// getPodDisruptionBudgets returns any PDBs that match the pod or err if there's an error.
func (r *EvictionREST) getPodDisruptionBudgets(ctx context.Context, pod *api.Pod) ([]policyv1.PodDisruptionBudget, error) {
	pdbList, err := r.podDisruptionBudgetClient.PodDisruptionBudgets(pod.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var pdbs []policyv1.PodDisruptionBudget
	for _, pdb := range pdbList.Items {
		if pdb.Namespace != pod.Namespace {
			continue
		}
		selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}
		// If a PDB with a nil or empty selector creeps in, it should match nothing, not everything.
		if selector.Empty() || !selector.Matches(labels.Set(pod.Labels)) {
			continue
		}

		pdbs = append(pdbs, pdb)
	}

	return pdbs, nil
}
