// Copyright 2014-2016 CNI authors
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

// Package skel provides skeleton code for a CNI plugin.
// In particular, it implements argument parsing and validation.
package skel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
)

// CmdArgs captures all the arguments passed in to the plugin
// via both env vars and stdin
type CmdArgs struct {
	ContainerID string
	Netns       string
	IfName      string
	Args        string
	Path        string
	StdinData   []byte
}

type dispatcher struct {
	Getenv func(string) string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	ConfVersionDecoder version.ConfigDecoder
	VersionReconciler  version.Reconciler
}

type reqForCmdEntry map[string]bool

// internal only error to indicate lack of required environment variables
type missingEnvError struct {
	msg string
}

func (e missingEnvError) Error() string {
	return e.msg
}

func (t *dispatcher) getCmdArgsFromEnv() (string, *CmdArgs, error) {
	var cmd, contID, netns, ifName, args, path string

	vars := []struct {
		name      string
		val       *string
		reqForCmd reqForCmdEntry
	}{
		{
			"CNI_COMMAND",
			&cmd,
			reqForCmdEntry{
				"ADD":   true,
				"CHECK": true,
				"DEL":   true,
			},
		},
		{
			"CNI_CONTAINERID",
			&contID,
			reqForCmdEntry{
				"ADD":   true,
				"CHECK": true,
				"DEL":   true,
			},
		},
		{
			"CNI_NETNS",
			&netns,
			reqForCmdEntry{
				"ADD":   true,
				"CHECK": true,
				"DEL":   false,
			},
		},
		{
			"CNI_IFNAME",
			&ifName,
			reqForCmdEntry{
				"ADD":   true,
				"CHECK": true,
				"DEL":   true,
			},
		},
		{
			"CNI_ARGS",
			&args,
			reqForCmdEntry{
				"ADD":   false,
				"CHECK": false,
				"DEL":   false,
			},
		},
		{
			"CNI_PATH",
			&path,
			reqForCmdEntry{
				"ADD":   true,
				"CHECK": true,
				"DEL":   true,
			},
		},
	}

	argsMissing := make([]string, 0)
	for _, v := range vars {
		*v.val = t.Getenv(v.name)
		if *v.val == "" {
			if v.reqForCmd[cmd] || v.name == "CNI_COMMAND" {
				argsMissing = append(argsMissing, v.name)
			}
		}
	}

	if len(argsMissing) > 0 {
		joined := strings.Join(argsMissing, ",")
		return "", nil, missingEnvError{fmt.Sprintf("required env variables [%s] missing", joined)}
	}

	if cmd == "VERSION" {
		t.Stdin = bytes.NewReader(nil)
	}

	stdinData, err := ioutil.ReadAll(t.Stdin)
	if err != nil {
		return "", nil, fmt.Errorf("error reading from stdin: %v", err)
	}

	cmdArgs := &CmdArgs{
		ContainerID: contID,
		Netns:       netns,
		IfName:      ifName,
		Args:        args,
		Path:        path,
		StdinData:   stdinData,
	}
	return cmd, cmdArgs, nil
}

func createTypedError(f string, args ...interface{}) *types.Error {
	return &types.Error{
		Code: 100,
		Msg:  fmt.Sprintf(f, args...),
	}
}

func (t *dispatcher) checkVersionAndCall(cmdArgs *CmdArgs, pluginVersionInfo version.PluginInfo, toCall func(*CmdArgs) error) error {
	configVersion, err := t.ConfVersionDecoder.Decode(cmdArgs.StdinData)
	if err != nil {
		return err
	}
	verErr := t.VersionReconciler.Check(configVersion, pluginVersionInfo)
	if verErr != nil {
		return &types.Error{
			Code:    types.ErrIncompatibleCNIVersion,
			Msg:     "incompatible CNI versions",
			Details: verErr.Details(),
		}
	}

	return toCall(cmdArgs)
}

func validateConfig(jsonBytes []byte) error {
	var conf struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(jsonBytes, &conf); err != nil {
		return fmt.Errorf("error reading network config: %s", err)
	}
	if conf.Name == "" {
		return fmt.Errorf("missing network name")
	}
	return nil
}

