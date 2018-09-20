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

// Package app does all of the work necessary to create a Kubernetes
// APIServer by binding together the API, master and APIServer infrastructure.
// It can be configured and called directly or via the hyperkube framework.
package app

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/admission"
	webhookconfig "k8s.io/apiserver/pkg/admission/plugin/webhook/config"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/server"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/filters"
	serveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/apiserver/pkg/server/options/encryptionconfig"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	clientgoinformers "k8s.io/client-go/informers"
	clientgoclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/cmd/kube-apiserver/app/options"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/admissionregistration"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/apis/networking"
	"k8s.io/kubernetes/pkg/apis/policy"
	"k8s.io/kubernetes/pkg/apis/storage"
	"k8s.io/kubernetes/pkg/capabilities"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	informers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	serviceaccountcontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/kubeapiserver"
	kubeapiserveradmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"
	kubeauthenticator "k8s.io/kubernetes/pkg/kubeapiserver/authenticator"
	"k8s.io/kubernetes/pkg/kubeapiserver/authorizer/modes"
	kubeoptions "k8s.io/kubernetes/pkg/kubeapiserver/options"
	kubeserver "k8s.io/kubernetes/pkg/kubeapiserver/server"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/master/reconcilers"
	quotainstall "k8s.io/kubernetes/pkg/quota/install"
	"k8s.io/kubernetes/pkg/registry/cachesize"
	rbacrest "k8s.io/kubernetes/pkg/registry/rbac/rest"
	"k8s.io/kubernetes/pkg/version"
	"k8s.io/kubernetes/pkg/version/verflag"

	utilflag "k8s.io/kubernetes/pkg/util/flag"
	_ "k8s.io/kubernetes/pkg/util/reflector/prometheus" // for reflector metric registration
	_ "k8s.io/kubernetes/pkg/util/workqueue/prometheus" // for workqueue metric registration
)

const etcdRetryLimit = 60
const etcdRetryInterval = 1 * time.Second

var (
	DefaultProxyDialerFn utilnet.DialFunc
)

// NewAPIServerCommand creates a *cobra.Command object with default parameters
func NewAPIServerCommand() *cobra.Command {
	s := options.NewServerRunOptions()
	cmd := &cobra.Command{
		Use: "kube-apiserver",
		Long: `The Kubernetes API server validates and configures data
for the api objects which include pods, services, replicationcontrollers, and
others. The API Server services REST operations and provides the frontend to the
cluster's shared state through which all other components interact.`,
		Run: func(cmd *cobra.Command, args []string) {
			verflag.PrintAndExitIfRequested()
			utilflag.PrintFlags(cmd.Flags())

			stopCh := server.SetupSignalHandler()
			if err := Run(s, stopCh); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		},
	}
	s.AddFlags(cmd.Flags())

	return cmd
}

// Run runs the specified APIServer.  This should never exit.
func Run(runOptions *options.ServerRunOptions, stopCh <-chan struct{}) error {
	// To help debugging, immediately log version
	glog.Infof("Version: %+v", version.Get())

	_, server, err := CreateServerChain(runOptions, stopCh)
	if err != nil {
		return err
	}

	return server.PrepareRun().Run(stopCh)
}

// CreateServerChain creates the apiservers connected via delegation.
func CreateServerChain(runOptions *options.ServerRunOptions, stopCh <-chan struct{}) (*master.Config, *genericapiserver.GenericAPIServer, error) {
	runOptions.KubeletConfig.Dial = DefaultProxyDialerFn

	kubeAPIServerConfig, sharedInformers, versionedInformers, _, pluginInitializer, err := CreateKubeAPIServerConfig(runOptions)
	if err != nil {
		return nil, nil, err
	}

	// TPRs are enabled and not yet beta, since this these are the successor, they fall under the same enablement rule
	// If additional API servers are added, they should be gated.
	apiExtensionsConfig, err := createAPIExtensionsConfig(*kubeAPIServerConfig.GenericConfig, versionedInformers, pluginInitializer, runOptions)
	if err != nil {
		return nil, nil, err
	}
	apiExtensionsServer, err := createAPIExtensionsServer(apiExtensionsConfig, genericapiserver.EmptyDelegate)
	if err != nil {
		return nil, nil, err
	}

	kubeAPIServer, err := CreateKubeAPIServer(kubeAPIServerConfig, apiExtensionsServer.GenericAPIServer, sharedInformers, versionedInformers)
	if err != nil {
		return nil, nil, err
	}

	apiExtensionsServer.GenericAPIServer.DiscoveryGroupManager = kubeAPIServer.GenericAPIServer.DiscoveryGroupManager

	// This will wire up openapi for extension api server
	apiExtensionsServer.GenericAPIServer.PrepareRun()

	return kubeAPIServerConfig, kubeAPIServer.GenericAPIServer, nil
}

