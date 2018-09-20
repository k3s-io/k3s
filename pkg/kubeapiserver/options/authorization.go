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
	"strings"

	"github.com/spf13/pflag"

	versionedinformers "k8s.io/client-go/informers"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer"
	authzmodes "k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
)

type BuiltInAuthorizationOptions struct {
	Mode string
}

func NewBuiltInAuthorizationOptions() *BuiltInAuthorizationOptions {
	return &BuiltInAuthorizationOptions{
		Mode: authzmodes.ModeNode + "," + authzmodes.ModeAlwaysAllow,
	}
}

func (s *BuiltInAuthorizationOptions) Validate() []error {
	allErrors := []error{}
	return allErrors
}

func (s *BuiltInAuthorizationOptions) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&s.Mode, "authorization-mode", s.Mode, ""+
		"Ordered list of plug-ins to do authorization on secure port. Comma-delimited list of: "+
		strings.Join(authzmodes.AuthorizationModeChoices, ",")+".")

	fs.String("authorization-rbac-super-user", "", ""+
		"If specified, a username which avoids RBAC authorization checks and role binding "+
		"privilege escalation checks, to be used with --authorization-mode=RBAC.")
	fs.MarkDeprecated("authorization-rbac-super-user", "Removed during alpha to beta.  The 'system:masters' group has privileged access.")

}

func (s *BuiltInAuthorizationOptions) Modes() []string {
	modes := []string{}
	if len(s.Mode) > 0 {
		modes = strings.Split(s.Mode, ",")
	}
	return modes
}

func (s *BuiltInAuthorizationOptions) ToAuthorizationConfig(informerFactory informers.SharedInformerFactory, versionedInformerFactory versionedinformers.SharedInformerFactory) authorizer.AuthorizationConfig {
	return authorizer.AuthorizationConfig{
		AuthorizationModes:       s.Modes(),
		InformerFactory:          informerFactory,
		VersionedInformerFactory: versionedInformerFactory,
	}
}
