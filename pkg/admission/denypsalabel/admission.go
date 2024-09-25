/*
Copyright 2020 The Kubernetes Authors.

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

package denypsalabel

import (
	"context"
	"fmt"
	"io"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/apis/core"
)

const (
	// PluginName is the name of this admission controller plugin
	PluginName     = "DenyPSALabel"
	PSALabelPrefix = "pod-security.kubernetes.io"
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register(PluginName, func(config io.Reader) (admission.Interface, error) {
		plugin := newPlugin()
		return plugin, nil
	})
}

// psaLabelDenialPlugin holds state for and implements the admission plugin.
type psaLabelDenialPlugin struct {
	*admission.Handler
}

var _ admission.Interface = &psaLabelDenialPlugin{}
var _ admission.ValidationInterface = &psaLabelDenialPlugin{}

// newPlugin creates a new admission plugin.
func newPlugin() *psaLabelDenialPlugin {
	return &psaLabelDenialPlugin{
		Handler: admission.NewHandler(admission.Create, admission.Update),
	}
}

// Validate ensures that applying PSA label to namespaces is denied
func (plug *psaLabelDenialPlugin) Validate(ctx context.Context, attr admission.Attributes, o admission.ObjectInterfaces) error {
	if attr.GetResource().GroupResource() != core.Resource("namespaces") {
		return nil
	}

	if len(attr.GetSubresource()) != 0 {
		return nil
	}

	// if we can't convert then we don't handle this object so just return
	newNS, ok := attr.GetObject().(*core.Namespace)
	if !ok {
		klog.V(3).Infof("Expected namespace resource, got: %v", attr.GetKind())
		return errors.NewInternalError(fmt.Errorf("expected namespace resource, got: %v", attr.GetKind()))
	}

	if !isPSALabel(newNS) {
		return nil
	}

	klog.V(4).Infof("Denying use of PSA label on namespace %s", newNS.Name)
	return admission.NewForbidden(attr, fmt.Errorf("denying use of PSA label on namespace"))
}

func isPSALabel(newNS *core.Namespace) bool {
	for labelName := range newNS.Labels {
		if strings.HasPrefix(labelName, PSALabelPrefix) {
			return true
		}
	}
	return false
}