// CreateKubeAPIServer creates and wires a workable kube-apiserver
func CreateKubeAPIServer(kubeAPIServerConfig *master.Config, delegateAPIServer genericapiserver.DelegationTarget, sharedInformers informers.SharedInformerFactory, versionedInformers clientgoinformers.SharedInformerFactory) (*master.Master, error) {
	kubeAPIServer, err := kubeAPIServerConfig.Complete(versionedInformers).New(delegateAPIServer)
	if err != nil {
		return nil, err
	}
	kubeAPIServer.GenericAPIServer.AddPostStartHook("start-kube-apiserver-informers", func(context genericapiserver.PostStartHookContext) error {
		sharedInformers.Start(context.StopCh)
		return nil
	})

	return kubeAPIServer, nil
}

// CreateKubeAPIServerConfig creates all the resources for running the API server, but runs none of them
func CreateKubeAPIServerConfig(
	s *options.ServerRunOptions,
) (
	config *master.Config,
	sharedInformers informers.SharedInformerFactory,
	versionedInformers clientgoinformers.SharedInformerFactory,
	insecureServingInfo *kubeserver.InsecureServingInfo,
	pluginInitializers []admission.PluginInitializer,
	lastErr error,
) {
	// set defaults in the options before trying to create the generic config
	if lastErr = defaultOptions(s); lastErr != nil {
		return
	}

	// validate options
	if errs := s.Validate(); len(errs) != 0 {
		lastErr = utilerrors.NewAggregate(errs)
		return
	}

	var genericConfig *genericapiserver.Config
	genericConfig, sharedInformers, versionedInformers, insecureServingInfo, pluginInitializers, lastErr = BuildGenericConfig(s)
	if lastErr != nil {
		return
	}

	capabilities.Initialize(capabilities.Capabilities{
		AllowPrivileged: s.AllowPrivileged,
		// TODO(vmarmol): Implement support for HostNetworkSources.
		PrivilegedSources: capabilities.PrivilegedSources{
			HostNetworkSources: []string{},
			HostPIDSources:     []string{},
			HostIPCSources:     []string{},
		},
		PerConnectionBandwidthLimitBytesPerSec: s.MaxConnectionBytesPerSec,
	})

	serviceIPRange, apiServerServiceIP, lastErr := master.DefaultServiceIPRange(s.ServiceClusterIPRange)
	if lastErr != nil {
		return
	}

	storageFactory, lastErr := BuildStorageFactory(s, genericConfig.MergedResourceConfig)
	if lastErr != nil {
		return
	}

	config = &master.Config{
		GenericConfig: genericConfig,
		ExtraConfig: master.ExtraConfig{
			APIResourceConfigSource: storageFactory.APIResourceConfigSource,
			StorageFactory:          storageFactory,
			EnableCoreControllers:   true,
			EventTTL:                s.EventTTL,
			KubeletClientConfig:     s.KubeletConfig,
			EnableLogsSupport:       s.EnableLogsHandler,

			ServiceIPRange:       serviceIPRange,
			APIServerServiceIP:   apiServerServiceIP,
			APIServerServicePort: 443,

			ServiceNodePortRange:      s.ServiceNodePortRange,
			KubernetesServiceNodePort: s.KubernetesServiceNodePort,

			EndpointReconcilerType: reconcilers.Type(s.EndpointReconcilerType),
			MasterCount:            s.MasterCount,
		},
	}

	return
}

