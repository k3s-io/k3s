// +build windows

// Copyright 2015 flannel authors
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

package powershell

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

//commandWrapper ensures that exceptions are written to stdout and the powershell process exit code is -1
const commandWrapper = `$ErrorActionPreference="Stop";try { %s } catch { Write-Host $_; os.Exit(-1) }`

// RunCommand executes a given powershell command.
//
// When the command throws a powershell exception, RunCommand will return the exception message as error.
func RunCommand(command string) ([]byte, error) {
	cmd := exec.Command("powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-Command", fmt.Sprintf(commandWrapper, command))

	stdout, err := cmd.Output()
	if err != nil {
		if cmd.ProcessState.ExitCode() != 0 {
			message := strings.TrimSpace(string(stdout))
			return []byte{}, errors.New(message)
		}

		return []byte{}, err
	}

	return stdout, nil
}

// RunCommandf executes a given powershell command. Command argument formats according to a format specifier (See fmt.Sprintf).
//
// When the command throws a powershell exception, RunCommandf will return the exception message as error.
func RunCommandf(command string, a ...interface{}) ([]byte, error) {
	return RunCommand(fmt.Sprintf(command, a...))
}

// RunCommandWithJsonResult executes a given powershell command.
// The command will be wrapped with ConvertTo-Json.
//
// You can Wrap your command with @(<cmd>) to ensure that the returned json is an array
//
// When the command throws a powershell exception, RunCommandf will return the exception message as error.
func RunCommandWithJsonResult(command string, v interface{}) error {
	wrappedCommand := fmt.Sprintf(commandWrapper, "ConvertTo-Json (%s)")
	wrappedCommand = fmt.Sprintf(wrappedCommand, command)

	stdout, err := RunCommandf(wrappedCommand)
	if err != nil {
		return err
	}

	err = json.Unmarshal(stdout, v)
	if err != nil {
		return err
	}

	return nil
}
