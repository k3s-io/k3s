package control

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"encoding/base64"

	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/clientv3"
)

const (
	etcdDialTimeout    = 5 * time.Second
	k3sRuntimeEtcdPath = "/k3s/runtime"
	bootstrapTypeNone  = "none"
	bootstrapTypeRead  = "read"
	bootstrapTypeWrite = "write"
	bootstrapTypeFull  = "full"
)

type serverBootstrap struct {
	ServerCAData           string `json:"serverCAData,omitempty"`
	ServerCAKeyData        string `json:"serverCAKeyData,omitempty"`
	ClientCAData           string `json:"clientCAData,omitempty"`
	ClientCAKeyData        string `json:"clientCAKeyData,omitempty"`
	ServiceKeyData         string `json:"serviceKeyData,omitempty"`
	PasswdFileData         string `json:"passwdFileData,omitempty"`
	RequestHeaderCAData    string `json:"requestHeaderCAData,omitempty"`
	RequestHeaderCAKeyData string `json:"requestHeaderCAKeyData,omitempty"`
	ClientKubeletKey       string `json:"clientKubeletKey,omitempty"`
	ClientKubeProxyKey     string `json:"clientKubeProxyKey,omitempty"`
	ServingKubeletKey      string `json:"servingKubeletKey,omitempty"`
}

var validBootstrapTypes = map[string]bool{
	bootstrapTypeRead:  true,
	bootstrapTypeWrite: true,
	bootstrapTypeFull:  true,
}

// fetchBootstrapData copies the bootstrap data (certs, keys, passwords)
// from etcd to inidividual files specified by cfg.Runtime.
func fetchBootstrapData(cfg *config.Control) error {
	if valid, err := checkBootstrapArgs(cfg, map[string]bool{
		bootstrapTypeFull: true,
		bootstrapTypeRead: true,
	}); !valid {
		if err != nil {
			logrus.Warnf("Not fetching bootstrap data: %v", err)
		}
		return nil
	}

	tlsConfig, err := genBootstrapTLSConfig(cfg)
	if err != nil {
		return err
	}

	endpoints := strings.Split(cfg.StorageEndpoint, ",")
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: etcdDialTimeout,
		TLS:         tlsConfig,
	})
	if err != nil {
		return err
	}
	defer cli.Close()

	logrus.Info("Fetching bootstrap data from etcd")
	gr, err := cli.Get(context.TODO(), k3sRuntimeEtcdPath)
	if err != nil {
		return err
	}
	if len(gr.Kvs) == 0 {
		if cfg.BootstrapType != bootstrapTypeRead {
			return nil
		}
		return errors.New("Unable to read bootstrap data from server")
	}

	runtimeJSON, err := base64.URLEncoding.DecodeString(string(gr.Kvs[0].Value))
	if err != nil {
		return err
	}
	serverRuntime := &serverBootstrap{}
	if err := json.Unmarshal(runtimeJSON, serverRuntime); err != nil {
		return err
	}
	return writeRuntimeBootstrapData(cfg.Runtime, serverRuntime)
}

// storeBootstrapData copies the bootstrap data in the opposite direction to
// fetchBootstrapData.
func storeBootstrapData(cfg *config.Control) error {
	if valid, err := checkBootstrapArgs(cfg, map[string]bool{
		bootstrapTypeFull:  true,
		bootstrapTypeWrite: true,
	}); !valid {
		if err != nil {
			logrus.Warnf("Not storing boostrap data: %v", err)
		}
		return nil
	}

	tlsConfig, err := genBootstrapTLSConfig(cfg)
	if err != nil {
		return err
	}

	endpoints := strings.Split(cfg.StorageEndpoint, ",")
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: etcdDialTimeout,
		TLS:         tlsConfig,
	})
	if err != nil {
		return err
	}
	defer cli.Close()

	if cfg.BootstrapType != bootstrapTypeWrite {
		gr, err := cli.Get(context.TODO(), k3sRuntimeEtcdPath)
		if err != nil {
			return err
		}
		if len(gr.Kvs) > 0 && string(gr.Kvs[0].Value) != "" {
			return nil
		}
	}

	certData, err := readRuntimeBootstrapData(cfg.Runtime)
	if err != nil {
		return err
	}

	logrus.Info("Storing bootstrap data to etcd")
	runtimeBase64 := base64.StdEncoding.EncodeToString(certData)
	_, err = cli.Put(context.TODO(), k3sRuntimeEtcdPath, runtimeBase64)
	if err != nil {
		return err
	}

	return nil
}

