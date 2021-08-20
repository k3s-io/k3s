/*
Copyright 2018 The Kubernetes Authors.

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
	"io"
	"net"
	"net/url"

	"github.com/spf13/pflag"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/apiserver"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	genericregistry "k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/options"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/util/proxy"
	"k8s.io/apiserver/pkg/util/webhook"
	corev1 "k8s.io/client-go/listers/core/v1"
)

const defaultEtcdPathPrefix = "/registry/apiextensions.kubernetes.io"

// CustomResourceDefinitionsServerOptions describes the runtime options of an apiextensions-apiserver.
type CustomResourceDefinitionsServerOptions struct {
	ServerRunOptions   *options.ServerRunOptions
	RecommendedOptions *genericoptions.RecommendedOptions
	APIEnablement      *genericoptions.APIEnablementOptions

	StdOut io.Writer
	StdErr io.Writer
}

// NewCustomResourceDefinitionsServerOptions creates default options of an apiextensions-apiserver.
func NewCustomResourceDefinitionsServerOptions(out, errOut io.Writer) *CustomResourceDefinitionsServerOptions {
	o := &CustomResourceDefinitionsServerOptions{
		ServerRunOptions: options.NewServerRunOptions(),
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			defaultEtcdPathPrefix,
			apiserver.Codecs.LegacyCodec(v1beta1.SchemeGroupVersion, v1.SchemeGroupVersion),
		),
		APIEnablement: genericoptions.NewAPIEnablementOptions(),

		StdOut: out,
		StdErr: errOut,
	}

	return o
}

// AddFlags adds the apiextensions-apiserver flags to the flagset.
func (o CustomResourceDefinitionsServerOptions) AddFlags(fs *pflag.FlagSet) {
	o.ServerRunOptions.AddUniversalFlags(fs)
	o.RecommendedOptions.AddFlags(fs)
	o.APIEnablement.AddFlags(fs)
}

// Validate validates the apiextensions-apiserver options.
func (o CustomResourceDefinitionsServerOptions) Validate() error {
	errors := []error{}
	errors = append(errors, o.ServerRunOptions.Validate()...)
	errors = append(errors, o.RecommendedOptions.Validate()...)
	errors = append(errors, o.APIEnablement.Validate(apiserver.Scheme)...)
	return utilerrors.NewAggregate(errors)
}

// Complete fills in missing options.
func (o *CustomResourceDefinitionsServerOptions) Complete() error {
	return nil
}

// Config returns an apiextensions-apiserver configuration.
func (o CustomResourceDefinitionsServerOptions) Config() (*apiserver.Config, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(apiserver.Codecs)
	if err := o.ServerRunOptions.ApplyTo(&serverConfig.Config); err != nil {
		return nil, err
	}
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}
	if err := o.APIEnablement.ApplyTo(&serverConfig.Config, apiserver.DefaultAPIResourceConfigSource(), apiserver.Scheme); err != nil {
		return nil, err
	}

	config := &apiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig: apiserver.ExtraConfig{
			CRDRESTOptionsGetter: NewCRDRESTOptionsGetter(*o.RecommendedOptions.Etcd),
			ServiceResolver:      &serviceResolver{serverConfig.SharedInformerFactory.Core().V1().Services().Lister()},
			AuthResolverWrapper:  webhook.NewDefaultAuthenticationInfoResolverWrapper(nil, nil, serverConfig.LoopbackClientConfig, nil),
		},
	}
	return config, nil
}

// NewCRDRESTOptionsGetter create a RESTOptionsGetter for CustomResources.
func NewCRDRESTOptionsGetter(etcdOptions genericoptions.EtcdOptions) genericregistry.RESTOptionsGetter {
	ret := apiserver.CRDRESTOptionsGetter{
		StorageConfig:           etcdOptions.StorageConfig,
		StoragePrefix:           etcdOptions.StorageConfig.Prefix,
		EnableWatchCache:        etcdOptions.EnableWatchCache,
		DefaultWatchCacheSize:   etcdOptions.DefaultWatchCacheSize,
		EnableGarbageCollection: etcdOptions.EnableGarbageCollection,
		DeleteCollectionWorkers: etcdOptions.DeleteCollectionWorkers,
		CountMetricPollPeriod:   etcdOptions.StorageConfig.CountMetricPollPeriod,
	}
	ret.StorageConfig.Codec = unstructured.UnstructuredJSONScheme

	return ret
}

type serviceResolver struct {
	services corev1.ServiceLister
}

func (r *serviceResolver) ResolveEndpoint(namespace, name string, port int32) (*url.URL, error) {
	return proxy.ResolveCluster(r.services, namespace, name, port)
}