// BuildGenericConfig takes the master server options and produces the genericapiserver.Config associated with it
func BuildGenericConfig(
	s *options.ServerRunOptions,
) (
	genericConfig *genericapiserver.Config,
	sharedInformers informers.SharedInformerFactory,
	versionedInformers clientgoinformers.SharedInformerFactory,
	insecureServingInfo *kubeserver.InsecureServingInfo,
	pluginInitializers []admission.PluginInitializer,
	lastErr error,
) {
	genericConfig = genericapiserver.NewConfig(legacyscheme.Codecs)
	if lastErr = s.GenericServerRunOptions.ApplyTo(genericConfig); lastErr != nil {
		return
	}

	if insecureServingInfo, lastErr = s.InsecureServing.ApplyTo(genericConfig); lastErr != nil {
		return
	}
	if lastErr = s.SecureServing.ApplyTo(genericConfig); lastErr != nil {
		return
	}
	if lastErr = s.Authentication.ApplyTo(genericConfig); lastErr != nil {
		return
	}
	if lastErr = s.Features.ApplyTo(genericConfig); lastErr != nil {
		return
	}
	if lastErr = s.APIEnablement.ApplyTo(genericConfig, master.DefaultAPIResourceConfigSource(), legacyscheme.Registry); lastErr != nil {
		return
	}

	genericConfig.EnableMetrics = true
	genericConfig.LongRunningFunc = filters.BasicLongRunningRequestCheck(
		sets.NewString("watch", "proxy"),
		sets.NewString("attach", "exec", "proxy", "log", "portforward"),
	)

	kubeVersion := version.Get()
	genericConfig.Version = &kubeVersion

	storageFactory, lastErr := BuildStorageFactory(s, genericConfig.MergedResourceConfig)
	if lastErr != nil {
		return
	}
	if lastErr = s.Etcd.ApplyWithStorageFactoryTo(storageFactory, genericConfig); lastErr != nil {
		return
	}

	// Use protobufs for self-communication.
	// Since not every generic apiserver has to support protobufs, we
	// cannot default to it in generic apiserver and need to explicitly
	// set it in kube-apiserver.
	genericConfig.LoopbackClientConfig.ContentConfig.ContentType = "application/vnd.kubernetes.protobuf"

	client, err := internalclientset.NewForConfig(genericConfig.LoopbackClientConfig)
	if err != nil {
		kubeAPIVersions := os.Getenv("KUBE_API_VERSIONS")
		if len(kubeAPIVersions) == 0 {
			lastErr = fmt.Errorf("failed to create clientset: %v", err)
			return
		}

		// KUBE_API_VERSIONS is used in test-update-storage-objects.sh, disabling a number of API
		// groups. This leads to a nil client above and undefined behaviour further down.
		//
		// TODO: get rid of KUBE_API_VERSIONS or define sane behaviour if set
		glog.Errorf("Failed to create clientset with KUBE_API_VERSIONS=%q. KUBE_API_VERSIONS is only for testing. Things will break.", kubeAPIVersions)
	}

	kubeClientConfig := genericConfig.LoopbackClientConfig
	sharedInformers = informers.NewSharedInformerFactory(client, 10*time.Minute)
	clientgoExternalClient, err := clientgoclientset.NewForConfig(kubeClientConfig)
	if err != nil {
		lastErr = fmt.Errorf("failed to create real external clientset: %v", err)
		return
	}
	versionedInformers = clientgoinformers.NewSharedInformerFactory(clientgoExternalClient, 10*time.Minute)

	genericConfig.Authentication.Authenticator, err = BuildAuthenticator(s, storageFactory, client, clientgoExternalClient, sharedInformers)
	if err != nil {
		lastErr = fmt.Errorf("invalid authentication config: %v", err)
		return
	}

	genericConfig.Authorization.Authorizer, genericConfig.RuleResolver, err = BuildAuthorizer(s, sharedInformers, versionedInformers)
	if err != nil {
		lastErr = fmt.Errorf("invalid authorization config: %v", err)
		return
	}
	if !sets.NewString(s.Authorization.Modes()...).Has(modes.ModeRBAC) {
		genericConfig.DisabledPostStartHooks.Insert(rbacrest.PostStartHookName)
	}

	webhookAuthResolverWrapper := func(delegate webhookconfig.AuthenticationInfoResolver) webhookconfig.AuthenticationInfoResolver {
		return &webhookconfig.AuthenticationInfoResolverDelegator{
			ClientConfigForFunc: func(server string) (*rest.Config, error) {
				if server == "kubernetes.default.svc" {
					return genericConfig.LoopbackClientConfig, nil
				}
				return delegate.ClientConfigFor(server)
			},
			ClientConfigForServiceFunc: func(serviceName, serviceNamespace string) (*rest.Config, error) {
				if serviceName == "kubernetes" && serviceNamespace == "default" {
					return genericConfig.LoopbackClientConfig, nil
				}
				ret, err := delegate.ClientConfigForService(serviceName, serviceNamespace)
				if err != nil {
					return nil, err
				}
				return ret, err
			},
		}
	}
	pluginInitializers, err = BuildAdmissionPluginInitializers(
		s,
		client,
		sharedInformers,
		webhookAuthResolverWrapper,
	)
	if err != nil {
		lastErr = fmt.Errorf("failed to create admission plugin initializer: %v", err)
		return
	}

	err = s.Admission.ApplyTo(
		genericConfig,
		versionedInformers,
		kubeClientConfig,
		legacyscheme.Scheme,
		pluginInitializers...)
	if err != nil {
		lastErr = fmt.Errorf("failed to initialize admission: %v", err)
	}
	return
}

