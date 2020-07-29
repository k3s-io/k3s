package etcd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	certutil "github.com/rancher/dynamiclistener/cert"
	"github.com/rancher/k3s/pkg/clientaccess"
	"github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/sirupsen/logrus"
	etcd "go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/etcdserver/etcdserverpb"
	utilnet "k8s.io/apimachinery/pkg/util/net"
)

type ETCD struct {
	client  *etcd.Client
	config  *config.Control
	name    string
	runtime *config.ControlRuntime
	address string
}

type Members struct {
	Members []*etcdserverpb.Member `json:"members"`
}

func (e *ETCD) EndpointName() string {
	return "etcd"
}

func (e *ETCD) Test(ctx context.Context, clientAccessInfo *clientaccess.Info) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	status, err := e.client.Status(ctx, "https://127.0.0.1:2379")
	if err != nil {
		return err
	}

	if status.IsLearner {
		if err := e.promoteMember(ctx, clientAccessInfo); err != nil {
			return err
		}
	}
	members, err := e.client.MemberList(ctx)
	if err != nil {
		return err
	}

	var cluster []string
	for _, member := range members.Members {
		for _, peerURL := range member.PeerURLs {
			if peerURL == e.peerURL() && e.name == member.Name {
				return nil
			}
		}
		if len(member.PeerURLs) > 0 {
			cluster = append(cluster, fmt.Sprintf("%s=%s", member.Name, member.PeerURLs[0]))
		}
	}
	msg := fmt.Sprintf("This server is a not a member of the etcd cluster "+"found %v and expecting to contain %s=%s", cluster, e.name, e.address)
	logrus.Error(msg)
	return fmt.Errorf(msg)
}

func walDir(config *config.Control) string {
	return filepath.Join(dataDir(config), "member", "wal")
}

func dataDir(config *config.Control) string {
	return filepath.Join(config.DataDir, "db", "etcd")
}

func nameFile(config *config.Control) string {
	return filepath.Join(dataDir(config), "name")
}

func (e *ETCD) IsInitialized(ctx context.Context, config *config.Control) (bool, error) {
	if s, err := os.Stat(walDir(config)); err == nil && s.IsDir() {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, errors.Wrapf(err, "failed to test if etcd is initialized")
	}
}

func (e *ETCD) Reset(ctx context.Context, clientAccessInfo *clientaccess.Info) error {
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for range t.C {
			if err := e.Test(ctx, clientAccessInfo); err == nil {
				members, err := e.client.MemberList(ctx)
				if err != nil {
					continue
				}

				if len(members.Members) == 1 && members.Members[0].Name == e.name {
					logrus.Infof("etcd is running, restart without --cluster-reset flag now. Backup and delete ${datadir}/server/db on each peer etcd server and rejoin the nodes")
					os.Exit(0)
				}
			}
		}
	}()
	return e.newCluster(ctx, true)
}

func (e *ETCD) Start(ctx context.Context, clientAccessInfo *clientaccess.Info) error {
	existingCluster, err := e.IsInitialized(ctx, e.config)
	if err != nil {
		return errors.Wrapf(err, "failed to validation")
	}

	e.config.Runtime.ClusterControllerStart = func(ctx context.Context) error {
		Register(ctx, e, e.config.Runtime.Core.Core().V1().Node())
		return nil
	}

	if existingCluster {
		opt, err := executor.CurrentETCDOptions()
		if err != nil {
			return err
		}
		return e.cluster(ctx, false, opt)
	}

	if clientAccessInfo == nil {
		return e.newCluster(ctx, false)
	}
	err = e.join(ctx, clientAccessInfo)
	return errors.Wrap(err, "joining etcd cluster")
}

