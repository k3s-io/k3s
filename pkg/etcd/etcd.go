package etcd

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/control/deps"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/etcd/s3"
	"github.com/k3s-io/k3s/pkg/etcd/snapshot"
	"github.com/k3s-io/k3s/pkg/server/auth"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/k3s-io/kine/pkg/client"
	endpoint2 "github.com/k3s-io/kine/pkg/endpoint"
	cp "github.com/otiai10/copy"
	"github.com/pkg/errors"
	certutil "github.com/rancher/dynamiclistener/cert"
	controllerv1 "github.com/rancher/wrangler/v3/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/v3/pkg/start"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.etcd.io/etcd/api/v3/v3rpc/rpctypes"
	clientv3 "go.etcd.io/etcd/client/v3"
	snapshotv3 "go.etcd.io/etcd/etcdutl/v3/snapshot"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	nodeHelper "k8s.io/component-helpers/node/util"
	nodeUtil "k8s.io/kubernetes/pkg/controller/util/node"
)

const (
	testTimeout          = time.Second * 30
	manageTickerTime     = time.Second * 15
	learnerMaxStallTime  = time.Minute * 5
	memberRemovalTimeout = time.Minute * 1

	// snapshotJitterMax defines the maximum time skew on cron-triggered snapshots. The actual jitter
	// will be a random Duration somewhere between 0 and snapshotJitterMax.
	snapshotJitterMax = time.Second * 5

	// defaultDialTimeout is intentionally short so that connections timeout within the testTimeout defined above
	defaultDialTimeout = 2 * time.Second
	// other defaults from k8s.io/apiserver/pkg/storage/storagebackend/factory/etcd3.go
	defaultKeepAliveTime    = 30 * time.Second
	defaultKeepAliveTimeout = 10 * time.Second
	heartbeatInterval       = 5 * time.Minute

	maxBackupRetention = 5

	etcdStatusType = v1.NodeConditionType("EtcdIsVoter")

	StatusUnjoined  MemberStatus = "unjoined"
	StatusUnhealthy MemberStatus = "unhealthy"
	StatusLearner   MemberStatus = "learner"
	StatusVoter     MemberStatus = "voter"
)

var (
	learnerProgressKey = version.Program + "/etcd/learnerProgress"
	// AddressKey will contain the value of api addresses list
	AddressKey = version.Program + "/apiaddresses"

	NodeNameAnnotation    = "etcd." + version.Program + ".cattle.io/node-name"
	NodeAddressAnnotation = "etcd." + version.Program + ".cattle.io/node-address"

	ErrAddressNotSet    = errors.New("apiserver addresses not yet set")
	ErrNotMember        = errNotMember()
	ErrMemberListFailed = errMemberListFailed()
)

type NodeControllerGetter func() controllerv1.NodeController

// explicit interface check
var _ managed.Driver = &ETCD{}

type MemberStatus string

type ETCD struct {
	client     *clientv3.Client
	config     *config.Control
	name       string
	address    string
	cron       *cron.Cron
	cancel     context.CancelFunc
	s3         *s3.Controller
	snapshotMu *sync.Mutex
}

type learnerProgress struct {
	ID               uint64      `json:"id,omitempty"`
	Name             string      `json:"name,omitempty"`
	RaftAppliedIndex uint64      `json:"raftAppliedIndex,omitempty"`
	LastProgress     metav1.Time `json:"lastProgress,omitempty"`
}

// Members contains a slice that holds all
// members of the cluster.
type Members struct {
	Members []*etcdserverpb.Member `json:"members"`
}

type membershipError struct {
	self    string
	members []string
}

func (e *membershipError) Error() string {
	return fmt.Sprintf("this server is a not a member of the etcd cluster. Found %v, expect: %s", e.members, e.self)
}

func (e *membershipError) Is(target error) bool {
	switch target {
	case ErrNotMember:
		return true
	}
	return false
}

func errNotMember() error { return &membershipError{} }

type memberListError struct {
	err error
}

func (e *memberListError) Error() string {
	return fmt.Sprintf("failed to get MemberList from server: %v", e.err)
}

func (e *memberListError) Is(target error) bool {
	switch target {
	case ErrMemberListFailed:
		return true
	}
	return false
}

func errMemberListFailed() error { return &memberListError{} }

// NewETCD creates a new value of type
// ETCD with initialized cron and snapshot mutex values.
func NewETCD() *ETCD {
	return &ETCD{
		cron:       cron.New(cron.WithLogger(cronLogger)),
		snapshotMu: &sync.Mutex{},
	}
}

// EndpointName returns the name of the endpoint.
func (e *ETCD) EndpointName() string {
	return "etcd"
}

// SetControlConfig passes the cluster config into the etcd datastore. This is necessary
// because the config may not yet be fully built at the time the Driver instance is registered.
func (e *ETCD) SetControlConfig(config *config.Control) error {
	if e.config != nil {
		return errors.New("control config already set")
	}

	e.config = config

	address, err := getAdvertiseAddress(e.config.PrivateIP)
	if err != nil {
		return err
	}
	e.address = address

	return e.setName(false)
}

// Test ensures that the local node is a voting member of the target cluster,
// and that the datastore is defragmented and not in maintenance mode due to alarms.
// If it is still a learner or not a part of the cluster, an error is raised.
// If it cannot be defragmented or has any alarms that cannot be disarmed, an error is raised.
func (e *ETCD) Test(ctx context.Context) error {
	if e.config == nil {
		return errors.New("control config not set")
	}
	if e.client == nil {
		return errors.New("etcd datastore is not started")
	}

	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	endpoints := getEndpoints(e.config)
	status, err := e.client.Status(ctx, endpoints[0])
	if err != nil {
		return err
	}

	if status.IsLearner {
		return errors.New("this server has not yet been promoted from learner to voting member")
	}

	if err := e.defragment(ctx); err != nil {
		return errors.Wrap(err, "failed to defragment etcd database")
	}

	if err := e.clearAlarms(ctx); err != nil {
		return errors.Wrap(err, "failed to report and disarm etcd alarms")
	}

	// refresh status to see if any errors remain after clearing alarms
	status, err = e.client.Status(ctx, endpoints[0])
	if err != nil {
		return err
	}

	if len(status.Errors) > 0 {
		return fmt.Errorf("etcd cluster errors: %s", strings.Join(status.Errors, ", "))
	}

	members, err := e.client.MemberList(ctx)
	if err != nil {
		return err
	}

	var memberNameUrls []string
	for _, member := range members.Members {
		for _, peerURL := range member.PeerURLs {
			if peerURL == e.peerURL() && e.name == member.Name {
				return nil
			}
		}
		if len(member.PeerURLs) > 0 {
			memberNameUrls = append(memberNameUrls, member.Name+"="+member.PeerURLs[0])
		}
	}
	return &membershipError{members: memberNameUrls, self: e.name + "=" + e.peerURL()}
}