// BuildAdmissionPluginInitializers constructs the admission plugin initializer
func BuildAdmissionPluginInitializers(
	s *options.ServerRunOptions,
	client internalclientset.Interface,
	sharedInformers informers.SharedInformerFactory,
	webhookAuthWrapper webhookconfig.AuthenticationInfoResolverWrapper,
) ([]admission.PluginInitializer, error) {
	var cloudConfig []byte

	if s.CloudProvider.CloudConfigFile != "" {
		var err error
		cloudConfig, err = ioutil.ReadFile(s.CloudProvider.CloudConfigFile)
		if err != nil {
			glog.Fatalf("Error reading from cloud configuration file %s: %#v", s.CloudProvider.CloudConfigFile, err)
		}
	}

	// TODO: use a dynamic restmapper. See https://github.com/kubernetes/kubernetes/pull/42615.
	restMapper := legacyscheme.Registry.RESTMapper()

	quotaConfiguration := quotainstall.NewQuotaConfigurationForAdmission()

	kubePluginInitializer := kubeapiserveradmission.NewPluginInitializer(client, sharedInformers, cloudConfig, restMapper, quotaConfiguration)

	return []admission.PluginInitializer{kubePluginInitializer}, nil
}

// BuildAuthenticator constructs the authenticator
func BuildAuthenticator(s *options.ServerRunOptions, storageFactory serverstorage.StorageFactory, client internalclientset.Interface, extclient clientgoclientset.Interface, sharedInformers informers.SharedInformerFactory) (authenticator.Request, error) {
	authenticatorConfig := s.Authentication.ToAuthenticationConfig()
	if s.Authentication.ServiceAccounts.Lookup {
		authenticatorConfig.ServiceAccountTokenGetter = serviceaccountcontroller.NewGetterFromClient(extclient)
	}
	return authenticatorConfig.New()
}

// BuildAuthorizer constructs the authorizer
func BuildAuthorizer(s *options.ServerRunOptions, sharedInformers informers.SharedInformerFactory, versionedInformers clientgoinformers.SharedInformerFactory) (authorizer.Authorizer, authorizer.RuleResolver, error) {
	authorizationConfig := s.Authorization.ToAuthorizationConfig(sharedInformers, versionedInformers)
	return authorizationConfig.New()
}

