/*
Copyright 2021 The Kubernetes Authors.

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

package ensurer

import (
	"context"
	"errors"
	"fmt"

	flowcontrolv1beta1 "k8s.io/api/flowcontrol/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	flowcontrolclient "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta1"
	flowcontrolapisv1beta1 "k8s.io/kubernetes/pkg/apis/flowcontrol/v1beta1"
)

var (
	errObjectNotPriorityLevel = errors.New("object is not a PriorityLevelConfiguration type")
)

// PriorityLevelEnsurer ensures the specified bootstrap configuration objects
type PriorityLevelEnsurer interface {
	Ensure([]*flowcontrolv1beta1.PriorityLevelConfiguration) error
}

// PriorityLevelRemover removes the specified bootstrap configuration objects
type PriorityLevelRemover interface {
	Remove([]string) error
}

// NewSuggestedPriorityLevelEnsurerEnsurer returns a PriorityLevelEnsurer instance that
// can be used to ensure a set of suggested PriorityLevelConfiguration configuration objects.
func NewSuggestedPriorityLevelEnsurerEnsurer(client flowcontrolclient.PriorityLevelConfigurationInterface) PriorityLevelEnsurer {
	wrapper := &priorityLevelConfigurationWrapper{
		client: client,
	}
	return &plEnsurer{
		strategy: newSuggestedEnsureStrategy(wrapper),
		wrapper:  wrapper,
	}
}

// NewMandatoryPriorityLevelEnsurer returns a PriorityLevelEnsurer instance that
// can be used to ensure a set of mandatory PriorityLevelConfiguration configuration objects.
func NewMandatoryPriorityLevelEnsurer(client flowcontrolclient.PriorityLevelConfigurationInterface) PriorityLevelEnsurer {
	wrapper := &priorityLevelConfigurationWrapper{
		client: client,
	}
	return &plEnsurer{
		strategy: newMandatoryEnsureStrategy(wrapper),
		wrapper:  wrapper,
	}
}

// NewPriorityLevelRemover returns a PriorityLevelRemover instance that
// can be used to remove a set of PriorityLevelConfiguration configuration objects.
func NewPriorityLevelRemover(client flowcontrolclient.PriorityLevelConfigurationInterface) PriorityLevelRemover {
	return &plEnsurer{
		wrapper: &priorityLevelConfigurationWrapper{
			client: client,
		},
	}
}

// GetPriorityLevelRemoveCandidate returns a list of PriorityLevelConfiguration
// names that are candidates for removal from the cluster.
// bootstrap: a set of hard coded PriorityLevelConfiguration configuration
// objects kube-apiserver maintains in-memory.
func GetPriorityLevelRemoveCandidate(client flowcontrolclient.PriorityLevelConfigurationInterface, bootstrap []*flowcontrolv1beta1.PriorityLevelConfiguration) ([]string, error) {
	// TODO(101667): Use a lister here to avoid periodic LIST calls
	plList, err := client.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list PriorityLevelConfiguration - %w", err)
	}

	bootstrapNames := sets.String{}
	for i := range bootstrap {
		bootstrapNames.Insert(bootstrap[i].GetName())
	}

	currentObjects := make([]metav1.Object, len(plList.Items))
	for i := range plList.Items {
		currentObjects[i] = &plList.Items[i]
	}

	return getRemoveCandidate(bootstrapNames, currentObjects), nil
}

type plEnsurer struct {
	strategy ensureStrategy
	wrapper  configurationWrapper
}

func (e *plEnsurer) Ensure(priorityLevels []*flowcontrolv1beta1.PriorityLevelConfiguration) error {
	for _, priorityLevel := range priorityLevels {
		if err := ensureConfiguration(e.wrapper, e.strategy, priorityLevel); err != nil {
			return err
		}
	}

	return nil
}

func (e *plEnsurer) Remove(priorityLevels []string) error {
	for _, priorityLevel := range priorityLevels {
		if err := removeConfiguration(e.wrapper, priorityLevel); err != nil {
			return err
		}
	}

	return nil
}

// priorityLevelConfigurationWrapper abstracts all PriorityLevelConfiguration specific logic,
// with this we can manage all boiler plate code in one place.
type priorityLevelConfigurationWrapper struct {
	client flowcontrolclient.PriorityLevelConfigurationInterface
}

func (fs *priorityLevelConfigurationWrapper) TypeName() string {
	return "PriorityLevelConfiguration"
}

func (fs *priorityLevelConfigurationWrapper) Create(object runtime.Object) (runtime.Object, error) {
	plObject, ok := object.(*flowcontrolv1beta1.PriorityLevelConfiguration)
	if !ok {
		return nil, errObjectNotPriorityLevel
	}

	return fs.client.Create(context.TODO(), plObject, metav1.CreateOptions{FieldManager: fieldManager})
}

func (fs *priorityLevelConfigurationWrapper) Update(object runtime.Object) (runtime.Object, error) {
	fsObject, ok := object.(*flowcontrolv1beta1.PriorityLevelConfiguration)
	if !ok {
		return nil, errObjectNotPriorityLevel
	}

	return fs.client.Update(context.TODO(), fsObject, metav1.UpdateOptions{FieldManager: fieldManager})
}

func (fs *priorityLevelConfigurationWrapper) Get(name string) (configurationObject, error) {
	return fs.client.Get(context.TODO(), name, metav1.GetOptions{})
}

func (fs *priorityLevelConfigurationWrapper) Delete(name string) error {
	return fs.client.Delete(context.TODO(), name, metav1.DeleteOptions{})
}

func (fs *priorityLevelConfigurationWrapper) CopySpec(bootstrap, current runtime.Object) error {
	bootstrapFS, ok := bootstrap.(*flowcontrolv1beta1.PriorityLevelConfiguration)
	if !ok {
		return errObjectNotPriorityLevel
	}
	currentFS, ok := current.(*flowcontrolv1beta1.PriorityLevelConfiguration)
	if !ok {
		return errObjectNotPriorityLevel
	}

	specCopy := bootstrapFS.Spec.DeepCopy()
	currentFS.Spec = *specCopy
	return nil
}

func (fs *priorityLevelConfigurationWrapper) HasSpecChanged(bootstrap, current runtime.Object) (bool, error) {
	bootstrapFS, ok := bootstrap.(*flowcontrolv1beta1.PriorityLevelConfiguration)
	if !ok {
		return false, errObjectNotPriorityLevel
	}
	currentFS, ok := current.(*flowcontrolv1beta1.PriorityLevelConfiguration)
	if !ok {
		return false, errObjectNotPriorityLevel
	}

	return priorityLevelSpecChanged(bootstrapFS, currentFS), nil
}

func priorityLevelSpecChanged(expected, actual *flowcontrolv1beta1.PriorityLevelConfiguration) bool {
	copiedExpectedPriorityLevel := expected.DeepCopy()
	flowcontrolapisv1beta1.SetObjectDefaults_PriorityLevelConfiguration(copiedExpectedPriorityLevel)
	return !equality.Semantic.DeepEqual(copiedExpectedPriorityLevel.Spec, actual.Spec)
}
