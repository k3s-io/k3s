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

package authenticatorfactory

import (
	"errors"
	"fmt"
	"time"

	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authentication/group"
	"k8s.io/apiserver/pkg/authentication/request/headerrequest"
	unionauth "k8s.io/apiserver/pkg/authentication/request/union"
	"k8s.io/apiserver/pkg/authentication/request/x509"
	authenticationclient "k8s.io/client-go/kubernetes/typed/authentication/v1"
	"k8s.io/client-go/util/cert"
)

// DelegatingAuthenticatorConfig is the minimal configuration needed to create an authenticator
// built to delegate authentication to a kube API server
type DelegatingAuthenticatorConfig struct {
	// TokenAccessReviewClient is a client to do token review. It can be nil. Then every token is ignored.
	TokenAccessReviewClient authenticationclient.TokenReviewInterface

	// CacheTTL is the length of time that a token authentication answer will be cached.
	CacheTTL time.Duration

	// ClientCAFile is the CA bundle file used to authenticate client certificates
	ClientCAFile string

	RequestHeaderConfig *RequestHeaderConfig
}

func (c DelegatingAuthenticatorConfig) New() (authenticator.Request, error) {
	authenticators := []authenticator.Request{}

	// front-proxy first, then remote
	// Add the front proxy authenticator if requested
	if c.RequestHeaderConfig != nil {
		requestHeaderAuthenticator, err := headerrequest.NewSecure(
			c.RequestHeaderConfig.ClientCA,
			c.RequestHeaderConfig.AllowedClientNames,
			c.RequestHeaderConfig.UsernameHeaders,
			c.RequestHeaderConfig.GroupHeaders,
			c.RequestHeaderConfig.ExtraHeaderPrefixes,
		)
		if err != nil {
			return nil, err
		}
		authenticators = append(authenticators, requestHeaderAuthenticator)
	}

	// x509 client cert auth
	if len(c.ClientCAFile) > 0 {
		clientCAs, err := cert.NewPool(c.ClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("unable to load client CA file %s: %v", c.ClientCAFile, err)
		}
		verifyOpts := x509.DefaultVerifyOptions()
		verifyOpts.Roots = clientCAs
		authenticators = append(authenticators, x509.New(verifyOpts, x509.CommonNameUserConversion))
	}

	if len(authenticators) == 0 {
		return nil, errors.New("No authentication method configured")
	}

	authenticator := group.NewAuthenticatedGroupAdder(unionauth.New(authenticators...))
	return authenticator, nil
}