// BuildStorageFactory constructs the storage factory. If encryption at rest is used, it expects
// all supported KMS plugins to be registered in the KMS plugin registry before being called.
func BuildStorageFactory(s *options.ServerRunOptions, apiResourceConfig *serverstorage.ResourceConfig) (*serverstorage.DefaultStorageFactory, error) {
	storageGroupsToEncodingVersion, err := s.StorageSerialization.StorageGroupsToEncodingVersion()
	if err != nil {
		return nil, fmt.Errorf("error generating storage version map: %s", err)
	}
	storageFactory, err := kubeapiserver.NewStorageFactory(
		s.Etcd.StorageConfig, s.Etcd.DefaultStorageMediaType, legacyscheme.Codecs,
		serverstorage.NewDefaultResourceEncodingConfig(legacyscheme.Registry), storageGroupsToEncodingVersion,
		// The list includes resources that need to be stored in a different
		// group version than other resources in the groups.
		// FIXME (soltysh): this GroupVersionResource override should be configurable
		[]schema.GroupVersionResource{
			batch.Resource("cronjobs").WithVersion("v1beta1"),
			storage.Resource("volumeattachments").WithVersion("v1beta1"),
			admissionregistration.Resource("initializerconfigurations").WithVersion("v1alpha1"),
		},
		apiResourceConfig)
	if err != nil {
		return nil, fmt.Errorf("error in initializing storage factory: %s", err)
	}

	storageFactory.AddCohabitatingResources(networking.Resource("networkpolicies"), extensions.Resource("networkpolicies"))
	storageFactory.AddCohabitatingResources(apps.Resource("deployments"), extensions.Resource("deployments"))
	storageFactory.AddCohabitatingResources(apps.Resource("daemonsets"), extensions.Resource("daemonsets"))
	storageFactory.AddCohabitatingResources(apps.Resource("replicasets"), extensions.Resource("replicasets"))
	// TODO(#54933): 1.11: switch to using policy storage and flip the order here
	storageFactory.AddCohabitatingResources(extensions.Resource("podsecuritypolicies"), policy.Resource("podsecuritypolicies"))
	for _, override := range s.Etcd.EtcdServersOverrides {
		tokens := strings.Split(override, "#")
		if len(tokens) != 2 {
			glog.Errorf("invalid value of etcd server overrides: %s", override)
			continue
		}

		apiresource := strings.Split(tokens[0], "/")
		if len(apiresource) != 2 {
			glog.Errorf("invalid resource definition: %s", tokens[0])
			continue
		}
		group := apiresource[0]
		resource := apiresource[1]
		groupResource := schema.GroupResource{Group: group, Resource: resource}

		servers := strings.Split(tokens[1], ";")
		storageFactory.SetEtcdLocation(groupResource, servers)
	}

	if len(s.Etcd.EncryptionProviderConfigFilepath) != 0 {
		transformerOverrides, err := encryptionconfig.GetTransformerOverrides(s.Etcd.EncryptionProviderConfigFilepath)
		if err != nil {
			return nil, err
		}
		for groupResource, transformer := range transformerOverrides {
			storageFactory.SetTransformer(groupResource, transformer)
		}
	}

	return storageFactory, nil
}

