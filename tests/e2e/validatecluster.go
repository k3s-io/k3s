package e2e

import (
	"strconv"
	"fmt"
	"testing"
)

var (
	Kubeconfig string
	port string
	Ports string
	serverNodenames string
	agentNodenames string
	err error
)

func CreateCluster(nodeos string, serverCount int, agentCount int, t *testing.T) (string, [] string, [] string, error) {
	scount := make([]int, serverCount)
	serverNodenames := make([]string, serverCount+1)
	for i := range scount {
		serverNodenames[i] = "server-" + strconv.Itoa(i)
	}

	acount := make([]int, agentCount)
	agentNodenames := make([]string, agentCount+1)
	for i := range acount {
		agentNodenames[i] = "agent-" + strconv.Itoa(i)
	}

	cmd := "vagrant up"
	_, err := RunCommand(cmd)
	if err != nil {
		fmt.Printf("Error Creating Cluster")
		return "", nil, nil, err
	} else {
		cmd = "vagrant ssh server-0 -c \"cat /etc/rancher/k3s/k3s.yaml\""
		Kubeconfig, err = RunCommand(cmd)
	}

	return Kubeconfig, serverNodenames, agentNodenames, err
}
