// +build !no_embedded_executor

package executor

import (
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/embed"
)

func (e Embedded) CurrentETCDOptions() (InitialOptions, error) {
	return InitialOptions{}, nil
}

func (e Embedded) ETCD(args ETCDConfig) error {
	configFile, err := args.ToConfigFile()
	if err != nil {
		return err
	}
	cfg, err := embed.ConfigFromFile(configFile)
	if err != nil {
		return err
	}
	etcd, err := embed.StartEtcd(cfg)
	if err != nil {
		return nil
	}

	go func() {
		select {
		case <-etcd.Server.StopNotify():
			logrus.Fatalf("etcd stopped - if this node was removed from the cluster, you must backup and delete %s before rejoining", args.DataDir)
		case err := <-etcd.Err():
			logrus.Fatalf("etcd exited: %v", err)
		}
	}()
	return nil
}
