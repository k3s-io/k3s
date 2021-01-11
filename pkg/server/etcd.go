package server

import (
	"context"
	"io/ioutil"
	net2 "net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/daemons/control"
	"github.com/rancher/k3s/pkg/etcd"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/net"
)

func StartETCD(ctx context.Context, config *Config) error {
	if err := setupDataDirAndChdir(&config.ControlConfig); err != nil {
		return err
	}

	if err := control.ETCD(ctx, &config.ControlConfig); err != nil {
		return errors.Wrap(err, "starting etcd")
	}

	config.ControlConfig.Runtime.Handler = router(&config.ControlConfig)

	go setETCDLabelsAndAnnotations(ctx, config)

	ip := net2.ParseIP(config.ControlConfig.BindAddress)
	if ip == nil {
		hostIP, err := net.ChooseHostInterface()
		if err == nil {
			ip = hostIP
		} else {
			ip = net2.ParseIP("127.0.0.1")
		}
	}

	if err := printTokens(ip.String(), &config.ControlConfig); err != nil {
		return err
	}

	return writeKubeConfig(config.ControlConfig.Runtime.ServerCA, config)
}

func setETCDLabelsAndAnnotations(ctx context.Context, config *Config) error {
	t := time.NewTicker(5 * time.Second)
	defer t.Stop()
	for range t.C {
		controlConfig := &config.ControlConfig

		sc, err := newContext(ctx, controlConfig.Runtime.KubeConfigAdmin)
		if err != nil {
			logrus.Infof("Waiting for control-plane node agent startup %v", err)
			continue
		}

		if err := stageFiles(ctx, sc, controlConfig); err != nil {
			logrus.Infof("Waiting for control-plane node agent startup %v", err)
			continue
		}

		if err := sc.Start(ctx); err != nil {
			logrus.Infof("Waiting for control-plane node agent startup %v", err)
			continue
		}

		controlConfig.Runtime.Core = sc.Core
		nodes := sc.Core.Core().V1().Node()

		nodeName := os.Getenv("NODE_NAME")
		if nodeName == "" {
			logrus.Info("Waiting for control-plane node agent startup")
			continue
		}
		node, err := nodes.Get(nodeName, metav1.GetOptions{})
		if err != nil {
			logrus.Infof("Waiting for control-plane node %s startup: %v", nodeName, err)
			continue
		}
		if v, ok := node.Labels[ETCDRoleLabelKey]; ok && v == "true" {
			break
		}
		if node.Labels == nil {
			node.Labels = make(map[string]string)
		}
		node.Labels[ETCDRoleLabelKey] = "true"

		// this is replacement to the etcd controller handleself function
		if node.Annotations == nil {
			node.Annotations = map[string]string{}
		}
		fileName := filepath.Join(controlConfig.DataDir, "db", "etcd", "name")

		data, err := ioutil.ReadFile(fileName)
		if err != nil {
			logrus.Infof("Waiting for etcd node name file to be available: %v", err)
			continue
		}
		etcdNodeName := string(data)
		node.Annotations[etcd.NodeID] = etcdNodeName

		address, err := etcd.GetAdvertiseAddress(controlConfig.PrivateIP)
		if err != nil {
			logrus.Infof("Waiting for etcd node address to be available: %v", err)
			continue
		}
		node.Annotations[etcd.NodeAddress] = address

		_, err = nodes.Update(node)
		if err == nil {
			logrus.Infof("ETCD role label and annotations has been set successfully on node: %s", nodeName)
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return nil
}
