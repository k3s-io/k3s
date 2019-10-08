// Copyright 2017 flannel authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
// +build !windows

package ipsec

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bronze1man/goStrongswanVici"
	"github.com/coreos/flannel/subnet"
	log "k8s.io/klog"
	"golang.org/x/net/context"
)

type Uri struct {
	network, address string
}

type CharonIKEDaemon struct {
	viciUri     Uri
	espProposal string
	ctx         context.Context
}

func NewCharonIKEDaemon(ctx context.Context, wg sync.WaitGroup, espProposal string) (*CharonIKEDaemon, error) {

	charon := &CharonIKEDaemon{ctx: ctx, espProposal: espProposal}

	addr := strings.Split("unix:///var/run/charon.vici", "://")
	charon.viciUri = Uri{addr[0], addr[1]}

	cmd, err := charon.runBundled("/usr/lib/strongswan/", "charon")

	if err != nil {
		log.Errorf("Error starting charon daemon: %v", err)
		return nil, err
	} else {
		log.Info("Charon daemon started")
	}
	wg.Add(1)
	go func() {
		select {
		case <-ctx.Done():
			cmd.Process.Signal(syscall.SIGTERM)
			cmd.Wait()
			log.Infof("Stopped charon daemon")
			wg.Done()
		}
	}()
	return charon, nil
}

func (charon *CharonIKEDaemon) getClient(wait bool) (client *goStrongswanVici.ClientConn, err error) {
	for {
		socket_conn, err := net.Dial(charon.viciUri.network, charon.viciUri.address)
		if err == nil {
			return goStrongswanVici.NewClientConn(socket_conn), nil
		} else {
			if wait {
				select {
				case <-charon.ctx.Done():
					log.Error("Cancel waiting for charon")
					return nil, err
				default:
					log.Errorf("ClientConnection failed: %v", err)
				}

				log.Info("Retrying in a second ...")
				time.Sleep(time.Second)
			} else {
				return nil, err
			}
		}
	}
}

func (charon *CharonIKEDaemon) runBundled(staticLocation string, command string) (cmd *exec.Cmd, err error) {
	path, err := exec.LookPath(command)
	if err != nil {
		path = staticLocation + command
	}
	cmd = &exec.Cmd{
		Path: path,
		SysProcAttr: &syscall.SysProcAttr{
			Pdeathsig: syscall.SIGTERM,
		},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}
	err = cmd.Start()
	return
}

func (charon *CharonIKEDaemon) LoadSharedKey(remotePublicIP, password string) error {
	var err error
	var client *goStrongswanVici.ClientConn

	client, err = charon.getClient(true)
	if err != nil {
		log.Errorf("Failed to acquire Vici client: %v", err)
		return err
	}

	defer client.Close()

	sharedKey := &goStrongswanVici.Key{
		Typ:    "IKE",
		Data:   password,
		Owners: []string{remotePublicIP},
	}

	for {
		err = client.LoadShared(sharedKey)
		if err != nil {
			log.Errorf("Failed to load my key. Retrying. %v", err)
			time.Sleep(time.Second)
		} else {
			break
		}
	}

	log.Infof("Loaded shared key for: %v", remotePublicIP)
	return nil
}

func (charon *CharonIKEDaemon) LoadConnection(localLease, remoteLease *subnet.Lease,
	reqID, encap string) error {
	var err error
	var client *goStrongswanVici.ClientConn

	client, err = charon.getClient(true)
	if err != nil {
		log.Errorf("Failed to acquire Vici client: %s", err)
		return err
	}
	defer client.Close()

	childConfMap := make(map[string]goStrongswanVici.ChildSAConf)
	childSAConf := goStrongswanVici.ChildSAConf{
		Local_ts:     []string{localLease.Subnet.String()},
		Remote_ts:    []string{remoteLease.Subnet.String()},
		ESPProposals: []string{charon.espProposal},
		StartAction:  "start",
		CloseAction:  "trap",
		Mode:         "tunnel",
		ReqID:        reqID,
		//		RekeyTime:     rekeyTime,
		InstallPolicy: "no",
	}

	childSAConfName := formatChildSAConfName(localLease, remoteLease)

	childConfMap[childSAConfName] = childSAConf

	localAuthConf := goStrongswanVici.AuthConf{
		AuthMethod: "psk",
	}
	remoteAuthConf := goStrongswanVici.AuthConf{
		AuthMethod: "psk",
	}

	ikeConf := goStrongswanVici.IKEConf{
		LocalAddrs:  []string{localLease.Attrs.PublicIP.String()},
		RemoteAddrs: []string{remoteLease.Attrs.PublicIP.String()},
		Proposals:   []string{"aes256-sha256-modp4096"},
		Version:     "2",
		KeyingTries: "0", //continues to retry
		LocalAuth:   localAuthConf,
		RemoteAuth:  remoteAuthConf,
		Children:    childConfMap,
		Encap:       encap,
	}
	ikeConfMap := make(map[string]goStrongswanVici.IKEConf)

	connectionName := formatConnectionName(localLease, remoteLease)
	ikeConfMap[connectionName] = ikeConf

	err = client.LoadConn(&ikeConfMap)
	if err != nil {
		return err
	}

	log.Infof("Loaded connection: %v", connectionName)
	return nil
}

func (charon *CharonIKEDaemon) UnloadCharonConnection(localLease,
	remoteLease *subnet.Lease) error {
	client, err := charon.getClient(false)
	if err != nil {
		log.Errorf("Failed to acquire Vici client: %s", err)
		return err
	}
	defer client.Close()

	connectionName := formatConnectionName(localLease, remoteLease)
	unloadConnRequest := &goStrongswanVici.UnloadConnRequest{
		Name: connectionName,
	}

	err = client.UnloadConn(unloadConnRequest)
	if err != nil {
		return err
	}

	log.Infof("Unloaded connection: %v", connectionName)
	return nil
}

func formatConnectionName(localLease, remoteLease *subnet.Lease) string {
	return fmt.Sprintf("%s-%s-%s-%s", localLease.Attrs.PublicIP,
		localLease.Subnet, remoteLease.Subnet, remoteLease.Attrs.PublicIP)
}

func formatChildSAConfName(localLease, remoteLease *subnet.Lease) string {
	return fmt.Sprintf("%s-%s", localLease.Subnet, remoteLease.Subnet)
}
