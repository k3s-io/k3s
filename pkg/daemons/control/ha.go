package control

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"encoding/base64"

	"github.com/rancher/k3s/pkg/daemons/config"
	"go.etcd.io/etcd/clientv3"
)

const (
	etcdDialTimeout    = 5 * time.Second
	k3sRuntimeEtcdPath = "/k3s/runtime"
)

type serverHA struct {
	ServerCAData           string `json:"serverCAData,omitempty"`
	ServerCAKeyData        string `json:"serverCAKeyData,omitempty"`
	ClientCAData           string `json:"clientCAData,omitempty"`
	ClientCAKeyData        string `json:"clientCAKeyData,omitempty"`
	ServiceKeyData         string `json:"serviceKeyData,omitempty"`
	PasswdFileData         string `json:"passwdFileData,omitempty"`
	RequestHeaderCAData    string `json:"requestHeaderCAData,omitempty"`
	RequestHeaderCAKeyData string `json:"requestHeaderCAKeyData,omitempty"`
}

func setHAData(cfg *config.Control) error {
	if cfg.StorageBackend != "etcd3" || cfg.CertStorageBackend != "etcd3" {
		return nil
	}
	tlsConfig, err := genTLSConfig(cfg)
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

	gr, err := cli.Get(context.TODO(), k3sRuntimeEtcdPath)
	if err != nil {
		return err
	}
	if len(gr.Kvs) > 0 && string(gr.Kvs[0].Value) != "" {
		return nil
	}
	certData, err := readRuntimeCertData(cfg.Runtime)
	if err != nil {
		return err
	}

	runtimeBase64 := base64.StdEncoding.EncodeToString(certData)
	_, err = cli.Put(context.TODO(), k3sRuntimeEtcdPath, runtimeBase64)
	if err != nil {
		return err
	}

	return nil
}

func getHAData(cfg *config.Control) error {
	serverRuntime := &serverHA{}
	if cfg.StorageBackend != "etcd3" || cfg.CertStorageBackend != "etcd3" {
		return nil
	}
	tlsConfig, err := genTLSConfig(cfg)
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

	gr, err := cli.Get(context.TODO(), k3sRuntimeEtcdPath)
	if err != nil {
		return err
	}
	if len(gr.Kvs) == 0 {
		return nil
	}

	runtimeJSON, err := base64.URLEncoding.DecodeString(string(gr.Kvs[0].Value))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(runtimeJSON, serverRuntime); err != nil {
		return err
	}
	return writeRuntimeCertData(cfg.Runtime, serverRuntime)
}

func genTLSConfig(cfg *config.Control) (*tls.Config, error) {
	tlsConfig := &tls.Config{}
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
		tlsConfig.Certificates = []tls.Certificate{tlsCert}
	}
	if cfg.StorageCAFile != "" {
		caData, err := ioutil.ReadFile(cfg.StorageCAFile)
		if err != nil {
			return nil, err
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caData)
		tlsConfig.RootCAs = certPool
	}
	return tlsConfig, nil
}

func readRuntimeCertData(runtime *config.ControlRuntime) ([]byte, error) {
	serverHACerts := map[string]string{
		runtime.ServerCA:           "",
		runtime.ServerCAKey:        "",
		runtime.ClientCA:           "",
		runtime.ClientCAKey:        "",
		runtime.ServiceKey:         "",
		runtime.PasswdFile:         "",
		runtime.RequestHeaderCA:    "",
		runtime.RequestHeaderCAKey: "",
	}
	for k := range serverHACerts {
		data, err := ioutil.ReadFile(k)
		if err != nil {
			return nil, err
		}
		serverHACerts[k] = string(data)
	}
	serverHACertsData := &serverHA{
		ServerCAData:           serverHACerts[runtime.ServerCA],
		ServerCAKeyData:        serverHACerts[runtime.ServerCAKey],
		ClientCAData:           serverHACerts[runtime.ClientCA],
		ClientCAKeyData:        serverHACerts[runtime.ClientCAKey],
		ServiceKeyData:         serverHACerts[runtime.ServiceKey],
		PasswdFileData:         serverHACerts[runtime.PasswdFile],
		RequestHeaderCAData:    serverHACerts[runtime.RequestHeaderCA],
		RequestHeaderCAKeyData: serverHACerts[runtime.RequestHeaderCAKey],
	}
	return json.Marshal(serverHACertsData)
}

func writeRuntimeCertData(runtime *config.ControlRuntime, runtimeData *serverHA) error {
	runtimePathValue := map[string]string{
		runtime.ServerCA:           runtimeData.ServerCAData,
		runtime.ServerCAKey:        runtimeData.ServerCAKeyData,
		runtime.ClientCA:           runtimeData.ClientCAData,
		runtime.ClientCAKey:        runtimeData.ClientCAKeyData,
		runtime.ServiceKey:         runtimeData.ServiceKeyData,
		runtime.PasswdFile:         runtimeData.PasswdFileData,
		runtime.RequestHeaderCA:    runtimeData.RequestHeaderCAData,
		runtime.RequestHeaderCAKey: runtimeData.RequestHeaderCAKeyData,
	}
	for k, v := range runtimePathValue {
		if _, err := os.Stat(k); os.IsNotExist(err) {
			if err := ioutil.WriteFile(k, []byte(v), 600); err != nil {
				return err
			}
		}
	}
	return nil
}
