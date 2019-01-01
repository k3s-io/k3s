package agent

import "github.com/urfave/cli"

type Agent struct {
	T_Token   string `desc:"Token to use for authentication" env:"K3S_TOKEN"`
	S_Server  string `desc:"Server to connect to" env:"K3S_URL"`
	D_DataDir string `desc:"Folder to hold state" default:"/var/lib/rancher/k3s"`
	L_Log     string `desc:"log to file"`
	AgentShared
}

type AgentShared struct {
	I_NodeIp string `desc:"IP address to advertise for node"`
}

func (a *Agent) Customize(command *cli.Command) {
	command.Category = "CLUSTER RUNTIME"
}
