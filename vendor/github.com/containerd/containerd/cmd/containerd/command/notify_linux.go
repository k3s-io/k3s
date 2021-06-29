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

package command

import (
	"context"

	sd "github.com/coreos/go-systemd/v22/daemon"

	"github.com/containerd/containerd/log"
)

// notifyReady notifies systemd that the daemon is ready to serve requests
func notifyReady(ctx context.Context) error {
	return sdNotify(ctx, sd.SdNotifyReady)
}

// notifyStopping notifies systemd that the daemon is about to be stopped
func notifyStopping(ctx context.Context) error {
	return sdNotify(ctx, sd.SdNotifyStopping)
}

func sdNotify(ctx context.Context, state string) error {
	notified, err := sd.SdNotify(false, state)
	log.G(ctx).
		WithError(err).
		WithField("notified", notified).
		WithField("state", state).
		Debug("sd notification")
	return err
}
