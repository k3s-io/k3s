// +build linux

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package runc

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	"github.com/containerd/containerd/api/events"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/runtime"
	specs "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

// GetTopic converts an event from an interface type to the specific
// event topic id
func GetTopic(e interface{}) string {
	switch e.(type) {
	case *events.TaskCreate:
		return runtime.TaskCreateEventTopic
	case *events.TaskStart:
		return runtime.TaskStartEventTopic
	case *events.TaskOOM:
		return runtime.TaskOOMEventTopic
	case *events.TaskExit:
		return runtime.TaskExitEventTopic
	case *events.TaskDelete:
		return runtime.TaskDeleteEventTopic
	case *events.TaskExecAdded:
		return runtime.TaskExecAddedEventTopic
	case *events.TaskExecStarted:
		return runtime.TaskExecStartedEventTopic
	case *events.TaskPaused:
		return runtime.TaskPausedEventTopic
	case *events.TaskResumed:
		return runtime.TaskResumedEventTopic
	case *events.TaskCheckpointed:
		return runtime.TaskCheckpointedEventTopic
	default:
		logrus.Warnf("no topic for type %#v", e)
	}
	return runtime.TaskUnknownTopic
}

// ShouldKillAllOnExit reads the bundle's OCI spec and returns true if
// there is an error reading the spec or if the container has a private PID namespace
func ShouldKillAllOnExit(ctx context.Context, bundlePath string) bool {
	var bundleSpec specs.Spec
	bundleConfigContents, err := ioutil.ReadFile(filepath.Join(bundlePath, "config.json"))
	if err != nil {
		log.G(ctx).WithError(err).Error("shouldKillAllOnExit: failed to read config.json")
		return true
	}
	if err := json.Unmarshal(bundleConfigContents, &bundleSpec); err != nil {
		log.G(ctx).WithError(err).Error("shouldKillAllOnExit: failed to unmarshal bundle json")
		return true
	}
	if bundleSpec.Linux != nil {
		for _, ns := range bundleSpec.Linux.Namespaces {
			if ns.Type == specs.PIDNamespace && ns.Path == "" {
				return false
			}
		}
	}
	return true
}
