package etcdsnapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"text/tabwriter"
	"time"

	k3s "github.com/k3s-io/k3s/pkg/apis/k3s.cattle.io/v1"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/clientaccess"
	"github.com/k3s-io/k3s/pkg/cluster/managed"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/etcd"
	"github.com/k3s-io/k3s/pkg/proctitle"
	"github.com/k3s-io/k3s/pkg/server"
	util2 "github.com/k3s-io/k3s/pkg/util"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/printers"
)

var timeout = 2 * time.Minute

// commandSetup setups up common things needed
// for each etcd command.
func commandSetup(app *cli.Context, cfg *cmds.Server) (*etcd.SnapshotRequest, *clientaccess.Info, error) {
	// hide process arguments from ps output, since they may contain
	// database credentials or other secrets.
	proctitle.SetProcTitle(os.Args[0] + " etcd-snapshot")

	sr := &etcd.SnapshotRequest{}
	// Operation and name are set by the command handler.
	// Compression, dir, and retention take the server defaults if not overridden on the CLI.
	if app.IsSet("etcd-snapshot-compress") {
		sr.Compress = &cfg.EtcdSnapshotCompress
	}
	if app.IsSet("etcd-snapshot-dir") {
		sr.Dir = &cfg.EtcdSnapshotDir
	}
	if app.IsSet("etcd-snapshot-retention") {
		sr.Retention = &cfg.EtcdSnapshotRetention
	}

	if cfg.EtcdS3 {
		sr.S3 = &config.EtcdS3{
			AccessKey:     cfg.EtcdS3AccessKey,
			Bucket:        cfg.EtcdS3BucketName,
			ConfigSecret:  cfg.EtcdS3ConfigSecret,
			Endpoint:      cfg.EtcdS3Endpoint,
			EndpointCA:    cfg.EtcdS3EndpointCA,
			Folder:        cfg.EtcdS3Folder,
			Insecure:      cfg.EtcdS3Insecure,
			Proxy:         cfg.EtcdS3Proxy,
			Region:        cfg.EtcdS3Region,
			SecretKey:     cfg.EtcdS3SecretKey,
			SkipSSLVerify: cfg.EtcdS3SkipSSLVerify,
			Timeout:       metav1.Duration{Duration: cfg.EtcdS3Timeout},
		}
		// extend request timeout to allow the S3 operation to complete
		timeout += cfg.EtcdS3Timeout
	}

	dataDir, err := server.ResolveDataDir(cfg.DataDir)
	if err != nil {
		return nil, nil, err
	}

	if cfg.Token == "" {
		fp := filepath.Join(dataDir, "token")
		tokenByte, err := os.ReadFile(fp)
		if err != nil {
			return nil, nil, err
		}
		cfg.Token = string(bytes.TrimRight(tokenByte, "\n"))
	}
	info, err := clientaccess.ParseAndValidateToken(cmds.ServerConfig.ServerURL, cfg.Token, clientaccess.WithUser("server"))
	return sr, info, err
}

func wrapServerError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		// if the request timed out the server log likely won't contain anything useful,
		// since the operation may have actualy succeeded despite the client timing out the request.
		return err
	}
	return errors.Wrap(err, "see server log for details")
}

// Save triggers an on-demand etcd snapshot operation
func Save(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return save(app, &cmds.ServerConfig)
}

func save(app *cli.Context, cfg *cmds.Server) error {
	if len(app.Args()) > 0 {
		return util2.ErrCommandNoArgs
	}

	// Save always sets retention to 0 to disable automatic pruning.
	// Prune can be run manually after save, if desired.
	app.Set("etcd-snapshot-retention", "0")

	sr, info, err := commandSetup(app, cfg)
	if err != nil {
		return err
	}

	sr.Operation = etcd.SnapshotOperationSave
	sr.Name = []string{cfg.EtcdSnapshotName}

	b, err := json.Marshal(sr)
	if err != nil {
		return err
	}
	r, err := info.Post("/db/snapshot", b, clientaccess.WithTimeout(timeout))
	if err != nil {
		return wrapServerError(err)
	}
	resp := &managed.SnapshotResult{}
	if err := json.Unmarshal(r, resp); err != nil {
		return err
	}

	for _, name := range resp.Created {
		logrus.Infof("Snapshot %s saved.", name)
	}

	return nil
}