func (t *dispatcher) pluginMain(cmdAdd, cmdCheck, cmdDel func(_ *CmdArgs) error, versionInfo version.PluginInfo, about string) *types.Error {
	cmd, cmdArgs, err := t.getCmdArgsFromEnv()
	if err != nil {
		// Print the about string to stderr when no command is set
		if _, ok := err.(missingEnvError); ok && t.Getenv("CNI_COMMAND") == "" && about != "" {
			fmt.Fprintln(t.Stderr, about)
			return nil
		}
		return createTypedError(err.Error())
	}

	if cmd != "VERSION" {
		err = validateConfig(cmdArgs.StdinData)
		if err != nil {
			return createTypedError(err.Error())
		}
	}

	switch cmd {
	case "ADD":
		err = t.checkVersionAndCall(cmdArgs, versionInfo, cmdAdd)
	case "CHECK":
		configVersion, err := t.ConfVersionDecoder.Decode(cmdArgs.StdinData)
		if err != nil {
			return createTypedError(err.Error())
		}
		if gtet, err := version.GreaterThanOrEqualTo(configVersion, "0.4.0"); err != nil {
			return createTypedError(err.Error())
		} else if !gtet {
			return &types.Error{
				Code: types.ErrIncompatibleCNIVersion,
				Msg:  "config version does not allow CHECK",
			}
		}
		for _, pluginVersion := range versionInfo.SupportedVersions() {
			gtet, err := version.GreaterThanOrEqualTo(pluginVersion, configVersion)
			if err != nil {
				return createTypedError(err.Error())
			} else if gtet {
				if err := t.checkVersionAndCall(cmdArgs, versionInfo, cmdCheck); err != nil {
					return createTypedError(err.Error())
				}
				return nil
			}
		}
		return &types.Error{
			Code: types.ErrIncompatibleCNIVersion,
			Msg:  "plugin version does not allow CHECK",
		}
	case "DEL":
		err = t.checkVersionAndCall(cmdArgs, versionInfo, cmdDel)
	case "VERSION":
		err = versionInfo.Encode(t.Stdout)
	default:
		return createTypedError("unknown CNI_COMMAND: %v", cmd)
	}

	if err != nil {
		if e, ok := err.(*types.Error); ok {
			// don't wrap Error in Error
			return e
		}
		return createTypedError(err.Error())
	}
	return nil
}

// PluginMainWithError is the core "main" for a plugin. It accepts
// callback functions for add, check, and del CNI commands and returns an error.
//
// The caller must also specify what CNI spec versions the plugin supports.
//
// It is the responsibility of the caller to check for non-nil error return.
//
// For a plugin to comply with the CNI spec, it must print any error to stdout
// as JSON and then exit with nonzero status code.
//
// To let this package automatically handle errors and call os.Exit(1) for you,
// use PluginMain() instead.
func PluginMainWithError(cmdAdd, cmdCheck, cmdDel func(_ *CmdArgs) error, versionInfo version.PluginInfo, about string) *types.Error {
	return (&dispatcher{
		Getenv: os.Getenv,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}).pluginMain(cmdAdd, cmdCheck, cmdDel, versionInfo, about)
}

// PluginMain is the core "main" for a plugin which includes automatic error handling.
//
// The caller must also specify what CNI spec versions the plugin supports.
//
// The caller can specify an "about" string, which is printed on stderr
// when no CNI_COMMAND is specified. The recommended output is "CNI plugin <foo> v<version>"
//
// When an error occurs in either cmdAdd, cmdCheck, or cmdDel, PluginMain will print the error
// as JSON to stdout and call os.Exit(1).
//
// To have more control over error handling, use PluginMainWithError() instead.
func PluginMain(cmdAdd, cmdCheck, cmdDel func(_ *CmdArgs) error, versionInfo version.PluginInfo, about string) {
	if e := PluginMainWithError(cmdAdd, cmdCheck, cmdDel, versionInfo, about); e != nil {
		if err := e.Print(); err != nil {
			log.Print("Error writing error JSON to stdout: ", err)
		}
		os.Exit(1)
	}
}