func defaultOptions(s *options.ServerRunOptions) error {
	// set defaults
	if err := s.GenericServerRunOptions.DefaultAdvertiseAddress(s.SecureServing.SecureServingOptions); err != nil {
		return err
	}
	if err := kubeoptions.DefaultAdvertiseAddress(s.GenericServerRunOptions, s.InsecureServing); err != nil {
		return err
	}
	serviceIPRange, apiServerServiceIP, err := master.DefaultServiceIPRange(s.ServiceClusterIPRange)
	if err != nil {
		return fmt.Errorf("error determining service IP ranges: %v", err)
	}
	s.ServiceClusterIPRange = serviceIPRange
	if err := s.SecureServing.MaybeDefaultWithSelfSignedCerts(s.GenericServerRunOptions.AdvertiseAddress.String(), []string{"kubernetes.default.svc", "kubernetes.default", "kubernetes"}, []net.IP{apiServerServiceIP}); err != nil {
		return fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	if len(s.GenericServerRunOptions.ExternalHost) == 0 {
		if len(s.GenericServerRunOptions.AdvertiseAddress) > 0 {
			s.GenericServerRunOptions.ExternalHost = s.GenericServerRunOptions.AdvertiseAddress.String()
		} else {
			if hostname, err := os.Hostname(); err == nil {
				s.GenericServerRunOptions.ExternalHost = hostname
			} else {
				return fmt.Errorf("error finding host name: %v", err)
			}
		}
		glog.Infof("external host was not specified, using %v", s.GenericServerRunOptions.ExternalHost)
	}

	s.Authentication.ApplyAuthorization(s.Authorization)

	// Use (ServiceAccountSigningKeyFile != "") as a proxy to the user enabling
	// TokenRequest functionality. This defaulting was convenient, but messed up
	// a lot of people when they rotated their serving cert with no idea it was
	// connected to their service account keys. We are taking this oppurtunity to
	// remove this problematic defaulting.
	if s.ServiceAccountSigningKeyFile == "" {
		// Default to the private server key for service account token signing
		if len(s.Authentication.ServiceAccounts.KeyFiles) == 0 && s.SecureServing.ServerCert.CertKey.KeyFile != "" {
			if kubeauthenticator.IsValidServiceAccountKeyFile(s.SecureServing.ServerCert.CertKey.KeyFile) {
				s.Authentication.ServiceAccounts.KeyFiles = []string{s.SecureServing.ServerCert.CertKey.KeyFile}
			} else {
				glog.Warning("No TLS key provided, service account token authentication disabled")
			}
		}
	}

	if s.Etcd.StorageConfig.DeserializationCacheSize == 0 {
		// When size of cache is not explicitly set, estimate its size based on
		// target memory usage.
		glog.V(2).Infof("Initializing deserialization cache size based on %dMB limit", s.GenericServerRunOptions.TargetRAMMB)

		// This is the heuristics that from memory capacity is trying to infer
		// the maximum number of nodes in the cluster and set cache sizes based
		// on that value.
		// From our documentation, we officially recommend 120GB machines for
		// 2000 nodes, and we scale from that point. Thus we assume ~60MB of
		// capacity per node.
		// TODO: We may consider deciding that some percentage of memory will
		// be used for the deserialization cache and divide it by the max object
		// size to compute its size. We may even go further and measure
		// collective sizes of the objects in the cache.
		clusterSize := s.GenericServerRunOptions.TargetRAMMB / 60
		s.Etcd.StorageConfig.DeserializationCacheSize = 25 * clusterSize
		if s.Etcd.StorageConfig.DeserializationCacheSize < 1000 {
			s.Etcd.StorageConfig.DeserializationCacheSize = 1000
		}
	}
	if s.Etcd.EnableWatchCache {
		glog.V(2).Infof("Initializing cache sizes based on %dMB limit", s.GenericServerRunOptions.TargetRAMMB)
		sizes := cachesize.NewHeuristicWatchCacheSizes(s.GenericServerRunOptions.TargetRAMMB)
		if userSpecified, err := serveroptions.ParseWatchCacheSizes(s.Etcd.WatchCacheSizes); err == nil {
			for resource, size := range userSpecified {
				sizes[resource] = size
			}
		}
		s.Etcd.WatchCacheSizes, err = serveroptions.WriteWatchCacheSizes(sizes)
		if err != nil {
			return err
		}
	}

	// TODO: remove when we stop supporting the legacy group version.
	if s.APIEnablement.RuntimeConfig != nil {
		for key, value := range s.APIEnablement.RuntimeConfig {
			if key == "v1" || strings.HasPrefix(key, "v1/") ||
				key == "api/v1" || strings.HasPrefix(key, "api/v1/") {
				delete(s.APIEnablement.RuntimeConfig, key)
				s.APIEnablement.RuntimeConfig["/v1"] = value
			}
			if key == "api/legacy" {
				delete(s.APIEnablement.RuntimeConfig, key)
			}
		}
	}

	return nil
}
