package dqlite

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/go-dqlite"
	"github.com/canonical/go-dqlite/client"
	"github.com/pkg/errors"
	controllerclient "github.com/rancher/k3s/pkg/dqlite/controller/client"
	"github.com/rancher/k3s/pkg/dqlite/dialer"
	dqlitedriver "github.com/rancher/kine/pkg/drivers/dqlite"
	v1 "github.com/rancher/wrangler-api/pkg/generated/controllers/core/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/net"
)

const (
	PeersFile  = "peers.db"
	NodeIDFile = "node-id"
)

var (
	ignoreFile = map[string]bool{
		PeersFile:  true,
		NodeIDFile: true,
	}
)

type Certs struct {
	ServerTrust *x509.Certificate
	ClientTrust *x509.Certificate
	ClientCert  tls.Certificate
}

type DQLite struct {
	ClientCA             string
	ClientCAKey          string
	ClientCert           string
	ClientCertKey        string
	ServerCA             string
	ServerCAKey          string
	AdvertiseIP          string
	AdvertisePort        int
	DataDir              string
	NodeStore            client.NodeStore
	NodeInfo             client.NodeInfo
	node                 *dqlite.Node
	StorageEndpoint      string
	NodeControllerGetter NodeControllerGetter
	clientOpts           []client.Option
}

type NodeControllerGetter func() v1.NodeController

func New(dataDir, advertiseIP string, advertisePort int, getter NodeControllerGetter) *DQLite {
	return &DQLite{
		AdvertiseIP:          advertiseIP,
		AdvertisePort:        advertisePort,
		DataDir:              dataDir,
		NodeControllerGetter: getter,
	}
}

func (d *DQLite) Start(ctx context.Context, initCluster, resetCluster bool, certs *Certs, next http.Handler) (http.Handler, error) {
	bindAddress := d.getBindAddress()

	clientTLSConfig, err := getClientTLSConfig(certs.ClientCert, certs.ServerTrust)
	if err != nil {
		return nil, err
	}

	advertise, err := getAdvertiseAddress(d.AdvertiseIP, d.AdvertisePort)
	if err != nil {
		return nil, errors.Wrap(err, "get advertise address")
	}

	dial, err := getDialer(advertise, bindAddress, clientTLSConfig)
	if err != nil {
		return nil, err
	}

	dqlitedriver.Dialer = dial
	dqlitedriver.Logger = log()

	d.clientOpts = append(d.clientOpts, client.WithDialFunc(dial), client.WithLogFunc(log()))

	nodeInfo, node, err := getNode(d.DataDir, advertise, bindAddress, initCluster, dial)
	if err != nil {
		return nil, err
	}

	d.NodeInfo = nodeInfo
	d.node = node

	go func() {
		<-ctx.Done()
		node.Close()
	}()

	if err := d.nodeStore(ctx, initCluster); err != nil {
		return nil, err
	}

	go d.startController(ctx)

	if !resetCluster {
		if err := node.Start(); err != nil {
			return nil, err
		}
	}

	return router(ctx, next, nodeInfo, certs.ClientTrust, "kube-apiserver", bindAddress), nil
}

func (d *DQLite) startController(ctx context.Context) {
	for {
		if nc := d.NodeControllerGetter(); nc != nil {
			if os.Getenv("NODE_NAME") == "" {
				logrus.Errorf("--disable-agent is not compatible with dqlite")
			} else {
				break
			}
		}
		time.Sleep(time.Second)
	}

	controllerclient.Register(ctx, os.Getenv("NODE_NAME"), d.NodeInfo, d.NodeStore, d.NodeControllerGetter(), d.clientOpts)
}

func (d *DQLite) nodeStore(ctx context.Context, initCluster bool) error {
	peerDB := filepath.Join(GetDBDir(d.DataDir), PeersFile)
	ns, err := client.DefaultNodeStore(peerDB)
	if err != nil {
		return err
	}
	d.NodeStore = ns
	d.StorageEndpoint = fmt.Sprintf("dqlite://?peer-file=%s", peerDB)
	if initCluster {
		if err := dqlitedriver.AddPeers(ctx, d.NodeStore, d.NodeInfo); err != nil {
			return err
		}
	}
	return nil
}

func getAdvertiseAddress(advertiseIP string, advertisePort int) (string, error) {
	ip := advertiseIP
	if ip == "" {
		ipAddr, err := net.ChooseHostInterface()
		if err != nil {
			return "", err
		}
		ip = ipAddr.String()
	}

	return fmt.Sprintf("%s:%d", ip, advertisePort), nil
}

func getClientTLSConfig(cert tls.Certificate, ca *x509.Certificate) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		RootCAs: x509.NewCertPool(),
		Certificates: []tls.Certificate{
			cert,
		},
		ServerName: "kubernetes",
	}
	tlsConfig.RootCAs.AddCert(ca)

	return tlsConfig, nil
}

func getDialer(advertiseAddress, bindAddress string, tlsConfig *tls.Config) (client.DialFunc, error) {
	return dialer.NewHTTPDialer(advertiseAddress, bindAddress, tlsConfig)
}

func GetDBDir(dataDir string) string {
	return filepath.Join(dataDir, "db", "state.dqlite")
}

func getNode(dataDir string, advertiseAddress, bindAddress string, initCluster bool, dial client.DialFunc) (dqlite.NodeInfo, *dqlite.Node, error) {
	id, err := getClusterID(initCluster, dataDir)
	if err != nil {
		return dqlite.NodeInfo{}, nil, errors.Wrap(err, "reading cluster id")
	}

	dbDir := GetDBDir(dataDir)

	node, err := dqlite.New(id, advertiseAddress, dbDir,
		dqlite.WithBindAddress(bindAddress),
		dqlite.WithDialFunc(dial),
		dqlite.WithNetworkLatency(20*time.Millisecond))
	return dqlite.NodeInfo{
		ID:      id,
		Address: advertiseAddress,
	}, node, err
}

func writeClusterID(id uint64, dataDir string) error {
	idFile := filepath.Join(GetDBDir(dataDir), NodeIDFile)
	if err := os.MkdirAll(filepath.Dir(idFile), 0700); err != nil {
		return err
	}
	return ioutil.WriteFile(idFile, []byte(strconv.FormatUint(id, 10)), 0644)
}

func getClusterID(initCluster bool, dataDir string) (uint64, error) {
	idFile := filepath.Join(GetDBDir(dataDir), NodeIDFile)
	content, err := ioutil.ReadFile(idFile)
	if os.IsNotExist(err) {
		content = nil
	} else if err != nil {
		return 0, err
	}

	idStr := strings.TrimSpace(string(content))
	if idStr == "" {
		id := rand.Uint64()
		if initCluster {
			id = 1
		}
		return id, writeClusterID(id, dataDir)
	}

	return strconv.ParseUint(idStr, 10, 64)
}

func (d *DQLite) getBindAddress() string {
	// only anonymous works???
	return "@" + filepath.Join(GetDBDir(d.DataDir), "dqlite.sock")
}