func (e *ETCD) join(ctx context.Context, clientAccessInfo *clientaccess.Info) error {
	clientURLs, memberList, err := e.clientURLs(ctx, clientAccessInfo)
	if err != nil {
		return err
	}

	client, err := joinClient(ctx, e.runtime, clientURLs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var (
		cluster []string
		add     = true
	)

	members, err := client.MemberList(ctx)
	if err != nil {
		logrus.Errorf("failed to get member list from cluster, will assume this member is already added")
		members = &etcd.MemberListResponse{
			Members: append(memberList.Members, &etcdserverpb.Member{
				Name:     e.name,
				PeerURLs: []string{e.peerURL()},
			}),
		}
		add = false
	}

	for _, member := range members.Members {
		for _, peer := range member.PeerURLs {
			u, err := url.Parse(peer)
			if err != nil {
				return err
			}
			// An uninitialized member won't have a name
			if u.Hostname() == e.address && (member.Name == e.name || member.Name == "") {
				add = false
			}
			if member.Name == "" && u.Hostname() == e.address {
				member.Name = e.name
			}
			if len(member.PeerURLs) > 0 {
				cluster = append(cluster, fmt.Sprintf("%s=%s", member.Name, member.PeerURLs[0]))
			}
		}
	}

	if add {
		logrus.Infof("Adding %s to etcd cluster %v", e.peerURL(), cluster)
		if _, err = client.MemberAddAsLearner(ctx, []string{e.peerURL()}); err != nil {
			return err
		}
		cluster = append(cluster, fmt.Sprintf("%s=%s", e.name, e.peerURL()))
	}

	go e.promoteMember(ctx, clientAccessInfo)

	logrus.Infof("Starting etcd for cluster %v", cluster)
	return e.cluster(ctx, false, executor.InitialOptions{
		Cluster: strings.Join(cluster, ","),
		State:   "existing",
	})
}

func (e *ETCD) Register(ctx context.Context, config *config.Control, l net.Listener, handler http.Handler) (net.Listener, http.Handler, error) {
	e.config = config
	e.runtime = config.Runtime

	client, err := newClient(ctx, e.runtime)
	if err != nil {
		return nil, nil, err
	}
	e.client = client

	address, err := getAdvertiseAddress(config.AdvertiseIP)
	if err != nil {
		return nil, nil, err
	}
	e.address = address

	e.config.Datastore.Endpoint = "https://127.0.0.1:2379"
	e.config.Datastore.Config.CAFile = e.runtime.ETCDServerCA
	e.config.Datastore.Config.CertFile = e.runtime.ClientETCDCert
	e.config.Datastore.Config.KeyFile = e.runtime.ClientETCDKey

	if err := e.setName(); err != nil {
		return nil, nil, err
	}

	return l, e.handler(handler), err
}

func (e *ETCD) setName() error {
	fileName := nameFile(e.config)
	data, err := ioutil.ReadFile(fileName)
	if os.IsNotExist(err) {
		h, err := os.Hostname()
		if err != nil {
			return err
		}
		e.name = strings.SplitN(h, ".", 2)[0] + "-" + uuid.New().String()[:8]
		if err := os.MkdirAll(filepath.Dir(fileName), 0755); err != nil {
			return err
		}
		return ioutil.WriteFile(fileName, []byte(e.name), 0655)
	} else if err != nil {
		return err
	}
	e.name = string(data)
	return nil
}

func (e *ETCD) handler(next http.Handler) http.Handler {
	mux := mux.NewRouter()
	mux.Handle("/db/info", e.infoHandler())
	mux.NotFoundHandler = next
	return mux
}

func (e *ETCD) infoHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()

		members, err := e.client.MemberList(ctx)
		if err != nil {
			json.NewEncoder(rw).Encode(&Members{
				Members: []*etcdserverpb.Member{
					{
						Name:       e.name,
						PeerURLs:   []string{e.peerURL()},
						ClientURLs: []string{e.clientURL()},
					},
				},
			})
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(&Members{
			Members: members.Members,
		})
	})
}

func joinClient(ctx context.Context, runtime *config.ControlRuntime, peers []string) (*etcd.Client, error) {
	tlsConfig, err := toTLSConfig(runtime)
	if err != nil {
		return nil, err
	}

	cfg := etcd.Config{
		Endpoints: peers,
		TLS:       tlsConfig,
		Context:   ctx,
	}

	return etcd.New(cfg)
}

func newClient(ctx context.Context, runtime *config.ControlRuntime) (*etcd.Client, error) {
	tlsConfig, err := toTLSConfig(runtime)
	if err != nil {
		return nil, err
	}

	cfg := etcd.Config{
		Context:   ctx,
		Endpoints: []string{"https://127.0.0.1:2379"},
		TLS:       tlsConfig,
	}

	return etcd.New(cfg)
}

func toTLSConfig(runtime *config.ControlRuntime) (*tls.Config, error) {
	clientCert, err := tls.LoadX509KeyPair(runtime.ClientETCDCert, runtime.ClientETCDKey)
	if err != nil {
		return nil, err
	}

	pool, err := certutil.NewPool(runtime.ETCDServerCA)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{clientCert},
	}, nil
}

