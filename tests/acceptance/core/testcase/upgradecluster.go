package testcase

import (
	"fmt"
	"strings"
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

// upgradeNode upgrades a node server or agent type to the specified version
func upgradeNode(nodeType string, installType string, ips []string) error {
	var wg sync.WaitGroup
	var installFlag string
	errCh := make(chan error, len(ips))

	if strings.HasPrefix(installType, "v") {
		installFlag = fmt.Sprintf("INSTALL_K3S_VERSION=%s", installType)
	} else {
		installFlag = fmt.Sprintf("INSTALL_K3S_COMMIT=%s", installType)
	}

	channel := fmt.Sprintf("INSTALL_K3S_CHANNEL=%s", "stable")
	if customflag.ServiceFlag.InstallType.Channel != "" {
		channel = fmt.Sprintf("INSTALL_K3S_CHANNEL=%s", customflag.ServiceFlag.InstallType.Channel)
	}

	installK3s := "curl -sfL https://get.k3s.io | sudo %s %s sh -s - " + nodeType
	for _, ip := range ips {
		upgradeCommand := fmt.Sprintf(installK3s, installFlag, channel)
		wg.Add(1)
		go func(ip, installFlag string) {
			defer wg.Done()
			defer GinkgoRecover()

			fmt.Println("Upgrading " + nodeType + " to: " + upgradeCommand)
			if _, err := shared.RunCmdOnNode(upgradeCommand, ip); err != nil {
				fmt.Printf("\nError upgrading %s %s: %v\n\n", nodeType, ip, err)
				errCh <- err
				close(errCh)
				return
			}

			fmt.Println("Restarting " + nodeType + ": " + ip)
			if _, err := shared.RestartCluster(ip); err != nil {
				fmt.Printf("\nError restarting %s %s: %v\n\n", nodeType, ip, err)
				errCh <- err
				close(errCh)
				return
			}
			time.Sleep(20 * time.Second)
		}(ip, installType)
	}
	wg.Wait()
	close(errCh)

	return nil
}

func upgradeServer(installType string, serverIPs []string) error {
	return upgradeNode("server", installType, serverIPs)
}

func upgradeAgent(installType string, agentIPs []string) error {
	return upgradeNode("agent", installType, agentIPs)
}
