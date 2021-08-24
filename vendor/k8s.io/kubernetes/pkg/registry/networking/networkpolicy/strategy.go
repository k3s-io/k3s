/*
Copyright 2017 The Kubernetes Authors.

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

package networkpolicy

import (
	"context"
	"reflect"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/networking"
	"k8s.io/kubernetes/pkg/apis/networking/validation"
	"k8s.io/kubernetes/pkg/features"
)

// networkPolicyStrategy implements verification logic for NetworkPolicies
type networkPolicyStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the default logic that applies when creating and updating NetworkPolicy objects.
var Strategy = networkPolicyStrategy{legacyscheme.Scheme, names.SimpleNameGenerator}

// NamespaceScoped returns true because all NetworkPolicies need to be within a namespace.
func (networkPolicyStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForCreate clears the status of a NetworkPolicy before creation.
func (networkPolicyStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	networkPolicy := obj.(*networking.NetworkPolicy)
	networkPolicy.Generation = 1

	if !utilfeature.DefaultFeatureGate.Enabled(features.NetworkPolicyEndPort) {
		dropNetworkPolicyEndPort(networkPolicy)
	}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (networkPolicyStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	newNetworkPolicy := obj.(*networking.NetworkPolicy)
	oldNetworkPolicy := old.(*networking.NetworkPolicy)

	if !utilfeature.DefaultFeatureGate.Enabled(features.NetworkPolicyEndPort) && !endPortInUse(oldNetworkPolicy) {
		dropNetworkPolicyEndPort(newNetworkPolicy)
	}

	// Any changes to the spec increment the generation number, any changes to the
	// status should reflect the generation number of the corresponding object.
	// See metav1.ObjectMeta description for more information on Generation.
	if !reflect.DeepEqual(oldNetworkPolicy.Spec, newNetworkPolicy.Spec) {
		newNetworkPolicy.Generation = oldNetworkPolicy.Generation + 1
	}
}

// Validate validates a new NetworkPolicy.
func (networkPolicyStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	networkPolicy := obj.(*networking.NetworkPolicy)
	return validation.ValidateNetworkPolicy(networkPolicy)
}

// WarningsOnCreate returns warnings for the creation of the given object.
func (networkPolicyStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

// Canonicalize normalizes the object after validation.
func (networkPolicyStrategy) Canonicalize(obj runtime.Object) {}

// AllowCreateOnUpdate is false for NetworkPolicy; this means POST is needed to create one.
func (networkPolicyStrategy) AllowCreateOnUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (networkPolicyStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	validationErrorList := validation.ValidateNetworkPolicy(obj.(*networking.NetworkPolicy))
	updateErrorList := validation.ValidateNetworkPolicyUpdate(obj.(*networking.NetworkPolicy), old.(*networking.NetworkPolicy))
	return append(validationErrorList, updateErrorList...)
}

// WarningsOnUpdate returns warnings for the given update.
func (networkPolicyStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

// AllowUnconditionalUpdate is the default update policy for NetworkPolicy objects.
func (networkPolicyStrategy) AllowUnconditionalUpdate() bool {
	return true
}

// Drops Network Policy EndPort fields if Feature Gate is also disabled.
// This should be used in future Network Policy evolutions
func dropNetworkPolicyEndPort(netPol *networking.NetworkPolicy) {
	for idx, ingressSpec := range netPol.Spec.Ingress {
		for idxPort, port := range ingressSpec.Ports {
			if port.EndPort != nil {
				netPol.Spec.Ingress[idx].Ports[idxPort].EndPort = nil
			}
		}
	}

	for idx, egressSpec := range netPol.Spec.Egress {
		for idxPort, port := range egressSpec.Ports {
			if port.EndPort != nil {
				netPol.Spec.Egress[idx].Ports[idxPort].EndPort = nil
			}
		}
	}
}

func endPortInUse(netPol *networking.NetworkPolicy) bool {
	for _, ingressSpec := range netPol.Spec.Ingress {
		for _, port := range ingressSpec.Ports {
			if port.EndPort != nil {
				return true
			}
		}
	}

	for _, egressSpec := range netPol.Spec.Egress {
		for _, port := range egressSpec.Ports {
			if port.EndPort != nil {
				return true
			}
		}
	}
	return false
}