// dbDir returns the path to dataDir/db/etcd
func dbDir(config *config.Control) string {
	return filepath.Join(config.DataDir, "db", "etcd")
}

// walDir returns the path to etcdDBDir/member/wal
func walDir(config *config.Control) string {
	return filepath.Join(dbDir(config), "member", "wal")
}

func sqliteFile(config *config.Control) string {
	return filepath.Join(config.DataDir, "db", "state.db")
}

// nameFile returns the path to etcdDBDir/name.
func nameFile(config *config.Control) string {
	return filepath.Join(dbDir(config), "name")
}

// clearReset removes the reset file
func (e *ETCD) clearReset() error {
	if err := os.Remove(e.ResetFile()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsReset checks to see if the reset file exists, indicating that a cluster-reset has been completed successfully.
func (e *ETCD) IsReset() (bool, error) {
	if e.config == nil {
		return false, errors.New("control config not set")
	}

	if _, err := os.Stat(e.ResetFile()); err != nil {
		if !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	return true, nil
}

// ResetFile returns the path to etcdDBDir/reset-flag.
func (e *ETCD) ResetFile() string {
	if e.config == nil {
		panic("control config not set")
	}
	return filepath.Join(e.config.DataDir, "db", "reset-flag")
}

// IsInitialized checks to see if a WAL directory exists. If so, we assume that etcd
// has already been brought up at least once.
func (e *ETCD) IsInitialized() (bool, error) {
	if e.config == nil {
		return false, errors.New("control config not set")
	}

	dir := walDir(e.config)
	if s, err := os.Stat(dir); err == nil && s.IsDir() {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	} else {
		return false, errors.Wrap(err, "invalid state for wal directory "+dir)
	}
}

// Reset resets an etcd node to a single node cluster.
func (e *ETCD) Reset(ctx context.Context, rebootstrap func() error) error {
	// Wait for etcd to come up as a new single-node cluster, then exit
	go func() {
		<-e.config.Runtime.ContainerRuntimeReady
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for range t.C {
			if err := e.Test(ctx); err == nil {
				// reset the apiaddresses to nil since we are doing a restoration
				if _, err := e.client.Put(ctx, AddressKey, ""); err != nil {
					logrus.Warnf("failed to reset api addresses key in etcd: %v", err)
					continue
				}

				members, err := e.client.MemberList(ctx)
				if err != nil {
					continue
				}

				if rebootstrap != nil {
					// storageBootstrap() - runtime structure has been written with correct certificate data
					if err := rebootstrap(); err != nil {
						logrus.Fatal(err)
					}
				}

				// call functions to rewrite them from daemons/control/server.go (prepare())
				if err := deps.GenServerDeps(e.config); err != nil {
					logrus.Fatal(err)
				}

				if len(members.Members) == 1 && members.Members[0].Name == e.name {
					// Cancel the etcd server context and allow it time to shutdown cleanly.
					// Ideally we would use a waitgroup and properly sequence shutdown of the various components.
					e.cancel()
					time.Sleep(time.Second * 5)
					logrus.Infof("Managed etcd cluster membership has been reset, restart without --cluster-reset flag now. Backup and delete ${datadir}/server/db on each peer etcd server and rejoin the nodes")
					os.Exit(0)
				}
			} else {
				// make sure that peer ips are updated to the node ip in case the test fails
				members, err := e.client.MemberList(ctx)
				if err != nil {
					logrus.Warnf("failed to list etcd members: %v", err)
					continue
				}
				if len(members.Members) > 1 {
					logrus.Warnf("failed to update peer url: etcd still has more than one member")
					continue
				}
				if _, err := e.client.MemberUpdate(ctx, members.Members[0].ID, []string{e.peerURL()}); err != nil {
					logrus.Warnf("failed to update peer url: %v", err)
					continue
				}
			}
		}
	}()

	if err := e.startClient(ctx); err != nil {
		return err
	}

	// If asked to restore from a snapshot, do so
	if e.config.ClusterResetRestorePath != "" {
		if e.config.EtcdS3 != nil {
			logrus.Infof("Retrieving etcd snapshot %s from S3", e.config.ClusterResetRestorePath)
			s3client, err := e.getS3Client(ctx)
			if err != nil {
				if errors.Is(err, s3.ErrNoConfigSecret) {
					return errors.New("cannot use S3 config secret when restoring snapshot; configuration must be set in CLI or config file")
				} else {
					return errors.Wrap(err, "failed to initialize S3 client")
				}
			}
			dir, err := snapshotDir(e.config, true)
			if err != nil {
				return errors.Wrap(err, "failed to get the snapshot dir")
			}
			path, err := s3client.Download(ctx, e.config.ClusterResetRestorePath, dir)
			if err != nil {
				return errors.Wrap(err, "failed to download snapshot from S3")
			}
			e.config.ClusterResetRestorePath = path
			logrus.Infof("S3 download complete for %s", e.config.ClusterResetRestorePath)
		}

		info, err := os.Stat(e.config.ClusterResetRestorePath)
		if os.IsNotExist(err) {
			return fmt.Errorf("etcd: snapshot path does not exist: %s", e.config.ClusterResetRestorePath)
		}
		if info.IsDir() {
			return fmt.Errorf("etcd: snapshot path must be a file, not a directory: %s", e.config.ClusterResetRestorePath)
		}
		if err := e.Restore(ctx); err != nil {
			return err
		}
	}

	if err := e.setName(true); err != nil {
		return err
	}
	// touch a file to avoid multiple resets
	if err := os.WriteFile(e.ResetFile(), []byte{}, 0600); err != nil {
		return err
	}

	return e.newCluster(ctx, true)
}

// Start starts the datastore
func (e *ETCD) Start(ctx context.Context, clientAccessInfo *clientaccess.Info) error {
	isInitialized, err := e.IsInitialized()
	if err != nil {
		return errors.Wrapf(err, "failed to check for initialized etcd datastore")
	}

	if err := e.startClient(ctx); err != nil {
		return err
	}

	if !e.config.EtcdDisableSnapshots {
		e.setSnapshotFunction(ctx)
		e.cron.Start()
	}

	go e.manageLearners(ctx)
	go e.getS3Client(ctx)

	if isInitialized {
		// check etcd dir permission
		etcdDir := dbDir(e.config)
		info, err := os.Stat(etcdDir)
		if err != nil {
			return err
		}
		if info.Mode() != 0700 {
			if err := os.Chmod(etcdDir, 0700); err != nil {
				return err
			}
		}
		opt, err := executor.CurrentETCDOptions()
		if err != nil {
			return err
		}
		logrus.Infof("Starting etcd for existing cluster member")
		return e.cluster(ctx, false, opt)
	}

	if clientAccessInfo == nil {
		return e.newCluster(ctx, false)
	}

	go func() {
		for {
			select {
			case <-time.After(30 * time.Second):
				logrus.Infof("Waiting for container runtime to become ready before joining etcd cluster")
			case <-e.config.Runtime.ContainerRuntimeReady:
				if err := wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
					if err := e.join(ctx, clientAccessInfo); err != nil {
						// Retry the join if waiting for another member to be promoted, or waiting for peers to connect after promotion
						if errors.Is(err, rpctypes.ErrTooManyLearners) || errors.Is(err, rpctypes.ErrUnhealthy) {
							logrus.Infof("Waiting for other members to finish joining etcd cluster: %v", err)
							return false, nil
						}
						// Retry the join if waiting to retrieve the member list from the server
						if errors.Is(err, ErrMemberListFailed) {
							logrus.Infof("Waiting to retrieve etcd cluster member list: %v", err)
							return false, nil
						}
						return false, err
					}
					return true, nil
				}); err != nil {
					logrus.Fatalf("etcd cluster join failed: %v", err)
				}
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

// startClient sets up the config's datastore endpoints, and starts an etcd client connected to the server endpoint.
// The client is destroyed when the context is closed.
func (e *ETCD) startClient(ctx context.Context) error {
	if e.client != nil {
		return errors.New("etcd datastore already started")
	}

	endpoints := getEndpoints(e.config)
	e.config.Datastore.Endpoint = endpoints[0]
	e.config.Datastore.BackendTLSConfig.CAFile = e.config.Runtime.ETCDServerCA
	e.config.Datastore.BackendTLSConfig.CertFile = e.config.Runtime.ClientETCDCert
	e.config.Datastore.BackendTLSConfig.KeyFile = e.config.Runtime.ClientETCDKey

	client, err := getClient(ctx, e.config, endpoints...)
	if err != nil {
		return err
	}
	e.client = client

	go func() {
		<-ctx.Done()
		client := e.client
		e.client = nil
		client.Close()
	}()

	return nil
}

// join attempts to add a member to an existing cluster
func (e *ETCD) join(ctx context.Context, clientAccessInfo *clientaccess.Info) error {
	clientCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var (
		cluster []string
		add     = true
	)

	clientURLs, memberList, err := ClientURLs(clientCtx, clientAccessInfo, e.config.PrivateIP)
	if err != nil {
		return err
	}

	client, err := getClient(clientCtx, e.config, clientURLs...)
	if err != nil {
		return err
	}
	defer client.Close()

	for _, member := range memberList.Members {
		for _, peer := range member.PeerURLs {
			u, err := url.Parse(peer)
			if err != nil {
				return err
			}
			// An uninitialized joining member won't have a name; if it has our
			// address it must be us.
			if member.Name == "" && u.Hostname() == e.address {
				member.Name = e.name
			}

			// If we're already in the cluster, don't try to add ourselves.
			if member.Name == e.name && u.Hostname() == e.address {
				add = false
			}

			if len(member.PeerURLs) > 0 {
				cluster = append(cluster, fmt.Sprintf("%s=%s", member.Name, member.PeerURLs[0]))
			}
		}

		// Try to get the node name from the member name
		memberNodeName := member.Name
		if lastHyphen := strings.LastIndex(member.Name, "-"); lastHyphen > 1 {
			memberNodeName = member.Name[:lastHyphen]
		}

		// Make sure there's not already a member in the cluster with a duplicate node name
		if member.Name != e.name && memberNodeName == e.config.ServerNodeName {
			// make sure to remove the name file if a duplicate node name is used, so that we
			// generate a new member name when our node name is fixed.
			nameFile := nameFile(e.config)
			if err := os.Remove(nameFile); err != nil {
				logrus.Errorf("Failed to remove etcd name file %s: %v", nameFile, err)
			}
			return errors.New("duplicate node name found, please use a unique name for this node")
		}
	}

	if add {
		logrus.Infof("Adding member %s=%s to etcd cluster %v", e.name, e.peerURL(), cluster)
		if _, err = client.MemberAddAsLearner(clientCtx, []string{e.peerURL()}); err != nil {
			return err
		}
		cluster = append(cluster, fmt.Sprintf("%s=%s", e.name, e.peerURL()))
	}

	logrus.Infof("Starting etcd to join cluster with members %v", cluster)
	return e.cluster(ctx, false, executor.InitialOptions{
		Cluster: strings.Join(cluster, ","),
		State:   "existing",
	})
}

// Register adds db info routes for the http request handler, and registers cluster controller callbacks
func (e *ETCD) Register(handler http.Handler) (http.Handler, error) {
	e.config.Runtime.ClusterControllerStarts["etcd-node-metadata"] = func(ctx context.Context) {
		registerMetadataHandlers(ctx, e)
	}

	// The apiserver endpoint controller needs to run on a node with a local apiserver,
	// in order to successfully seed etcd with the endpoint list. The member removal controller
	// also needs to run on a non-etcd node as to avoid disruption if running on the node that
	// is being removed from the cluster.
	if !e.config.DisableAPIServer {
		e.config.Runtime.LeaderElectedClusterControllerStarts[version.Program+"-etcd"] = func(ctx context.Context) {
			registerEndpointsHandlers(ctx, e)
			registerMemberHandlers(ctx, e)
			registerSnapshotHandlers(ctx, e)

			// Re-run informer factory startup after core and leader-elected controllers have started.
			// Additional caches may need to start for the newly added OnChange/OnRemove callbacks.
			if err := start.All(ctx, 5, e.config.Runtime.K3s, e.config.Runtime.Core); err != nil {
				panic(errors.Wrap(err, "failed to start wrangler controllers"))
			}
		}
	}

	// Tombstone file checking is unnecessary if we're not running etcd.
	if !e.config.DisableETCD {
		tombstoneFile := filepath.Join(dbDir(e.config), "tombstone")
		if _, err := os.Stat(tombstoneFile); err == nil {
			logrus.Infof("tombstone file has been detected, removing data dir to rejoin the cluster")
			if _, err := backupDirWithRetention(dbDir(e.config), maxBackupRetention); err != nil {
				return nil, err
			}
		}

		if err := e.setName(false); err != nil {
			return nil, err
		}
	}

	return e.handler(handler), nil
}

// setName sets a unique name for this cluster member. The first time this is called,
// or if force is set to true, a new name will be generated and written to disk. The persistent
// name is used on subsequent calls.
func (e *ETCD) setName(force bool) error {
	fileName := nameFile(e.config)
	data, err := os.ReadFile(fileName)
	if os.IsNotExist(err) || force {
		if e.config.ServerNodeName == "" {
			return errors.New("server node name not set")
		}
		e.name = e.config.ServerNodeName + "-" + uuid.New().String()[:8]
		if err := os.MkdirAll(filepath.Dir(fileName), 0700); err != nil {
			return err
		}
		return os.WriteFile(fileName, []byte(e.name), 0600)
	} else if err != nil {
		return err
	}
	e.name = string(data)
	return nil
}

// handler wraps the handler with routes for database info
func (e *ETCD) handler(next http.Handler) http.Handler {
	r := mux.NewRouter().SkipClean(true)
	r.NotFoundHandler = next

	ir := r.Path("/db/info").Subrouter()
	ir.Use(auth.IsLocalOrHasRole(e.config, version.Program+":server"))
	ir.Handle("", e.infoHandler())

	sr := r.Path("/db/snapshot").Subrouter()
	sr.Use(auth.HasRole(e.config, version.Program+":server"))
	sr.Handle("", e.snapshotHandler())

	return r
}

// infoHandler returns etcd cluster information. This is used by new members when joining the cluster.
// If we can't retrieve an actual MemberList from etcd, we return a canned response with only the local node listed.
func (e *ETCD) infoHandler() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet {
			util.SendError(fmt.Errorf("method not allowed"), rw, req, http.StatusMethodNotAllowed)
			return
		}

		ctx, cancel := context.WithTimeout(req.Context(), 2*time.Second)
		defer cancel()

		members, err := e.client.MemberList(ctx)
		if err != nil {
			util.SendError(errors.Wrap(err, "failed to get etcd MemberList"), rw, req, http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")
		json.NewEncoder(rw).Encode(&Members{
			Members: members.Members,
		})
	})
}

// getClient returns an etcd client connected to the specified endpoints.
// If no endpoints are provided, endpoints are retrieved from the provided runtime config.
// If the runtime config does not list any endpoints, the default endpoint is used.
// The returned client should be closed when no longer needed, in order to avoid leaking GRPC
// client goroutines.
func getClient(ctx context.Context, control *config.Control, endpoints ...string) (*clientv3.Client, error) {
	cfg, err := getClientConfig(ctx, control, endpoints...)
	if err != nil {
		return nil, err
	}

	return clientv3.New(*cfg)
}

// getClientConfig generates an etcd client config connected to the specified endpoints.
// If no endpoints are provided, getEndpoints is called to provide defaults.
func getClientConfig(ctx context.Context, control *config.Control, endpoints ...string) (*clientv3.Config, error) {
	runtime := control.Runtime
	if len(endpoints) == 0 {
		endpoints = getEndpoints(control)
	}

	config := &clientv3.Config{
		Endpoints:            endpoints,
		Context:              ctx,
		DialTimeout:          defaultDialTimeout,
		DialKeepAliveTime:    defaultKeepAliveTime,
		DialKeepAliveTimeout: defaultKeepAliveTimeout,
		PermitWithoutStream:  true,
	}

	var err error
	if strings.HasPrefix(endpoints[0], "https://") {
		config.TLS, err = toTLSConfig(runtime)
	}
	return config, err
}

// getEndpoints returns the endpoints from the runtime config if set, otherwise the default endpoint.
func getEndpoints(control *config.Control) []string {
	runtime := control.Runtime
	if len(runtime.EtcdConfig.Endpoints) > 0 {
		return runtime.EtcdConfig.Endpoints
	}
	return []string{fmt.Sprintf("https://%s:2379", control.Loopback(true))}
}

// toTLSConfig converts the ControlRuntime configuration to TLS configuration suitable
// for use by etcd.
func toTLSConfig(runtime *config.ControlRuntime) (*tls.Config, error) {
	if runtime.ClientETCDCert == "" || runtime.ClientETCDKey == "" || runtime.ETCDServerCA == "" {
		return nil, util.ErrCoreNotReady
	}

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

// getAdvertiseAddress returns the IP address best suited for advertising to clients
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

// newCluster returns options to set up etcd for a new cluster
func (e *ETCD) newCluster(ctx context.Context, reset bool) error {
	logrus.Infof("Starting etcd for new cluster, cluster-reset=%v", reset)
	err := e.cluster(ctx, reset, executor.InitialOptions{
		AdvertisePeerURL: e.peerURL(),
		Cluster:          fmt.Sprintf("%s=%s", e.name, e.peerURL()),
		State:            "new",
	})
	if err != nil {
		return err
	}
	if !reset {
		if err := e.migrateFromSQLite(ctx); err != nil {
			return fmt.Errorf("failed to migrate content from sqlite to etcd: %w", err)
		}
	}
	return nil
}

func (e *ETCD) migrateFromSQLite(ctx context.Context) error {
	_, err := os.Stat(sqliteFile(e.config))
	if os.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}

	logrus.Infof("Migrating content from sqlite to etcd")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	_, err = endpoint2.Listen(ctx, endpoint2.Config{
		Endpoint: "sqlite://",
	})
	if err != nil {
		return err
	}

	sqliteClient, err := client.New(endpoint2.ETCDConfig{
		Endpoints: []string{"unix://kine.sock"},
	})
	if err != nil {
		return err
	}
	defer sqliteClient.Close()

	etcdClient, err := getClient(ctx, e.config)
	if err != nil {
		return err
	}
	defer etcdClient.Close()

	values, err := sqliteClient.List(ctx, "/registry/", 0)
	if err != nil {
		return err
	}

	for _, value := range values {
		logrus.Infof("Migrating etcd key %s", value.Key)
		_, err := etcdClient.Put(ctx, string(value.Key), string(value.Data))
		if err != nil {
			return err
		}
	}

	return os.Rename(sqliteFile(e.config), sqliteFile(e.config)+".migrated")
}

// peerURL returns the external peer access address for the local node.
func (e *ETCD) peerURL() string {
	return fmt.Sprintf("https://%s", net.JoinHostPort(e.address, "2380"))
}

// listenClientURLs returns a list of URLs to bind to for peer connections.
// During cluster reset/restore, we only listen on loopback to avoid having peers
// connect mid-process.
func (e *ETCD) listenPeerURLs(reset bool) string {
	peerURLs := fmt.Sprintf("https://%s:2380", e.config.Loopback(true))
	if !reset {
		peerURLs += "," + e.peerURL()
	}
	return peerURLs
}

// clientURL returns the external client access address for the local node.
func (e *ETCD) clientURL() string {
	return fmt.Sprintf("https://%s", net.JoinHostPort(e.address, "2379"))
}

// advertiseClientURLs returns the advertised addresses for the local node.
// During cluster reset/restore we only listen on loopback to avoid having apiservers
// on other nodes connect mid-process.
func (e *ETCD) advertiseClientURLs(reset bool) string {
	if reset {
		return fmt.Sprintf("https://%s:2379", e.config.Loopback(true))
	}
	return e.clientURL()
}

// listenClientURLs returns a list of URLs to bind to for client connections.
// During cluster reset/restore, we only listen on loopback to avoid having apiservers
// on other nodes connect mid-process.
func (e *ETCD) listenClientURLs(reset bool) string {
	clientURLs := fmt.Sprintf("https://%s:2379", e.config.Loopback(true))
	if !reset {
		clientURLs += "," + e.clientURL()
	}
	return clientURLs
}

// listenMetricsURLs returns a list of URLs to bind to for metrics connections.
func (e *ETCD) listenMetricsURLs(reset bool) string {
	metricsURLs := fmt.Sprintf("http://%s:2381", e.config.Loopback(true))
	if !reset && e.config.EtcdExposeMetrics {
		metricsURLs += "," + fmt.Sprintf("http://%s", net.JoinHostPort(e.address, "2381"))
	}
	return metricsURLs
}

// listenClientHTTPURLs returns a list of URLs to bind to for http client connections.
// This should no longer be used, but we must set it in order to free the listen URLs
// for dedicated use by GRPC.
// Ref: https://github.com/etcd-io/etcd/issues/15402
func (e *ETCD) listenClientHTTPURLs() string {
	return fmt.Sprintf("https://%s:2382", e.config.Loopback(true))
}

// cluster calls the executor to start etcd running with the provided configuration.
func (e *ETCD) cluster(ctx context.Context, reset bool, options executor.InitialOptions) error {
	ctx, e.cancel = context.WithCancel(ctx)
	return executor.ETCD(ctx, executor.ETCDConfig{
		Name:                e.name,
		InitialOptions:      options,
		ForceNewCluster:     reset,
		ListenClientURLs:    e.listenClientURLs(reset),
		ListenMetricsURLs:   e.listenMetricsURLs(reset),
		ListenPeerURLs:      e.listenPeerURLs(reset),
		AdvertiseClientURLs: e.advertiseClientURLs(reset),
		DataDir:             dbDir(e.config),
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
		SnapshotCount:        10000,
		ElectionTimeout:      5000,
		HeartbeatInterval:    500,
		Logger:               "zap",
		LogOutputs:           []string{"stderr"},
		ListenClientHTTPURLs: e.listenClientHTTPURLs(),

		ExperimentalInitialCorruptCheck:         true,
		ExperimentalWatchProgressNotifyInterval: e.config.Datastore.NotifyInterval,
	}, e.config.ExtraEtcdArgs)
}

func (e *ETCD) StartEmbeddedTemporary(ctx context.Context) error {
	etcdDataDir := dbDir(e.config)
	tmpDataDir := etcdDataDir + "-tmp"
	os.RemoveAll(tmpDataDir)

	go func() {
		<-ctx.Done()
		if err := os.RemoveAll(tmpDataDir); err != nil {
			logrus.Warnf("Failed to remove etcd temp dir: %v", err)
		}
	}()

	if e.client != nil {
		return errors.New("etcd datastore already started")
	}

	client, err := getClient(ctx, e.config)
	if err != nil {
		return err
	}
	e.client = client

	go func() {
		<-ctx.Done()
		client := e.client
		e.client = nil
		client.Close()
	}()

	if err := cp.Copy(etcdDataDir, tmpDataDir, cp.Options{PreserveOwner: true}); err != nil {
		return err
	}

	endpoints := getEndpoints(e.config)
	clientURL := endpoints[0]
	// peer URL is usually 1 more than client
	peerURL, err := addPort(endpoints[0], 1)
	if err != nil {
		return err
	}
	// client http URL is usually 3 more than client, after peer and metrics
	clientHTTPURL, err := addPort(endpoints[0], 3)
	if err != nil {
		return err
	}

	embedded := executor.Embedded{}
	ctx, e.cancel = context.WithCancel(ctx)
	return embedded.ETCD(ctx, executor.ETCDConfig{
		InitialOptions:       executor.InitialOptions{AdvertisePeerURL: peerURL},
		DataDir:              tmpDataDir,
		ForceNewCluster:      true,
		AdvertiseClientURLs:  clientURL,
		ListenClientURLs:     clientURL,
		ListenClientHTTPURLs: clientHTTPURL,
		ListenPeerURLs:       peerURL,
		Logger:               "zap",
		HeartbeatInterval:    500,
		ElectionTimeout:      5000,
		SnapshotCount:        10000,
		Name:                 e.name,
		LogOutputs:           []string{"stderr"},

		ExperimentalInitialCorruptCheck:         true,
		ExperimentalWatchProgressNotifyInterval: e.config.Datastore.NotifyInterval,
	}, append(e.config.ExtraEtcdArgs, "--max-snapshots=0", "--max-wals=0"))
}

func addPort(address string, offset int) (string, error) {
	u, err := url.Parse(address)
	if err != nil {
		return "", err
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		return "", err
	}
	port += offset
	return fmt.Sprintf("%s://%s:%d", u.Scheme, u.Hostname(), port), nil
}

// RemovePeer removes a peer from the cluster. The peer name and IP address must both match.
func (e *ETCD) RemovePeer(ctx context.Context, name, address string, allowSelfRemoval bool) error {
	ctx, cancel := context.WithTimeout(ctx, memberRemovalTimeout)
	defer cancel()
	members, err := e.client.MemberList(ctx)
	if err != nil {
		return err
	}

	for _, member := range members.Members {
		if member.Name != name {
			continue
		}
		for _, peerURL := range member.PeerURLs {
			u, err := url.Parse(peerURL)
			if err != nil {
				return err
			}
			if u.Hostname() == address {
				if e.address == address && !allowSelfRemoval {
					return errors.New("not removing self from etcd cluster")
				}
				logrus.Infof("Removing name=%s id=%d address=%s from etcd", member.Name, member.ID, address)
				_, err := e.client.MemberRemove(ctx, member.ID)
				if errors.Is(err, rpctypes.ErrGRPCMemberNotFound) {
					return nil
				}
				return err
			}
		}
	}

	return nil
}

// manageLearners monitors the etcd cluster to ensure that learners are making progress towards
// being promoted to full voting member. The checks only run on the cluster member that is
// the etcd leader.
func (e *ETCD) manageLearners(ctx context.Context) {
	<-e.config.Runtime.ContainerRuntimeReady
	t := time.NewTicker(manageTickerTime)
	defer t.Stop()

	for range t.C {
		ctx, cancel := context.WithTimeout(ctx, manageTickerTime)
		defer cancel()

		// Check to see if the local node is the leader. Only the leader should do learner management.
		if e.client == nil {
			logrus.Debug("Etcd client was nil")
			continue
		}

		endpoints := getEndpoints(e.config)
		if status, err := e.client.Status(ctx, endpoints[0]); err != nil {
			logrus.Errorf("Failed to check local etcd status for learner management: %v", err)
			continue
		} else if status.Header.MemberId != status.Leader {
			continue
		}

		progress, err := e.getLearnerProgress(ctx)
		if err != nil {
			logrus.Errorf("Failed to get recorded learner progress from etcd: %v", err)
			continue
		}

		members, err := e.client.MemberList(ctx)
		if err != nil {
			logrus.Errorf("Failed to get etcd members for learner management: %v", err)
			continue
		}

		client, err := util.GetClientSet(e.config.Runtime.KubeConfigSupervisor)
		if err != nil {
			logrus.Errorf("Failed to get k8s client for patch node status condition: %v", err)
			continue
		}

		nodes, err := e.getETCDNodes()
		if err != nil {
			logrus.Warnf("Failed to list nodes with etcd role: %v", err)
		}

		// a map to track if a node is a member of the etcd cluster or not
		nodeIsMember := make(map[string]bool)
		nodesMap := make(map[string]*v1.Node)
		for _, node := range nodes {
			nodeIsMember[node.Name] = false
			nodesMap[node.Name] = node
		}

		for _, member := range members.Members {
			status := StatusVoter
			message := ""

			if member.IsLearner {
				status = StatusLearner
				if err := e.trackLearnerProgress(ctx, progress, member); err != nil {
					logrus.Errorf("Failed to track learner progress towards promotion: %v", err)
				}
			}

			var node *v1.Node
			for _, n := range nodes {
				if member.Name == n.Annotations[NodeNameAnnotation] {
					node = n
					nodeIsMember[n.Name] = true
					break
				}
			}
			if node == nil {
				continue
			}

			// verify if the member is healthy and set the status
			if _, err := e.getETCDStatus(ctx, member.ClientURLs[0]); err != nil {
				message = err.Error()
				status = StatusUnhealthy
			}

			if err := e.setEtcdStatusCondition(node, client, member.Name, status, message); err != nil {
				logrus.Errorf("Unable to set etcd status condition %s: %v", member.Name, err)
			}
		}

		for nodeName, node := range nodesMap {
			if !nodeIsMember[nodeName] {
				if err := e.setEtcdStatusCondition(node, client, nodeName, StatusUnjoined, ""); err != nil {
					logrus.Errorf("Unable to set etcd status condition for a node that is not a cluster member %s: %v", nodeName, err)
				}
			}
		}
	}
}

func (e *ETCD) getETCDNodes() ([]*v1.Node, error) {
	if e.config.Runtime.Core == nil {
		return nil, util.ErrCoreNotReady
	}

	nodes := e.config.Runtime.Core.Core().V1().Node()
	etcdSelector := labels.Set{util.ETCDRoleLabelKey: "true"}

	return nodes.Cache().List(etcdSelector.AsSelector())
}

// trackLearnerProcess attempts to promote a learner. If it cannot be promoted, progress through the raft index is tracked.
// If the learner does not make any progress in a reasonable amount of time, it is evicted from the cluster.
func (e *ETCD) trackLearnerProgress(ctx context.Context, progress *learnerProgress, member *etcdserverpb.Member) error {
	// Try to promote it. If it can be promoted, no further tracking is necessary
	if _, err := e.client.MemberPromote(ctx, member.ID); err != nil {
		logrus.Debugf("Unable to promote learner %s: %v", member.Name, err)
	} else {
		logrus.Infof("Promoted learner %s", member.Name)
		return nil
	}

	now := time.Now()

	// If this is the first time we've tracked this member's progress, reset stats
	if progress.Name != member.Name || progress.ID != member.ID {
		progress.ID = member.ID
		progress.Name = member.Name
		progress.RaftAppliedIndex = 0
		progress.LastProgress.Time = now
	}

	// Update progress by retrieving status from the member's first reachable client URL
	for _, ep := range member.ClientURLs {
		status, err := e.getETCDStatus(ctx, ep)
		if err != nil {
			logrus.Debugf("Failed to get etcd status from learner %s at %s: %v", member.Name, ep, err)
			continue
		}

		if progress.RaftAppliedIndex < status.RaftAppliedIndex {
			logrus.Debugf("Learner %s has progressed from RaftAppliedIndex %d to %d", progress.Name, progress.RaftAppliedIndex, status.RaftAppliedIndex)
			progress.RaftAppliedIndex = status.RaftAppliedIndex
			progress.LastProgress.Time = now
		}
		break
	}

	// Warn if the learner hasn't made any progress
	if !progress.LastProgress.Time.Equal(now) {
		logrus.Warnf("Learner %s stalled at RaftAppliedIndex=%d for %s", progress.Name, progress.RaftAppliedIndex, now.Sub(progress.LastProgress.Time).String())
	}

	// See if it's time to evict yet
	if now.Sub(progress.LastProgress.Time) > learnerMaxStallTime {
		if _, err := e.client.MemberRemove(ctx, member.ID); err != nil {
			return err
		}
		logrus.Warnf("Removed learner %s from etcd cluster", member.Name)
		return nil
	}

	return e.setLearnerProgress(ctx, progress)
}

func (e *ETCD) getETCDStatus(ctx context.Context, url string) (*clientv3.StatusResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultDialTimeout)
	defer cancel()
	resp, err := e.client.Status(ctx, url)
	if err != nil {
		return resp, errors.Wrap(err, "failed to check etcd member status")
	}
	if len(resp.Errors) != 0 {
		return resp, errors.New("etcd member has status errors: " + strings.Join(resp.Errors, ","))
	}
	return resp, nil
}

func (e *ETCD) setEtcdStatusCondition(node *v1.Node, client kubernetes.Interface, memberName string, memberStatus MemberStatus, message string) error {
	var newCondition v1.NodeCondition
	switch memberStatus {
	case StatusLearner:
		newCondition = v1.NodeCondition{
			Type:    etcdStatusType,
			Status:  "False",
			Reason:  "MemberIsLearner",
			Message: "Node has not been promoted to voting member of the etcd cluster",
		}
	case StatusVoter:
		newCondition = v1.NodeCondition{
			Type:    etcdStatusType,
			Status:  "True",
			Reason:  "MemberNotLearner",
			Message: "Node is a voting member of the etcd cluster",
		}
	case StatusUnhealthy:
		newCondition = v1.NodeCondition{
			Type:    etcdStatusType,
			Status:  "False",
			Reason:  "Unhealthy",
			Message: "Node is unhealthy",
		}
	case StatusUnjoined:
		newCondition = v1.NodeCondition{
			Type:    etcdStatusType,
			Status:  "False",
			Reason:  "NotAMember",
			Message: "Node is not a member of the etcd cluster",
		}
	default:
		logrus.Warnf("Unknown etcd member status %s", memberStatus)
		return nil
	}

	if message != "" {
		newCondition.Message = message
	}

	if find, condition := nodeUtil.GetNodeCondition(&node.Status, etcdStatusType); find >= 0 {

		// if the condition is not changing, we only want to update the last heartbeat time
		if condition.Status == newCondition.Status && condition.Reason == newCondition.Reason && condition.Message == newCondition.Message {
			logrus.Debugf("Node %s is not changing etcd status condition", memberName)

			// If the condition status is not changing, we only want to update the last heartbeat time if the
			// LastHeartbeatTime is older than the heartbeatTimeout.
			if metav1.Now().Sub(condition.LastHeartbeatTime.Time) < heartbeatInterval {
				return nil
			}

			condition.LastHeartbeatTime = metav1.Now()
			return nodeHelper.SetNodeCondition(client, types.NodeName(node.Name), *condition)
		}

		logrus.Debugf("Node %s is changing etcd status condition", memberName)
		condition = &newCondition
		condition.LastHeartbeatTime = metav1.Now()
		condition.LastTransitionTime = metav1.Now()
		return nodeHelper.SetNodeCondition(client, types.NodeName(node.Name), *condition)
	}

	logrus.Infof("Adding node %s etcd status condition", memberName)
	newCondition.LastHeartbeatTime = metav1.Now()
	newCondition.LastTransitionTime = metav1.Now()
	return nodeHelper.SetNodeCondition(client, types.NodeName(node.Name), newCondition)
}

// getLearnerProgress returns the stored learnerProgress struct as retrieved from etcd
func (e *ETCD) getLearnerProgress(ctx context.Context) (*learnerProgress, error) {
	progress := &learnerProgress{}

	value, err := e.client.Get(ctx, learnerProgressKey)
	if err != nil {
		return nil, err
	}

	if value.Count < 1 {
		return progress, nil
	}

	if err := json.NewDecoder(bytes.NewBuffer(value.Kvs[0].Value)).Decode(progress); err != nil {
		return nil, err
	}
	return progress, nil
}

// setLearnerProgress stores the learnerProgress struct to etcd
func (e *ETCD) setLearnerProgress(ctx context.Context, status *learnerProgress) error {
	w := &bytes.Buffer{}

	if err := json.NewEncoder(w).Encode(status); err != nil {
		return err
	}

	_, err := e.client.Put(ctx, learnerProgressKey, w.String())
	return err
}

// clearAlarms checks for any alarms on the local etcd member. If found, they are
// reported and the alarm state is cleared.
func (e *ETCD) clearAlarms(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	if e.client == nil {
		return errors.New("etcd client was nil")
	}

	alarmList, err := e.client.AlarmList(ctx)
	if err != nil {
		return fmt.Errorf("etcd alarm list failed: %v", err)
	}

	for _, alarm := range alarmList.Alarms {
		logrus.Warnf("Alarm on etcd member %d: %s", alarm.MemberID, alarm.Alarm)
	}

	if len(alarmList.Alarms) > 0 {
		if _, err := e.client.AlarmDisarm(ctx, &clientv3.AlarmMember{}); err != nil {
			return fmt.Errorf("etcd alarm disarm failed: %v", err)
		}
		logrus.Infof("Alarms disarmed on etcd server")
	}
	return nil
}

func (e *ETCD) defragment(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	if e.client == nil {
		return errors.New("etcd client was nil")
	}

	logrus.Infof("Defragmenting etcd database")
	endpoints := getEndpoints(e.config)
	_, err := e.client.Defragment(ctx, endpoints[0])
	return err
}

// clientURLs returns a list of all non-learner etcd cluster member client access URLs.
// The list is retrieved from the remote server that is being joined.
func ClientURLs(ctx context.Context, clientAccessInfo *clientaccess.Info, selfIP string) ([]string, Members, error) {
	var memberList Members

	// find the address advertised for our own client URL, so that we don't connect to ourselves
	ip, err := getAdvertiseAddress(selfIP)
	if err != nil {
		return nil, memberList, err
	}

	// find the client URL of the server we're joining, so we can prioritize it
	joinURL, err := url.Parse(clientAccessInfo.BaseURL)
	if err != nil {
		return nil, memberList, err
	}

	// get the full list from the server we're joining
	resp, err := clientAccessInfo.Get("/db/info")
	if err != nil {
		return nil, memberList, &memberListError{err: err}
	}
	if err := json.Unmarshal(resp, &memberList); err != nil {
		return nil, memberList, err
	}

	// Build a list of client URLs. Learners and the current node are excluded;
	// the server we're joining is listed first if found.
	var clientURLs []string
	for _, member := range memberList.Members {
		var isSelf, isPreferred bool
		for _, clientURL := range member.ClientURLs {
			if u, err := url.Parse(clientURL); err == nil {
				switch u.Hostname() {
				case ip:
					isSelf = true
				case joinURL.Hostname():
					isPreferred = true
				}
			}
		}
		if !member.IsLearner && !isSelf {
			if isPreferred {
				clientURLs = append(member.ClientURLs, clientURLs...)
			} else {
				clientURLs = append(clientURLs, member.ClientURLs...)
			}
		}
	}
	return clientURLs, memberList, nil
}

// Restore performs a restore of the ETCD datastore from
// the given snapshot path. This operation exists upon
// completion.
func (e *ETCD) Restore(ctx context.Context) error {
	// check the old etcd data dir
	oldDataDir := dbDir(e.config) + "-old-" + strconv.Itoa(int(time.Now().Unix()))
	if e.config.ClusterResetRestorePath == "" {
		return errors.New("no etcd restore path was specified")
	}
	// make sure snapshot exists before restoration
	if _, err := os.Stat(e.config.ClusterResetRestorePath); err != nil {
		return err
	}

	var restorePath string
	if strings.HasSuffix(e.config.ClusterResetRestorePath, snapshot.CompressedExtension) {
		dir, err := snapshotDir(e.config, true)
		if err != nil {
			return errors.Wrap(err, "failed to get the snapshot dir")
		}

		decompressSnapshot, err := e.decompressSnapshot(dir, e.config.ClusterResetRestorePath)
		if err != nil {
			return err
		}

		restorePath = decompressSnapshot
	} else {
		restorePath = e.config.ClusterResetRestorePath
	}

	// move the data directory to a temp path
	if err := os.Rename(dbDir(e.config), oldDataDir); err != nil {
		return err
	}

	logrus.Infof("Pre-restore etcd database moved to %s", oldDataDir)
	return snapshotv3.NewV3(e.client.GetLogger()).Restore(snapshotv3.RestoreConfig{
		SnapshotPath:   restorePath,
		Name:           e.name,
		OutputDataDir:  dbDir(e.config),
		OutputWALDir:   walDir(e.config),
		PeerURLs:       []string{e.peerURL()},
		InitialCluster: e.name + "=" + e.peerURL(),
	})
}

// backupDirWithRetention will move the dir to a backup dir
// and will keep only maxBackupRetention of dirs.
func backupDirWithRetention(dir string, maxBackupRetention int) (string, error) {
	backupDir := dir + "-backup-" + strconv.Itoa(int(time.Now().Unix()))
	if _, err := os.Stat(dir); err != nil {
		return "", nil
	}
	entries, err := os.ReadDir(filepath.Dir(dir))
	if err != nil {
		return "", err
	}
	files := make([]fs.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			return "", err
		}
		files = append(files, info)
	}
	if err != nil {
		return "", err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().After(files[j].ModTime())
	})
	count := 0
	for _, f := range files {
		if strings.HasPrefix(f.Name(), filepath.Base(dir)+"-backup") && f.IsDir() {
			count++
			if count > maxBackupRetention {
				if err := os.RemoveAll(filepath.Join(filepath.Dir(dir), f.Name())); err != nil {
					return "", err
				}
			}
		}
	}
	// move the directory to a temp path
	if err := os.Rename(dir, backupDir); err != nil {
		return "", err
	}
	return backupDir, nil
}

// GetAPIServerURLsFromETCD will try to fetch the version.Program/apiaddresses key from etcd
// and unmarshal it to a list of apiserver endpoints.
func GetAPIServerURLsFromETCD(ctx context.Context, cfg *config.Control) ([]string, error) {
	cl, err := getClient(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer cl.Close()

	etcdResp, err := cl.KV.Get(ctx, AddressKey)
	if err != nil {
		return nil, err
	}

	if etcdResp.Count == 0 || len(etcdResp.Kvs[0].Value) == 0 {
		return nil, ErrAddressNotSet
	}

	var addresses []string
	if err := json.Unmarshal(etcdResp.Kvs[0].Value, &addresses); err != nil {
		return nil, fmt.Errorf("failed to unmarshal apiserver addresses from etcd: %v", err)
	}

	return addresses, nil
}

// GetMembersClientURLs will list through the member lists in etcd and return
// back a combined list of client urls for each member in the cluster
func (e *ETCD) GetMembersClientURLs(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	members, err := e.client.MemberList(ctx)
	if err != nil {
		return nil, err
	}

	var clientURLs []string
	for _, member := range members.Members {
		if !member.IsLearner {
			clientURLs = append(clientURLs, member.ClientURLs...)
		}
	}
	return clientURLs, nil
}

// GetMembersNames will list through the member lists in etcd and return
// back a combined list of member names
func (e *ETCD) GetMembersNames(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	members, err := e.client.MemberList(ctx)
	if err != nil {
		return nil, err
	}

	var memberNames []string
	for _, member := range members.Members {
		memberNames = append(memberNames, member.Name)
	}
	return memberNames, nil
}

// RemoveSelf will remove the member if it exists in the cluster.  This should
// only be called on a node that may have previously run etcd, but will not
// currently run etcd, to ensure that it is not a member of the cluster.
// This is also called by tests to do cleanup between runs.
func (e *ETCD) RemoveSelf(ctx context.Context) error {
	if e.client == nil {
		if err := e.startClient(ctx); err != nil {
			return err
		}
	}

	if err := e.RemovePeer(ctx, e.name, e.address, true); err != nil {
		return err
	}

	// backup the data dir to avoid issues when re-enabling etcd
	oldDataDir := dbDir(e.config) + "-old-" + strconv.Itoa(int(time.Now().Unix()))

	// move the data directory to a temp path
	return os.Rename(dbDir(e.config), oldDataDir)
}
