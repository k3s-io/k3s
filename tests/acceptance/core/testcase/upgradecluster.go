package testcase

import (
	"fmt"
	"sync"
	"time"

	"github.com/k3s-io/k3s/tests/acceptance/core/service/customflag"
	"github.com/k3s-io/k3s/tests/acceptance/core/service/factory"
	"github.com/k3s-io/k3s/tests/acceptance/shared"

	. "github.com/onsi/ginkgo/v2"
)

// TestUpgradeClusterManually upgrades the cluster "manually"
func TestUpgradeClusterManually(version string) error {
	if version == "" {
		return fmt.Errorf("please provide a non-empty k3s version or commit to upgrade to")
	}
	cluster := factory.GetCluster(GinkgoT())

	if cluster.NumServers == 0 && cluster.NumAgents == 0 {
		return fmt.Errorf("no nodes found to upgrade")
	}

	if cluster.NumServers > 0 {
		if err := upgradeServer(version, cluster.ServerIPs); err != nil {
			return err
		}
	}

	if cluster.NumAgents > 0 {
		if err := upgradeAgent(version, cluster.AgentIPs); err != nil {
			return err
		}
	}

	return nil
}

// upgradeServer upgrades servers in the cluster.
func upgradeServer(installType string, serverIPs []string) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(serverIPs))

	for _, ip := range serverIPs {
		switch {
		case customflag.ServiceFlag.InstallType.Version != "":
			installType = fmt.Sprintf("INSTALL_K3S_VERSION=%s", customflag.ServiceFlag.InstallType.Version)
		case customflag.ServiceFlag.InstallType.Commit != "":
			installType = fmt.Sprintf("INSTALL_K3S_COMMIT=%s", customflag.ServiceFlag.InstallType.Commit)
		}

		installK3sServer := "curl -sfL https://get.k3s.io | sudo %s  sh -s - server"
		upgradeCommand := fmt.Sprintf(installK3sServer, installType)

		wg.Add(1)
		go func(ip, installFlagServer string) {
			defer wg.Done()
			defer GinkgoRecover()

			fmt.Println("Upgrading server to:  " + upgradeCommand)
			if _, err := shared.RunCmdOnNode(upgradeCommand, ip); err != nil {
				fmt.Printf("Error upgrading server %s: %v\n\n", ip, err)
				errCh <- err
				close(errCh)
				return
			}
			time.Sleep(10 * time.Second)
			fmt.Println("Restarting server: " + ip)
			if _, err := shared.RestartCluster(ip); err != nil {
				fmt.Printf("Error restarting server %s: %v\n\n", ip, err)
				errCh <- err
				close(errCh)
				return
			}
			time.Sleep(10 * time.Second)
		}(ip, installType)
	}
	wg.Wait()
	close(errCh)

	return nil
}

// upgradeAgent upgrades agents in the cluster.
func upgradeAgent(installType string, agentIPs []string) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(agentIPs))

	for _, ip := range agentIPs {
		switch {
		case customflag.ServiceFlag.InstallType.Version != "":
			installType = fmt.Sprintf("INSTALL_K3S_VERSION=%s", customflag.ServiceFlag.InstallType.Version)
		case customflag.ServiceFlag.InstallType.Commit != "":
			installType = fmt.Sprintf("INSTALL_K3S_COMMIT=%s", customflag.ServiceFlag.InstallType.Commit)
		}

		installK3sAgent := "curl -sfL https://get.k3s.io | sudo %s sh -s - agent"
		upgradeCommand := fmt.Sprintf(installK3sAgent, installType)

		fmt.Println("\nUpgrading agent to: " + upgradeCommand)
		wg.Add(1)
		go func(ip, installFlagAgent string) {
			defer wg.Done()
			defer GinkgoRecover()

			if _, err := shared.RunCmdOnNode(upgradeCommand, ip); err != nil {
				fmt.Printf("Error upgrading agent %s: %v\n\n", ip, err)
				errCh <- err
				close(errCh)
				return
			}
		}(ip, installType)
	}
	wg.Wait()
	close(errCh)

	return nil
}
