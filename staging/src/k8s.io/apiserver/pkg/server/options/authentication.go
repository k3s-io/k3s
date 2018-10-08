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
	"time"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/authenticatorfactory"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// DelegatingAuthenticationOptions provides an easy way for composing API servers to delegate their authentication to
// the root kube API server.  The API federator will act as
// a front proxy and direction connections will be able to delegate to the core kube API server
type DelegatingAuthenticationOptions struct {
	// RemoteKubeConfigFile is the file to use to connect to a "normal" kube API server which hosts the
	// TokenAccessReview.authentication.k8s.io endpoint for checking tokens.
	RemoteKubeConfigFile string
	// RemoteKubeConfigFileOptional is specifying whether not specifying the kubeconfig or
	// a missing in-cluster config will be fatal.
	RemoteKubeConfigFileOptional bool

	// CacheTTL is the length of time that a token authentication answer will be cached.
	CacheTTL time.Duration

	SkipInClusterLookup bool
}

func NewDelegatingAuthenticationOptions() *DelegatingAuthenticationOptions {
	return &DelegatingAuthenticationOptions{
		// very low for responsiveness, but high enough to handle storms
		CacheTTL:   10 * time.Second,
	}
}

func (s *DelegatingAuthenticationOptions) Validate() []error {
	allErrors := []error{}
	return allErrors
}

func (s *DelegatingAuthenticationOptions) AddFlags(fs *pflag.FlagSet) {
	if s == nil {
		return
	}

	var optionalKubeConfigSentence string
	if s.RemoteKubeConfigFileOptional {
		optionalKubeConfigSentence = " This is optional. If empty, all token requests are considered to be anonymous and no client CA is looked up in the cluster."
	}
	fs.StringVar(&s.RemoteKubeConfigFile, "authentication-kubeconfig", s.RemoteKubeConfigFile, ""+
		"kubeconfig file pointing at the 'core' kubernetes server with enough rights to create "+
		"tokenaccessreviews.authentication.k8s.io."+optionalKubeConfigSentence)

	fs.DurationVar(&s.CacheTTL, "authentication-token-webhook-cache-ttl", s.CacheTTL,
		"The duration to cache responses from the webhook token authenticator.")

	fs.BoolVar(&s.SkipInClusterLookup, "authentication-skip-lookup", s.SkipInClusterLookup, ""+
		"If false, the authentication-kubeconfig will be used to lookup missing authentication "+
		"configuration from the cluster.")
}

func (s *DelegatingAuthenticationOptions) ApplyTo(c *server.AuthenticationInfo, servingInfo *server.SecureServingInfo) error {
	if s == nil {
		c.Authenticator = nil
		return nil
	}

	cfg := authenticatorfactory.DelegatingAuthenticatorConfig{
		CacheTTL:  s.CacheTTL,
	}

	client, err := s.getClient()
	if err != nil {
		return fmt.Errorf("failed to get delegated authentication kubeconfig: %v", err)
	}

	// look into configmaps/external-apiserver-authentication for missing authn info
	if !s.SkipInClusterLookup {
		err := s.lookupMissingConfigInCluster(client)
		if err != nil {
			return err
		}
	}

	// create authenticator
	authenticator, err := cfg.New()
	if err != nil {
		return err
	}
	c.Authenticator = authenticator
	c.SupportsBasicAuth = false

	return nil
}

const (
	authenticationConfigMapNamespace = metav1.NamespaceSystem
	// authenticationConfigMapName is the name of ConfigMap in the kube-system namespace holding the root certificate
	// bundle to use to verify client certificates on incoming requests before trusting usernames in headers specified
	// by --requestheader-username-headers. This is created in the cluster by the kube-apiserver.
	// "WARNING: generally do not depend on authorization being already done for incoming requests.")
	authenticationConfigMapName = "extension-apiserver-authentication"
	authenticationRoleName      = "extension-apiserver-authentication-reader"
)

func (s *DelegatingAuthenticationOptions) lookupMissingConfigInCluster(client kubernetes.Interface) error {
	if client == nil {
		return nil
	}

	_, err := client.CoreV1().ConfigMaps(authenticationConfigMapNamespace).Get(authenticationConfigMapName, metav1.GetOptions{})
	switch {
	case errors.IsNotFound(err):
		// ignore, authConfigMap is nil now
	case errors.IsForbidden(err):
		glog.Warningf("Unable to get configmap/%s in %s.  Usually fixed by "+
			"'kubectl create rolebinding -n %s ROLE_NAME --role=%s --serviceaccount=YOUR_NS:YOUR_SA'",
			authenticationConfigMapName, authenticationConfigMapNamespace, authenticationConfigMapNamespace, authenticationRoleName)
		return err
	case err != nil:
		return err
	}

	return nil
}

// getClient returns a Kubernetes clientset. If s.RemoteKubeConfigFileOptional is true, nil will be returned
// if no kubeconfig is specified by the user and the in-cluster config is not found.
func (s *DelegatingAuthenticationOptions) getClient() (kubernetes.Interface, error) {
	var clientConfig *rest.Config
	var err error
	if len(s.RemoteKubeConfigFile) > 0 {
		loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: s.RemoteKubeConfigFile}
		loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{})

		clientConfig, err = loader.ClientConfig()
	} else {
		// without the remote kubeconfig file, try to use the in-cluster config.  Most addon API servers will
		// use this path. If it is optional, ignore errors.
		clientConfig, err = rest.InClusterConfig()
		if err != nil && s.RemoteKubeConfigFileOptional {
			if err != rest.ErrNotInCluster {
				glog.Warningf("failed to read in-cluster kubeconfig for delegated authentication: %v", err)
			}
			return nil, nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get delegated authentication kubeconfig: %v", err)
	}

	// set high qps/burst limits since this will effectively limit API server responsiveness
	clientConfig.QPS = 200
	clientConfig.Burst = 400

	return kubernetes.NewForConfig(clientConfig)
}
