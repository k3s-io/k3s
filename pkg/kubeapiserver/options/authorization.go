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

package options

import (
	"fmt"
	"github.com/spf13/pflag"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	versionedinformers "k8s.io/client-go/informers"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer"
	authzmodes "k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
)

type BuiltInAuthorizationOptions struct {
	Modes                       []string
}

func NewBuiltInAuthorizationOptions() *BuiltInAuthorizationOptions {
	return &BuiltInAuthorizationOptions{
		Modes: []string{authzmodes.ModeAlwaysAllow},
	}
}

func (s *BuiltInAuthorizationOptions) Validate() []error {
	if s == nil {
		return nil
	}
	allErrors := []error{}

	if len(s.Modes) == 0 {
		allErrors = append(allErrors, fmt.Errorf("at least one authorization-mode must be passed"))
	}

	allowedModes := sets.NewString(authzmodes.AuthorizationModeChoices...)
	modes := sets.NewString(s.Modes...)
	for _, mode := range s.Modes {
		if !allowedModes.Has(mode) {
			allErrors = append(allErrors, fmt.Errorf("authorization-mode %q is not a valid mode", mode))
		}
	}

	if len(s.Modes) != len(modes.List()) {
		allErrors = append(allErrors, fmt.Errorf("authorization-mode %q has mode specified more than once", s.Modes))
	}

	return allErrors
}

func (s *BuiltInAuthorizationOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringSliceVar(&s.Modes, "authorization-mode", s.Modes, ""+
		"Ordered list of plug-ins to do authorization on secure port. Comma-delimited list of: "+
		strings.Join(authzmodes.AuthorizationModeChoices, ",")+".")
}

func (s *BuiltInAuthorizationOptions) ToAuthorizationConfig(versionedInformerFactory versionedinformers.SharedInformerFactory) authorizer.AuthorizationConfig {
	return authorizer.AuthorizationConfig{
		AuthorizationModes:          s.Modes,
		VersionedInformerFactory:    versionedInformerFactory,
	}
}
