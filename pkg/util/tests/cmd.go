package tests

import (
	"bufio"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// func K3sApp(cmdArgs ...string) (*logrustest.Hook, error) {
// 	app := cmds.NewApp()
// 	app.Commands = []cli.Command{
// 		cmds.NewServerCommand(server.Run),
// 		cmds.NewAgentCommand(agent.Run),
// 		cmds.NewKubectlCommand(kubectl.Run),
// 		cmds.NewCRICTL(crictl.Run),
// 		cmds.NewEtcdSnapshotCommand(etcdsnapshot.Run,
// 			cmds.NewEtcdSnapshotSubcommands(
// 				etcdsnapshot.Delete,
// 				etcdsnapshot.List,
// 				etcdsnapshot.Prune,
// 				etcdsnapshot.Run),
// 		),
// 	}
// 	_, hook := logrustest.NewNullLogger()
// 	return hook, app.Run(configfilearg.MustParse(cmdArgs))
// }

func findK3sExecutable() string {
	k3sBin := "dist/artifacts/k3s"
	for {
		_, err := os.Stat(k3sBin)
		if err != nil {
			k3sBin = "../" + k3sBin
			continue
		}
		break
	}
	return k3sBin
}

func K3sCmd(cmdName string, cmdArgs ...string) (string, error) {
	k3sBin := findK3sExecutable()
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	// Only run sudo if not root
	var cmd *exec.Cmd
	if currentUser.Username == "root" {
		k3sCmd := append([]string{cmdName}, cmdArgs...)
		cmd = exec.Command(k3sBin, k3sCmd...)
	} else {
		k3sCmd := append([]string{"-s", k3sBin, cmdName}, cmdArgs...)
		cmd = exec.Command("sudo", k3sCmd...)
	}
	byteOut, err := cmd.CombinedOutput()
	return string(byteOut), err
}

func FindStringInCmdAsync(scanner *bufio.Scanner, target string) bool {
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), target) {
			return true
		}
	}
	return false
}

//Launch a k3s command asynchronously
func K3sCmdAsync(cmdName string, cmdArgs ...string) (*exec.Cmd, *bufio.Scanner, error) {
	k3sBin := findK3sExecutable()
	currentUser, err := user.Current()
	if err != nil {
		return nil, nil, err
	}
	// Only run sudo if not root
	var cmd *exec.Cmd
	if currentUser.Username == "root" {
		k3sCmd := append([]string{cmdName}, cmdArgs...)
		cmd = exec.Command(k3sBin, k3sCmd...)
	} else {
		k3sCmd := append([]string{"-s", k3sBin, cmdName}, cmdArgs...)
		cmd = exec.Command("sudo", k3sCmd...)
	}
	cmdOut, _ := cmd.StderrPipe()
	err = cmd.Start()
	return cmd, bufio.NewScanner(cmdOut), err
}