func checkBootstrapArgs(cfg *config.Control, accepted map[string]bool) (bool, error) {
	if cfg.BootstrapType == "" || cfg.BootstrapType == bootstrapTypeNone {
		return false, nil
	}
	if !validBootstrapTypes[cfg.BootstrapType] {
		return false, fmt.Errorf("unsupported bootstrap type [%s]", cfg.BootstrapType)
	}
	if cfg.StorageBackend != "etcd3" {
		return false, errors.New("bootstrap only supported with etcd3 as storage backend")
	}
	if !accepted[cfg.BootstrapType] {
		return false, nil
	}
	return true, nil
}

func genBootstrapTLSConfig(cfg *config.Control) (*tls.Config, error) {
	secureTLSConfig := &tls.Config{}
	// Note: clientv3 excepts nil for non-tls
	var tlsConfig *tls.Config
	if cfg.StorageCertFile != "" && cfg.StorageKeyFile != "" {
		certPem, err := ioutil.ReadFile(cfg.StorageCertFile)
		if err != nil {
			return nil, err
		}
		keyPem, err := ioutil.ReadFile(cfg.StorageKeyFile)
		if err != nil {
			return nil, err
		}
		tlsCert, err := tls.X509KeyPair(certPem, keyPem)
		if err != nil {
			return nil, err
		}
		tlsConfig = secureTLSConfig
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}
	if cfg.StorageCAFile != "" {
		caData, err := ioutil.ReadFile(cfg.StorageCAFile)
		if err != nil {
			return nil, err
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caData)
		tlsConfig = secureTLSConfig
		tlsConfig.RootCAs = certPool
	}
	return tlsConfig, nil
}

func readRuntimeBootstrapData(runtime *config.ControlRuntime) ([]byte, error) {
	serverBootstrapFiles := map[string]string{
		runtime.ServerCA:           "",
		runtime.ServerCAKey:        "",
		runtime.ClientCA:           "",
		runtime.ClientCAKey:        "",
		runtime.ServiceKey:         "",
		runtime.PasswdFile:         "",
		runtime.RequestHeaderCA:    "",
		runtime.RequestHeaderCAKey: "",
		runtime.ClientKubeletKey:   "",
		runtime.ClientKubeProxyKey: "",
		runtime.ServingKubeletKey:  "",
	}
	for k := range serverBootstrapFiles {
		data, err := ioutil.ReadFile(k)
		if err != nil {
			return nil, err
		}
		serverBootstrapFiles[k] = string(data)
	}
	serverBootstrapFileData := &serverBootstrap{
		ServerCAData:           serverBootstrapFiles[runtime.ServerCA],
		ServerCAKeyData:        serverBootstrapFiles[runtime.ServerCAKey],
		ClientCAData:           serverBootstrapFiles[runtime.ClientCA],
		ClientCAKeyData:        serverBootstrapFiles[runtime.ClientCAKey],
		ServiceKeyData:         serverBootstrapFiles[runtime.ServiceKey],
		PasswdFileData:         serverBootstrapFiles[runtime.PasswdFile],
		RequestHeaderCAData:    serverBootstrapFiles[runtime.RequestHeaderCA],
		RequestHeaderCAKeyData: serverBootstrapFiles[runtime.RequestHeaderCAKey],
		ClientKubeletKey:       serverBootstrapFiles[runtime.ClientKubeletKey],
		ClientKubeProxyKey:     serverBootstrapFiles[runtime.ClientKubeProxyKey],
		ServingKubeletKey:      serverBootstrapFiles[runtime.ServingKubeletKey],
	}
	return json.Marshal(serverBootstrapFileData)
}

func writeRuntimeBootstrapData(runtime *config.ControlRuntime, runtimeData *serverBootstrap) error {
	runtimePathValue := map[string]string{
		runtime.ServerCA:           runtimeData.ServerCAData,
		runtime.ServerCAKey:        runtimeData.ServerCAKeyData,
		runtime.ClientCA:           runtimeData.ClientCAData,
		runtime.ClientCAKey:        runtimeData.ClientCAKeyData,
		runtime.ServiceKey:         runtimeData.ServiceKeyData,
		runtime.PasswdFile:         runtimeData.PasswdFileData,
		runtime.RequestHeaderCA:    runtimeData.RequestHeaderCAData,
		runtime.RequestHeaderCAKey: runtimeData.RequestHeaderCAKeyData,
		runtime.ClientKubeletKey:   runtimeData.ClientKubeletKey,
		runtime.ClientKubeProxyKey: runtimeData.ClientKubeProxyKey,
		runtime.ServingKubeletKey:  runtimeData.ServingKubeletKey,
	}
	for k, v := range runtimePathValue {
		if _, err := os.Stat(k); os.IsNotExist(err) {
			if err := ioutil.WriteFile(k, []byte(v), 0600); err != nil {
				return err
			}
		}
	}
	return nil
}