func getAdvertiseAddress(advertiseIP string) (string, error) {
	ip := advertiseIP
	if ip == "" {
		ipAddr, err := utilnet.ChooseHostInterface()
		if err != nil {
			return "", err
		}
		ip = ipAddr.String()
	}

	return ip, nil
}

func (e *ETCD) newCluster(ctx context.Context, reset bool) error {
	return e.cluster(ctx, reset, executor.InitialOptions{
		AdvertisePeerURL: fmt.Sprintf("https://%s:2380", e.address),
		Cluster:          fmt.Sprintf("%s=https://%s:2380", e.name, e.address),
		State:            "new",
	})
}

func (e *ETCD) peerURL() string {
	return fmt.Sprintf("https://%s:2380", e.address)
}

func (e *ETCD) clientURL() string {
	return fmt.Sprintf("https://%s:2379", e.address)
}

func (e *ETCD) cluster(ctx context.Context, forceNew bool, options executor.InitialOptions) error {
	return executor.ETCD(executor.ETCDConfig{
		Name:                e.name,
		InitialOptions:      options,
		ForceNewCluster:     forceNew,
		ListenClientURLs:    fmt.Sprintf(e.clientURL() + ",https://127.0.0.1:2379"),
		ListenMetricsURLs:   fmt.Sprintf("http://127.0.0.1:2381"),
		ListenPeerURLs:      e.peerURL(),
		AdvertiseClientURLs: e.clientURL(),
		DataDir:             dataDir(e.config),
		ServerTrust: executor.ServerTrust{
			CertFile:       e.config.Runtime.ServerETCDCert,
			KeyFile:        e.config.Runtime.ServerETCDKey,
			ClientCertAuth: true,
			TrustedCAFile:  e.config.Runtime.ETCDServerCA,
		},
		PeerTrust: executor.PeerTrust{
			CertFile:       e.config.Runtime.PeerServerClientETCDCert,
			KeyFile:        e.config.Runtime.PeerServerClientETCDKey,
			ClientCertAuth: true,
			TrustedCAFile:  e.config.Runtime.ETCDPeerCA,
		},
		ElectionTimeout:   5000,
		HeartbeatInterval: 500,
	})
}

func (e *ETCD) removePeer(ctx context.Context, id, address string) error {
	members, err := e.client.MemberList(ctx)
	if err != nil {
		return err
	}

	for _, member := range members.Members {
		if member.Name != id {
			continue
		}
		for _, peerURL := range member.PeerURLs {
			u, err := url.Parse(peerURL)
			if err != nil {
				return err
			}
			if u.Hostname() == address {
				logrus.Infof("Removing name=%s id=%d address=%s from etcd", member.Name, member.ID, address)
				_, err := e.client.MemberRemove(ctx, member.ID)
				return err
			}
		}
	}

	return nil
}

func (e *ETCD) promoteMember(ctx context.Context, clientAccessInfo *clientaccess.Info) error {
	clientURLs, _, err := e.clientURLs(ctx, clientAccessInfo)
	if err != nil {
		return err
	}
	memberPromoted := true
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for range t.C {
		client, err := joinClient(ctx, e.runtime, clientURLs)
		// continue on errors to keep trying to promote member
		// grpc error are shown so no need to re log them
		if err != nil {
			continue
		}
		members, err := client.MemberList(ctx)
		if err != nil {
			continue
		}
		for _, member := range members.Members {
			// only one learner can exist in the cluster
			if !member.IsLearner {
				continue
			}
			if _, err := client.MemberPromote(ctx, member.ID); err != nil {
				memberPromoted = false
				break
			}
		}
		if memberPromoted {
			break
		}
	}
	return nil
}

func (e *ETCD) clientURLs(ctx context.Context, clientAccessInfo *clientaccess.Info) ([]string, Members, error) {
	var memberList Members
	resp, err := clientaccess.Get("/db/info", clientAccessInfo)
	if err != nil {
		return nil, memberList, err
	}

	if err := json.Unmarshal(resp, &memberList); err != nil {
		return nil, memberList, err
	}

	var clientURLs []string
	for _, member := range memberList.Members {
		// excluding learner member from the client list
		if member.IsLearner {
			continue
		}
		clientURLs = append(clientURLs, member.ClientURLs...)
	}
	return clientURLs, memberList, nil
}
