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

package options

// This file exists to force the desired plugin implementations to be linked.
// This should probably be part of some configuration fed into the build for a
// given binary target.
import (
	// Admission policies
	"k8s.io/kubernetes/plugin/pkg/admission/defaulttolerationseconds"
	"k8s.io/kubernetes/plugin/pkg/admission/limitranger"
	"k8s.io/kubernetes/plugin/pkg/admission/noderestriction"
	"k8s.io/kubernetes/plugin/pkg/admission/nodetaint"
	podpriority "k8s.io/kubernetes/plugin/pkg/admission/priority"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"
	"k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"
	"k8s.io/kubernetes/plugin/pkg/admission/storage/persistentvolume/resize"
	"k8s.io/kubernetes/plugin/pkg/admission/storage/storageclass/setdefault"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/admission/plugin/namespace/lifecycle"
	mutatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/mutating"
	validatingwebhook "k8s.io/apiserver/pkg/admission/plugin/webhook/validating"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/features"
)

// AllOrderedPlugins is the list of all the plugins in order.
var AllOrderedPlugins = []string{
	lifecycle.PluginName,                    // NamespaceLifecycle
	limitranger.PluginName,                  // LimitRanger
	serviceaccount.PluginName,               // ServiceAccount
	noderestriction.PluginName,              // NodeRestriction
	nodetaint.PluginName,                    // TaintNodesByCondition
	podpriority.PluginName,                  // Priority
	defaulttolerationseconds.PluginName,     // DefaultTolerationSeconds
	setdefault.PluginName,                   // DefaultStorageClass
	resize.PluginName,                       // PersistentVolumeClaimResize
	mutatingwebhook.PluginName,              // MutatingAdmissionWebhook
	validatingwebhook.PluginName,            // ValidatingAdmissionWebhook
	resourcequota.PluginName,                // ResourceQuota
}

// RegisterAllAdmissionPlugins registers all admission plugins and
// sets the recommended plugins order.
func RegisterAllAdmissionPlugins(plugins *admission.Plugins) {
	defaulttolerationseconds.Register(plugins)
	limitranger.Register(plugins)
	noderestriction.Register(plugins)
	nodetaint.Register(plugins)
	resourcequota.Register(plugins)
	podpriority.Register(plugins)
	serviceaccount.Register(plugins)
	setdefault.Register(plugins)
	resize.Register(plugins)
}

// DefaultOffAdmissionPlugins get admission plugins off by default for kube-apiserver.
func DefaultOffAdmissionPlugins() sets.String {
	defaultOnPlugins := sets.NewString(
		lifecycle.PluginName,                //NamespaceLifecycle
		limitranger.PluginName,              //LimitRanger
		serviceaccount.PluginName,           //ServiceAccount
		setdefault.PluginName,               //DefaultStorageClass
		resize.PluginName,                   //PersistentVolumeClaimResize
		defaulttolerationseconds.PluginName, //DefaultTolerationSeconds
		mutatingwebhook.PluginName,          //MutatingAdmissionWebhook
		validatingwebhook.PluginName,        //ValidatingAdmissionWebhook
		resourcequota.PluginName,            //ResourceQuota
	)

	if utilfeature.DefaultFeatureGate.Enabled(features.PodPriority) {
		defaultOnPlugins.Insert(podpriority.PluginName) //PodPriority
	}

	if utilfeature.DefaultFeatureGate.Enabled(features.TaintNodesByCondition) {
		defaultOnPlugins.Insert(nodetaint.PluginName) //TaintNodesByCondition
	}

	return sets.NewString(AllOrderedPlugins...).Difference(defaultOnPlugins)
}
