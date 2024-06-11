package s3

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd/snapshot"
	"github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/wrangler/v3/pkg/generated/controllers/core"
	corev1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/generic/fake"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/lru"
)

var gmt = time.FixedZone("GMT", 0)

func Test_UnitControllerGetClient(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Dummy server with http and https listeners as a simple S3 mock
	server := &http.Server{Handler: s3Router(t)}

	// Create temp cert/key
	cert, key, _ := cert.GenerateSelfSignedCertKey("localhost", []net.IP{net.ParseIP("::1"), net.ParseIP("127.0.0.1")}, nil)
	tempDir := t.TempDir()
	certFile := filepath.Join(tempDir, "test.crt")
	keyFile := filepath.Join(tempDir, "test.key")
	os.WriteFile(certFile, cert, 0600)
	os.WriteFile(keyFile, key, 0600)

	listener, _ := net.Listen("tcp", ":0")
	listenerTLS, _ := net.Listen("tcp", ":0")

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	listenerAddr := net.JoinHostPort("localhost", port)
	_, port, _ = net.SplitHostPort(listenerTLS.Addr().String())
	listenerTLSAddr := net.JoinHostPort("localhost", port)

	go server.Serve(listener)
	go server.ServeTLS(listenerTLS, certFile, keyFile)
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	type fields struct {
		clusterID   string
		tokenHash   string
		nodeName    string
		clientCache *lru.Cache
	}
	type args struct {
		ctx    context.Context
		etcdS3 *config.EtcdS3
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		setup   func(t *testing.T, a args, f fields, c *Client) (core.Interface, error)
		want    *Client
		wantErr bool
	}{
		{
			name: "Fail to get client with nil config",
			args: args{
				ctx: ctx,
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			wantErr: true,
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Fail to get client when bucket not set",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			wantErr: true,
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Fail to get client when bucket does not exist",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "badbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			wantErr: true,
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Fail to get client with missing Secret",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					Endpoint:     defaultEtcdS3.Endpoint,
					Region:       defaultEtcdS3.Region,
					ConfigSecret: "my-etcd-s3-config-secret",
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			wantErr: true,
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				coreMock.v1.secret.EXPECT().Get(metav1.NamespaceSystem, "my-etcd-s3-config-secret", gomock.Any()).AnyTimes().DoAndReturn(func(namespace, name string, _ metav1.GetOptions) (*v1.Secret, error) {
					return nil, errorNotFound("secret", name)
				})
				return coreMock, nil
			},
		},
		{
			name: "Create client for config from secret",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					Endpoint:     defaultEtcdS3.Endpoint,
					Region:       defaultEtcdS3.Region,
					ConfigSecret: "my-etcd-s3-config-secret",
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				coreMock.v1.secret.EXPECT().Get(metav1.NamespaceSystem, "my-etcd-s3-config-secret", gomock.Any()).AnyTimes().DoAndReturn(func(namespace, name string, _ metav1.GetOptions) (*v1.Secret, error) {
					return &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Type: v1.SecretTypeOpaque,
						Data: map[string][]byte{
							"etcd-s3-access-key":  []byte("test"),
							"etcd-s3-bucket":      []byte("testbucket"),
							"etcd-s3-endpoint":    []byte(listenerTLSAddr),
							"etcd-s3-region":      []byte("us-west-2"),
							"etcd-s3-timeout":     []byte("1m"),
							"etcd-s3-endpoint-ca": cert,
						},
					}, nil
				})
				return coreMock, nil
			},
		},
		{
			name: "Create client for config from secret with CA in configmap",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					Endpoint:     defaultEtcdS3.Endpoint,
					Region:       defaultEtcdS3.Region,
					ConfigSecret: "my-etcd-s3-config-secret",
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				coreMock.v1.secret.EXPECT().Get(metav1.NamespaceSystem, "my-etcd-s3-config-secret", gomock.Any()).AnyTimes().DoAndReturn(func(namespace, name string, _ metav1.GetOptions) (*v1.Secret, error) {
					return &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Type: v1.SecretTypeOpaque,
						Data: map[string][]byte{
							"etcd-s3-access-key":       []byte("test"),
							"etcd-s3-bucket":           []byte("testbucket"),
							"etcd-s3-endpoint":         []byte(listenerTLSAddr),
							"etcd-s3-region":           []byte("us-west-2"),
							"etcd-s3-timeout":          []byte("1m"),
							"etcd-s3-endpoint-ca-name": []byte("my-etcd-s3-ca"),
							"etcd-s3-skip-ssl-verify":  []byte("false"),
						},
					}, nil
				})
				coreMock.v1.configMap.EXPECT().Get(metav1.NamespaceSystem, "my-etcd-s3-ca", gomock.Any()).AnyTimes().DoAndReturn(func(namespace, name string, _ metav1.GetOptions) (*v1.ConfigMap, error) {
					return &v1.ConfigMap{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Data: map[string]string{
							"dummy-ca": string(cert),
						},
						BinaryData: map[string][]byte{
							"dummy-ca-binary": cert,
						},
					}, nil
				})
				return coreMock, nil
			},
		},
		{
			name: "Fail to create client for config from secret with CA in missing configmap",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					Endpoint:     defaultEtcdS3.Endpoint,
					Region:       defaultEtcdS3.Region,
					ConfigSecret: "my-etcd-s3-config-secret",
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			wantErr: true,
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				coreMock.v1.secret.EXPECT().Get(metav1.NamespaceSystem, "my-etcd-s3-config-secret", gomock.Any()).AnyTimes().DoAndReturn(func(namespace, name string, _ metav1.GetOptions) (*v1.Secret, error) {
					return &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Type: v1.SecretTypeOpaque,
						Data: map[string][]byte{
							"etcd-s3-access-key":       []byte("test"),
							"etcd-s3-bucket":           []byte("testbucket"),
							"etcd-s3-endpoint":         []byte(listenerTLSAddr),
							"etcd-s3-region":           []byte("us-west-2"),
							"etcd-s3-timeout":          []byte("invalid"),
							"etcd-s3-endpoint-ca":      []byte("invalid"),
							"etcd-s3-endpoint-ca-name": []byte("my-etcd-s3-ca"),
							"etcd-s3-skip-ssl-verify":  []byte("invalid"),
							"etcd-s3-insecure":         []byte("invalid"),
						},
					}, nil
				})
				coreMock.v1.configMap.EXPECT().Get(metav1.NamespaceSystem, "my-etcd-s3-ca", gomock.Any()).AnyTimes().DoAndReturn(func(namespace, name string, _ metav1.GetOptions) (*v1.ConfigMap, error) {
					return nil, errorNotFound("configmap", name)
				})
				return coreMock, nil
			},
		},
		{
			name: "Create insecure client for config from cli when secret is also set",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey:    "test",
					Bucket:       "testbucket",
					Region:       "us-west-2",
					ConfigSecret: "my-etcd-s3-config-secret",
					Endpoint:     listenerAddr,
					Insecure:     true,
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Create skip-ssl-verify client for config from cli when secret is also set",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey:     "test",
					Bucket:        "testbucket",
					Region:        "us-west-2",
					ConfigSecret:  "my-etcd-s3-config-secret",
					Endpoint:      listenerTLSAddr,
					SkipSSLVerify: true,
					Timeout:       *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Create client for config from cli when secret is not set",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Region:    "us-west-2",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Get cached client for config from secret",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					Endpoint:     defaultEtcdS3.Endpoint,
					Region:       defaultEtcdS3.Region,
					ConfigSecret: "my-etcd-s3-config-secret",
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			want: &Client{},
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				c.etcdS3 = &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				}
				f.clientCache.Add(*c.etcdS3, c)
				coreMock := newCoreMock(gomock.NewController(t))
				coreMock.v1.secret.EXPECT().Get(metav1.NamespaceSystem, "my-etcd-s3-config-secret", gomock.Any()).AnyTimes().DoAndReturn(func(namespace, name string, _ metav1.GetOptions) (*v1.Secret, error) {
					return &v1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Type: v1.SecretTypeOpaque,
						Data: map[string][]byte{
							"etcd-s3-access-key": []byte("test"),
							"etcd-s3-bucket":     []byte("testbucket"),
							"etcd-s3-endpoint":   []byte(listenerAddr),
							"etcd-s3-insecure":   []byte("true"),
						},
					}, nil
				})
				return coreMock, nil
			},
		},
		{
			name: "Get cached client for config from cli",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey:    "test",
					Bucket:       "testbucket",
					Region:       "us-west-2",
					ConfigSecret: "my-etcd-s3-config-secret",
					Endpoint:     listenerAddr,
					Insecure:     true,
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			want: &Client{},
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				c.etcdS3 = a.etcdS3
				f.clientCache.Add(*c.etcdS3, c)
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Create client for config from cli with proxy",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey:    "test",
					Bucket:       "testbucket",
					Region:       "us-west-2",
					ConfigSecret: "my-etcd-s3-config-secret",
					Endpoint:     listenerAddr,
					Insecure:     true,
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
					Proxy:        "http://" + listenerAddr,
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Fail to create client for config from cli with invalid proxy",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey:    "test",
					Bucket:       "testbucket",
					Region:       "us-west-2",
					ConfigSecret: "my-etcd-s3-config-secret",
					Endpoint:     listenerAddr,
					Insecure:     true,
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
					Proxy:        "http://%invalid",
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			wantErr: true,
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Fail to create client for config from cli with no proxy scheme",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey:    "test",
					Bucket:       "testbucket",
					Region:       "us-west-2",
					ConfigSecret: "my-etcd-s3-config-secret",
					Endpoint:     listenerAddr,
					Insecure:     true,
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
					Proxy:        "/proxy",
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			wantErr: true,
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Create client for config from cli with CA path",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey:    "test",
					Bucket:       "testbucket",
					Region:       "us-west-2",
					ConfigSecret: "my-etcd-s3-config-secret",
					Endpoint:     listenerTLSAddr,
					EndpointCA:   certFile,
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
		{
			name: "Fail to create client for config from cli with invalid CA path",
			args: args{
				ctx: ctx,
				etcdS3: &config.EtcdS3{
					AccessKey:    "test",
					Bucket:       "testbucket",
					Region:       "us-west-2",
					ConfigSecret: "my-etcd-s3-config-secret",
					Endpoint:     listenerTLSAddr,
					EndpointCA:   "/does/not/exist",
					Timeout:      *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			fields: fields{
				clusterID:   "1234",
				tokenHash:   "abcd",
				nodeName:    "server01",
				clientCache: lru.New(5),
			},
			wantErr: true,
			setup: func(t *testing.T, a args, f fields, c *Client) (core.Interface, error) {
				coreMock := newCoreMock(gomock.NewController(t))
				return coreMock, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, err := tt.setup(t, tt.args, tt.fields, tt.want)
			if err != nil {
				t.Errorf("Setup for Controller.GetClient() failed = %v", err)
				return
			}
			c := &Controller{
				clusterID:   tt.fields.clusterID,
				tokenHash:   tt.fields.tokenHash,
				nodeName:    tt.fields.nodeName,
				clientCache: tt.fields.clientCache,
				core:        core,
			}
			got, err := c.GetClient(tt.args.ctx, tt.args.etcdS3)
			t.Logf("Got client=%#v err=%v", got, err)
			if (err != nil) != tt.wantErr {
				t.Errorf("Controller.GetClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Controller.GetClient() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitClientUpload(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Dummy server with http listener as a simple S3 mock
	server := &http.Server{Handler: s3Router(t)}

	listener, _ := net.Listen("tcp", ":0")

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	listenerAddr := net.JoinHostPort("localhost", port)

	go server.Serve(listener)
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	controller, err := Start(ctx, &config.Control{ClusterReset: true})
	if err != nil {
		t.Errorf("Start() for Client.Upload() failed = %v", err)
		return
	}

	tempDir := t.TempDir()
	metadataDir := filepath.Join(tempDir, ".metadata")
	snapshotDir := filepath.Join(tempDir, "snapshots")
	snapshotPath := filepath.Join(snapshotDir, "snapshot-01")
	metadataPath := filepath.Join(metadataDir, "snapshot-01")
	if err := os.Mkdir(snapshotDir, 0700); err != nil {
		t.Errorf("Mkdir() failed = %v", err)
		return
	}
	if err := os.Mkdir(metadataDir, 0700); err != nil {
		t.Errorf("Mkdir() failed = %v", err)
		return
	}
	if err := os.WriteFile(snapshotPath, []byte("test snapshot file\n"), 0600); err != nil {
		t.Errorf("WriteFile() failed = %v", err)
		return
	}
	if err := os.WriteFile(metadataPath, []byte("test snapshot metadata\n"), 0600); err != nil {
		t.Errorf("WriteFile() failed = %v", err)
		return
	}

	t.Logf("Using snapshot = %s, metadata = %s", snapshotPath, metadataPath)

	type fields struct {
		controller *Controller
		etcdS3     *config.EtcdS3
	}
	type args struct {
		ctx           context.Context
		snapshotPath  string
		extraMetadata *v1.ConfigMap
		now           time.Time
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *snapshot.File
		wantErr bool
	}{
		{
			name: "Successful Upload",
			fields: fields{
				controller: controller,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			args: args{
				ctx:           ctx,
				snapshotPath:  snapshotPath,
				extraMetadata: &v1.ConfigMap{Data: map[string]string{"foo": "bar"}},
				now:           time.Now(),
			},
		},
		{
			name: "Successful Upload with Prefix",
			fields: fields{
				controller: controller,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Folder:    "testfolder",
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			args: args{
				ctx:           ctx,
				snapshotPath:  snapshotPath,
				extraMetadata: &v1.ConfigMap{Data: map[string]string{"foo": "bar"}},
				now:           time.Now(),
			},
		},
		{
			name: "Fails Upload to Nonexistent Bucket",
			fields: fields{
				controller: controller,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "badbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			args: args{
				ctx:           ctx,
				snapshotPath:  snapshotPath,
				extraMetadata: &v1.ConfigMap{Data: map[string]string{"foo": "bar"}},
				now:           time.Now(),
			},
			wantErr: true,
		},
		{
			name: "Fails Upload to Unauthorized Bucket",
			fields: fields{
				controller: controller,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "authbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			args: args{
				ctx:           ctx,
				snapshotPath:  snapshotPath,
				extraMetadata: &v1.ConfigMap{Data: map[string]string{"foo": "bar"}},
				now:           time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := tt.fields.controller.GetClient(tt.args.ctx, tt.fields.etcdS3)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("GetClient for Client.Upload() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			got, err := c.Upload(tt.args.ctx, tt.args.snapshotPath, tt.args.extraMetadata, tt.args.now)
			t.Logf("Got File=%#v err=%v", got, err)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Upload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.Upload() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitClientDownload(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Dummy server with http listener as a simple S3 mock
	server := &http.Server{Handler: s3Router(t)}

	listener, _ := net.Listen("tcp", ":0")

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	listenerAddr := net.JoinHostPort("localhost", port)

	go server.Serve(listener)
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	controller, err := Start(ctx, &config.Control{ClusterReset: true})
	if err != nil {
		t.Errorf("Start() for Client.Download() failed = %v", err)
		return
	}

	snapshotName := "snapshot-01"
	tempDir := t.TempDir()
	metadataDir := filepath.Join(tempDir, ".metadata")
	snapshotDir := filepath.Join(tempDir, "snapshots")
	if err := os.Mkdir(snapshotDir, 0700); err != nil {
		t.Errorf("Mkdir() failed = %v", err)
		return
	}
	if err := os.Mkdir(metadataDir, 0700); err != nil {
		t.Errorf("Mkdir() failed = %v", err)
		return
	}

	type fields struct {
		etcdS3     *config.EtcdS3
		controller *Controller
	}
	type args struct {
		ctx          context.Context
		snapshotName string
		snapshotDir  string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Successful Download",
			fields: fields{
				controller: controller,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			args: args{
				ctx:          ctx,
				snapshotName: snapshotName,
				snapshotDir:  snapshotDir,
			},
		},
		{
			name: "Unauthorizied Download",
			fields: fields{
				controller: controller,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "authbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			args: args{
				ctx:          ctx,
				snapshotName: snapshotName,
				snapshotDir:  snapshotDir,
			},
			wantErr: true,
		},
		{
			name: "Nonexistent Bucket",
			fields: fields{
				controller: controller,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "badbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			args: args{
				ctx:          ctx,
				snapshotName: snapshotName,
				snapshotDir:  snapshotDir,
			},
			wantErr: true,
		},
		{
			name: "Nonexistent Snapshot",
			fields: fields{
				controller: controller,
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
			},
			args: args{
				ctx:          ctx,
				snapshotName: "badfile-1",
				snapshotDir:  snapshotDir,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := tt.fields.controller.GetClient(tt.args.ctx, tt.fields.etcdS3)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("GetClient for Client.Upload() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			got, err := c.Download(tt.args.ctx, tt.args.snapshotName, tt.args.snapshotDir)
			t.Logf("Got snapshotPath=%#v err=%v", got, err)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.Download() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("Client.Download() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitClientListSnapshots(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Dummy server with http listener as a simple S3 mock
	server := &http.Server{Handler: s3Router(t)}

	listener, _ := net.Listen("tcp", ":0")

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	listenerAddr := net.JoinHostPort("localhost", port)

	go server.Serve(listener)
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	controller, err := Start(ctx, &config.Control{ClusterReset: true})
	if err != nil {
		t.Errorf("Start() for Client.Download() failed = %v", err)
		return
	}

	type fields struct {
		etcdS3     *config.EtcdS3
		controller *Controller
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    map[string]snapshot.File
		wantErr bool
	}{
		{
			name: "List Snapshots",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx: ctx,
			},
		},
		{
			name: "List Snapshots with Prefix",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Folder:    "testfolder",
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx: ctx,
			},
		},
		{
			name: "Fail to List Snapshots from Nonexistent Bucket",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "badbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx: ctx,
			},
			wantErr: true,
		},
		{
			name: "Fail to List Snapshots from Unauthorized Bucket",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "authbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx: ctx,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := tt.fields.controller.GetClient(tt.args.ctx, tt.fields.etcdS3)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("GetClient for Client.Upload() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			got, err := c.ListSnapshots(tt.args.ctx)
			t.Logf("Got snapshots=%#v err=%v", got, err)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.ListSnapshots() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.ListSnapshots() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

func Test_UnitClientDeleteSnapshot(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Dummy server with http listener as a simple S3 mock
	server := &http.Server{Handler: s3Router(t)}

	listener, _ := net.Listen("tcp", ":0")

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	listenerAddr := net.JoinHostPort("localhost", port)

	go server.Serve(listener)
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	controller, err := Start(ctx, &config.Control{ClusterReset: true})
	if err != nil {
		t.Errorf("Start() for Client.Download() failed = %v", err)
		return
	}

	type fields struct {
		etcdS3     *config.EtcdS3
		controller *Controller
	}
	type args struct {
		ctx context.Context
		key string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Delete Snapshot",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx: ctx,
				key: "snapshot-01",
			},
		},
		{
			name: "Fails to Delete from Nonexistent Bucket",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "badbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx: ctx,
				key: "snapshot-01",
			},
			wantErr: true,
		},
		{
			name: "Fails to Delete from Unauthorized Bucket",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "authbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx: ctx,
				key: "snapshot-01",
			},
			wantErr: true,
		},
		{
			name: "Fails to Delete Nonexistent Snapshot",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx: ctx,
				key: "badfile-1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := tt.fields.controller.GetClient(tt.args.ctx, tt.fields.etcdS3)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("GetClient for Client.DeleteSnapshot() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			err = c.DeleteSnapshot(tt.args.ctx, tt.args.key)
			t.Logf("DeleteSnapshot got error=%v", err)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.DeleteSnapshot() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_UnitClientSnapshotRetention(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Dummy server with http listener as a simple S3 mock
	server := &http.Server{Handler: s3Router(t)}

	listener, _ := net.Listen("tcp", ":0")

	_, port, _ := net.SplitHostPort(listener.Addr().String())
	listenerAddr := net.JoinHostPort("localhost", port)

	go server.Serve(listener)
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	controller, err := Start(ctx, &config.Control{ClusterReset: true})
	if err != nil {
		t.Errorf("Start() for Client.Download() failed = %v", err)
		return
	}

	type fields struct {
		etcdS3     *config.EtcdS3
		controller *Controller
	}
	type args struct {
		ctx       context.Context
		retention int
		prefix    string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "Prune Snapshots - keep all, no folder",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx:       ctx,
				retention: 10,
				prefix:    "snapshot-",
			},
		},
		{
			name: "Prune Snapshots keep 2 of 3, no folder",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx:       ctx,
				retention: 2,
				prefix:    "snapshot-",
			},
			want: []string{"snapshot-03"},
		},
		{
			name: "Prune Snapshots - keep 1 of 3, no folder",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx:       ctx,
				retention: 1,
				prefix:    "snapshot-",
			},
			want: []string{"snapshot-02", "snapshot-03"},
		},
		{
			name: "Prune Snapshots - keep all, with folder",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Folder:    "testfolder",
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx:       ctx,
				retention: 10,
				prefix:    "snapshot-",
			},
		},
		{
			name: "Prune Snapshots keep 2 of 3, with folder",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Folder:    "testfolder",
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx:       ctx,
				retention: 2,
				prefix:    "snapshot-",
			},
			want: []string{"snapshot-06"},
		},
		{
			name: "Prune Snapshots - keep 1 of 3, with folder",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "testbucket",
					Endpoint:  listenerAddr,
					Folder:    "testfolder",
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx:       ctx,
				retention: 1,
				prefix:    "snapshot-",
			},
			want: []string{"snapshot-05", "snapshot-06"},
		},
		{
			name: "Fail to Prune from Unauthorized Bucket",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "authbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx:       ctx,
				retention: 1,
				prefix:    "snapshot-",
			},
			wantErr: true,
		},
		{
			name: "Fail to Prune from Nonexistent Bucket",
			fields: fields{
				etcdS3: &config.EtcdS3{
					AccessKey: "test",
					Bucket:    "badbucket",
					Endpoint:  listenerAddr,
					Insecure:  true,
					Region:    defaultEtcdS3.Region,
					Timeout:   *defaultEtcdS3.Timeout.DeepCopy(),
				},
				controller: controller,
			},
			args: args{
				ctx:       ctx,
				retention: 1,
				prefix:    "snapshot-",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := tt.fields.controller.GetClient(tt.args.ctx, tt.fields.etcdS3)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("GetClient for Client.SnapshotRetention() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			got, err := c.SnapshotRetention(tt.args.ctx, tt.args.retention, tt.args.prefix)
			t.Logf("Got snapshots=%#v err=%v", got, err)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.SnapshotRetention() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.SnapshotRetention() = %+v\nWant = %+v", got, tt.want)
			}
		})
	}
}

//
// Mocks so that we can call Runtime.Core.Core().V1() without a functioning apiserver
//

// explicit interface check for core mock
var _ core.Interface = &coreMock{}

type coreMock struct {
	v1 *v1Mock
}

func newCoreMock(c *gomock.Controller) *coreMock {
	return &coreMock{
		v1: newV1Mock(c),
	}
}

func (m *coreMock) V1() corev1.Interface {
	return m.v1
}

// explicit interface check for core v1 mock
var _ corev1.Interface = &v1Mock{}

type v1Mock struct {
	configMap             *fake.MockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList]
	endpoints             *fake.MockControllerInterface[*v1.Endpoints, *v1.EndpointsList]
	event                 *fake.MockControllerInterface[*v1.Event, *v1.EventList]
	namespace             *fake.MockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList]
	node                  *fake.MockNonNamespacedControllerInterface[*v1.Node, *v1.NodeList]
	persistentVolume      *fake.MockNonNamespacedControllerInterface[*v1.PersistentVolume, *v1.PersistentVolumeList]
	persistentVolumeClaim *fake.MockControllerInterface[*v1.PersistentVolumeClaim, *v1.PersistentVolumeClaimList]
	pod                   *fake.MockControllerInterface[*v1.Pod, *v1.PodList]
	secret                *fake.MockControllerInterface[*v1.Secret, *v1.SecretList]
	service               *fake.MockControllerInterface[*v1.Service, *v1.ServiceList]
	serviceAccount        *fake.MockControllerInterface[*v1.ServiceAccount, *v1.ServiceAccountList]
}

func newV1Mock(c *gomock.Controller) *v1Mock {
	return &v1Mock{
		configMap:             fake.NewMockControllerInterface[*v1.ConfigMap, *v1.ConfigMapList](c),
		endpoints:             fake.NewMockControllerInterface[*v1.Endpoints, *v1.EndpointsList](c),
		event:                 fake.NewMockControllerInterface[*v1.Event, *v1.EventList](c),
		namespace:             fake.NewMockNonNamespacedControllerInterface[*v1.Namespace, *v1.NamespaceList](c),
		node:                  fake.NewMockNonNamespacedControllerInterface[*v1.Node, *v1.NodeList](c),
		persistentVolume:      fake.NewMockNonNamespacedControllerInterface[*v1.PersistentVolume, *v1.PersistentVolumeList](c),
		persistentVolumeClaim: fake.NewMockControllerInterface[*v1.PersistentVolumeClaim, *v1.PersistentVolumeClaimList](c),
		pod:                   fake.NewMockControllerInterface[*v1.Pod, *v1.PodList](c),
		secret:                fake.NewMockControllerInterface[*v1.Secret, *v1.SecretList](c),
		service:               fake.NewMockControllerInterface[*v1.Service, *v1.ServiceList](c),
		serviceAccount:        fake.NewMockControllerInterface[*v1.ServiceAccount, *v1.ServiceAccountList](c),
	}
}

func (m *v1Mock) ConfigMap() corev1.ConfigMapController {
	return m.configMap
}

func (m *v1Mock) Endpoints() corev1.EndpointsController {
	return m.endpoints
}

func (m *v1Mock) Event() corev1.EventController {
	return m.event
}

func (m *v1Mock) Namespace() corev1.NamespaceController {
	return m.namespace
}

func (m *v1Mock) Node() corev1.NodeController {
	return m.node
}

func (m *v1Mock) PersistentVolume() corev1.PersistentVolumeController {
	return m.persistentVolume
}

func (m *v1Mock) PersistentVolumeClaim() corev1.PersistentVolumeClaimController {
	return m.persistentVolumeClaim
}

func (m *v1Mock) Pod() corev1.PodController {
	return m.pod
}

func (m *v1Mock) Secret() corev1.SecretController {
	return m.secret
}

func (m *v1Mock) Service() corev1.ServiceController {
	return m.service
}

func (m *v1Mock) ServiceAccount() corev1.ServiceAccountController {
	return m.serviceAccount
}

func errorNotFound(gv, name string) error {
	return apierrors.NewNotFound(schema.ParseGroupResource(gv), name)
}

//
//  ListObjects response body template
//

var listObjectsV2ResponseTemplate = `
{{- /* */ -}}
{{ with $b := . -}}
<ListBucketResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">
  <Name>{{$b.Name}}</Name>
	{{ if $b.Prefix }}<Prefix>{{$b.Prefix}}</Prefix>{{ else }}<Prefix/>{{ end }}
  <KeyCount>{{ len $b.Objects }}</KeyCount>
  <MaxKeys>1000</MaxKeys>
  <Delimiter/>
  <IsTruncated>false</IsTruncated>
  {{- range $o := $b.Objects }}
  <Contents>
    <Key>{{ $o.Key }}</Key>
    <LastModified>{{ $o.LastModified }}</LastModified>
    <ETag>{{ printf "%q" $o.ETag }}</ETag>
    <Size>{{ $o.Size }}</Size>
    <Owner>
      <ID>0</ID>
      <DisplayName>test</DisplayName>
    </Owner>
    <StorageClass>STANDARD</StorageClass>
  </Contents>
  {{- end }}
  <EncodingType>url</EncodingType>
</ListBucketResult>
{{- end }}
`

func s3Router(t *testing.T) http.Handler {
	var listResponse = template.Must(template.New("listObjectsV2").Parse(listObjectsV2ResponseTemplate))

	type object struct {
		Key          string
		LastModified string
		ETag         string
		Size         int
	}

	type bucket struct {
		Name    string
		Prefix  string
		Objects []object
	}

	snapshotId := 0
	objects := []object{}
	timestamp := time.Now().Format(time.RFC3339)
	for _, prefix := range []string{"", "testfolder", "testfolder/netsted", "otherfolder"} {
		for idx := range []int{0, 1, 2} {
			snapshotId++
			objects = append(objects, object{
				Key:          path.Join(prefix, fmt.Sprintf("snapshot-%02d", snapshotId)),
				LastModified: timestamp,
				ETag:         "0000",
				Size:         100,
			})
			if idx != 0 {
				objects = append(objects, object{
					Key:          path.Join(prefix, fmt.Sprintf(".metadata/snapshot-%02d", snapshotId)),
					LastModified: timestamp,
					ETag:         "0000",
					Size:         10,
				})
			}
		}
	}

	// badbucket returns 404 for all requests
	// authbucket returns 200 for HeadBucket, 403 for all others
	// others return 200 for objects with name prefix snapshot, 404 for all others
	router := mux.NewRouter().SkipClean(true)
	// HeadBucket
	router.Path("/{bucket}/").Methods(http.MethodHead).HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch vars["bucket"] {
		case "badbucket":
			rw.WriteHeader(http.StatusNotFound)
		}
	})
	// ListObjectsV2
	router.Path("/{bucket}/").Methods(http.MethodGet).HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch vars["bucket"] {
		case "badbucket":
			rw.WriteHeader(http.StatusNotFound)
		case "authbucket":
			rw.WriteHeader(http.StatusForbidden)
		default:
			prefix := r.URL.Query().Get("prefix")
			filtered := []object{}
			for _, object := range objects {
				if strings.HasPrefix(object.Key, prefix) {
					filtered = append(filtered, object)
				}
			}
			if err := listResponse.Execute(rw, bucket{Name: vars["bucket"], Prefix: prefix, Objects: filtered}); err != nil {
				t.Errorf("Failed to generate ListObjectsV2 response, error = %v", err)
				rw.WriteHeader(http.StatusInternalServerError)
			}
		}
	})
	// HeadObject - snapshot
	router.Path("/{bucket}/{prefix:.*}snapshot-{snapshot}").Methods(http.MethodHead).HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch vars["bucket"] {
		case "badbucket":
			rw.WriteHeader(http.StatusNotFound)
		case "authbucket":
			rw.WriteHeader(http.StatusForbidden)
		default:
			rw.Header().Add("last-modified", time.Now().In(gmt).Format(time.RFC1123))
		}
	})
	// GetObject - snapshot
	router.Path("/{bucket}/{prefix:.*}snapshot-{snapshot}").Methods(http.MethodGet).HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch vars["bucket"] {
		case "badbucket":
			rw.WriteHeader(http.StatusNotFound)
		case "authbucket":
			rw.WriteHeader(http.StatusForbidden)
		default:
			rw.Header().Add("last-modified", time.Now().In(gmt).Format(time.RFC1123))
			rw.Write([]byte("test snapshot file\n"))
		}
	})
	// PutObject/DeleteObject - snapshot
	router.Path("/{bucket}/{prefix:.*}snapshot-{snapshot}").Methods(http.MethodPut, http.MethodDelete).HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch vars["bucket"] {
		case "badbucket":
			rw.WriteHeader(http.StatusNotFound)
		case "authbucket":
			rw.WriteHeader(http.StatusForbidden)
		default:
			if r.Method == http.MethodDelete {
				rw.WriteHeader(http.StatusNoContent)
			}
		}
	})
	// HeadObject - snapshot metadata
	router.Path("/{bucket}/{prefix:.*}.metadata/snapshot-{snapshot}").Methods(http.MethodHead).HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch vars["bucket"] {
		case "badbucket":
			rw.WriteHeader(http.StatusNotFound)
		case "authbucket":
			rw.WriteHeader(http.StatusForbidden)
		default:
			rw.Header().Add("last-modified", time.Now().In(gmt).Format(time.RFC1123))
		}
	})
	// GetObject - snapshot metadata
	router.Path("/{bucket}/{prefix:.*}.metadata/snapshot-{snapshot}").Methods(http.MethodGet).HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch vars["bucket"] {
		case "badbucket":
			rw.WriteHeader(http.StatusNotFound)
		case "authbucket":
			rw.WriteHeader(http.StatusForbidden)
		default:
			rw.Header().Add("last-modified", time.Now().In(gmt).Format(time.RFC1123))
			rw.Write([]byte("test snapshot metadata\n"))
		}
	})
	// PutObject/DeleteObject - snapshot metadata
	router.Path("/{bucket}/{prefix:.*}.metadata/snapshot-{snapshot}").Methods(http.MethodPut, http.MethodDelete).HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		switch vars["bucket"] {
		case "badbucket":
			rw.WriteHeader(http.StatusNotFound)
		case "authbucket":
			rw.WriteHeader(http.StatusForbidden)
		default:
			if r.Method == http.MethodDelete {
				rw.WriteHeader(http.StatusNoContent)
			}
		}
	})
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		logrus.Infof("%s %s://%s %s", r.Method, scheme, r.Host, r.URL)
		router.ServeHTTP(rw, r)
	})
}