func Delete(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return delete(app, &cmds.ServerConfig)
}

func delete(app *cli.Context, cfg *cmds.Server) error {
	snapshots := app.Args()
	if len(snapshots) == 0 {
		return errors.New("no snapshots given for removal")
	}

	sr, info, err := commandSetup(app, cfg)
	if err != nil {
		return err
	}

	sr.Operation = etcd.SnapshotOperationDelete
	sr.Name = snapshots

	b, err := json.Marshal(sr)
	if err != nil {
		return err
	}
	r, err := info.Post("/db/snapshot", b, clientaccess.WithTimeout(timeout))
	if err != nil {
		return wrapServerError(err)
	}
	resp := &managed.SnapshotResult{}
	if err := json.Unmarshal(r, resp); err != nil {
		return err
	}

	for _, name := range resp.Deleted {
		logrus.Infof("Snapshot %s deleted.", name)
	}
	for _, name := range snapshots {
		if !slices.Contains(resp.Deleted, name) {
			logrus.Warnf("Snapshot %s not found.", name)
		}
	}

	return nil
}

func List(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return list(app, &cmds.ServerConfig)
}

var etcdListFormats = []string{"json", "yaml", "table"}

func validEtcdListFormat(format string) bool {
	for _, supportedFormat := range etcdListFormats {
		if format == supportedFormat {
			return true
		}
	}
	return false
}

func list(app *cli.Context, cfg *cmds.Server) error {
	if cfg.EtcdListFormat != "" && !validEtcdListFormat(cfg.EtcdListFormat) {
		return errors.New("invalid output format: " + cfg.EtcdListFormat)
	}

	sr, info, err := commandSetup(app, cfg)
	if err != nil {
		return err
	}

	sr.Operation = etcd.SnapshotOperationList

	b, err := json.Marshal(sr)
	if err != nil {
		return err
	}
	r, err := info.Post("/db/snapshot", b, clientaccess.WithTimeout(timeout))
	if err != nil {
		return wrapServerError(err)
	}

	sf := &k3s.ETCDSnapshotFileList{}
	if err := json.Unmarshal(r, sf); err != nil {
		return err
	}

	sort.Slice(sf.Items, func(i, j int) bool {
		if sf.Items[i].Status.CreationTime.Equal(sf.Items[j].Status.CreationTime) {
			return sf.Items[i].Spec.SnapshotName < sf.Items[j].Spec.SnapshotName
		}
		return sf.Items[i].Status.CreationTime.Before(sf.Items[j].Status.CreationTime)
	})

	switch cfg.EtcdListFormat {
	case "json":
		json := printers.JSONPrinter{}
		if err := json.PrintObj(sf, os.Stdout); err != nil {
			return err
		}
		return nil
	case "yaml":
		yaml := printers.YAMLPrinter{}
		if err := yaml.PrintObj(sf, os.Stdout); err != nil {
			return err
		}
		return nil
	default:
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', 0)
		defer w.Flush()

		fmt.Fprint(w, "Name\tLocation\tSize\tCreated\n")
		for _, esf := range sf.Items {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", esf.Spec.SnapshotName, esf.Spec.Location, esf.Status.Size.Value(), esf.Status.CreationTime.Format(time.RFC3339))
		}
	}

	return nil
}

func Prune(app *cli.Context) error {
	if err := cmds.InitLogging(); err != nil {
		return err
	}
	return prune(app, &cmds.ServerConfig)
}

func prune(app *cli.Context, cfg *cmds.Server) error {
	sr, info, err := commandSetup(app, cfg)
	if err != nil {
		return err
	}

	sr.Operation = etcd.SnapshotOperationPrune
	sr.Name = []string{cfg.EtcdSnapshotName}

	b, err := json.Marshal(sr)
	if err != nil {
		return err
	}
	r, err := info.Post("/db/snapshot", b, clientaccess.WithTimeout(timeout))
	if err != nil {
		return wrapServerError(err)
	}
	resp := &managed.SnapshotResult{}
	if err := json.Unmarshal(r, resp); err != nil {
		return err
	}

	for _, name := range resp.Deleted {
		logrus.Infof("Snapshot %s deleted.", name)
	}

	return nil
}
