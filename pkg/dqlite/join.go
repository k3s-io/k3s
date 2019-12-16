package dqlite

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/canonical/go-dqlite/client"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

func (d *DQLite) Test(ctx context.Context) error {
	var ips []string
	peers, err := d.NodeStore.Get(ctx)
	if err != nil {
		return err
	}

	for _, peer := range peers {
		ips = append(ips, peer.Address)
	}

	logrus.Infof("Testing connection to peers %v", ips)
	if err := d.Join(ctx, nil); err != nil {
		return err
	}
	logrus.Infof("Connection OK to peers %v", ips)
	return nil
}

func (d *DQLite) Join(ctx context.Context, nodes []client.NodeInfo) error {
	if len(nodes) > 0 {
		if err := d.NodeStore.Set(ctx, nodes); err != nil {
			return err
		}
	}

	client, err := client.FindLeader(ctx, d.NodeStore, d.clientOpts...)
	if err != nil {
		return err
	}
	defer client.Close()

	current, err := client.Cluster(ctx)
	if err != nil {
		return err
	}

	for _, testNode := range current {
		if testNode.Address == d.NodeInfo.Address {
			nodeID, err := getClusterID(false, d.DataDir)
			if err != nil {
				return errors.Wrap(err, "get cluster ID")
			}
			if testNode.ID != nodeID {
				if err := d.node.Close(); err != nil {
					return errors.Wrap(err, "node close for id reset")
				}
				if err := writeClusterID(testNode.ID, d.DataDir); err != nil {
					return errors.Wrap(err, "restart node to reset ID")
				}
				return fmt.Errorf("reseting node ID from %d to %d, please restart", nodeID, testNode.ID)
			}
			return nil
		}
	}

	if found, err := cleanDir(d.DataDir, true); err != nil {
		return err
	} else if found {
		if err := d.node.Close(); err != nil {
			return errors.Wrap(err, "node close for cleaning")
		}
		_, _ = cleanDir(d.DataDir, false)
		return fmt.Errorf("cleaned DB directory, now restart and join")
	}

	logrus.Infof("Joining dqlite cluster as address=%s, id=%d", d.NodeInfo.Address, d.NodeInfo.ID)
	return client.Add(ctx, d.NodeInfo)
}

func cleanDir(dataDir string, check bool) (bool, error) {
	dbDir := GetDBDir(dataDir)
	backupDir := filepath.Join(dbDir, fmt.Sprintf(".backup-%d", time.Now().Unix()))
	files, err := ioutil.ReadDir(dbDir)
	if err != nil {
		return false, errors.Wrap(err, "cleaning dqlite DB dir")
	}

	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") || ignoreFile[file.Name()] {
			continue
		}
		if check {
			return true, nil
		}
		if err := os.MkdirAll(backupDir, 0700); err != nil {
			return false, errors.Wrapf(err, "creating backup dir %s", backupDir)
		}
		oldName := filepath.Join(dbDir, file.Name())
		newName := filepath.Join(backupDir, file.Name())
		logrus.Infof("Backing up %s => %s", oldName, newName)
		if err := os.Rename(oldName, newName); err != nil {
			return false, errors.Wrapf(err, "backup %s", oldName)
		}
	}

	return false, nil
}
