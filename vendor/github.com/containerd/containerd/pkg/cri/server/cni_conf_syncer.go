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

package server

import (
	"os"
	"sync"

	cni "github.com/containerd/go-cni"
	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// cniNetConfSyncer is used to reload cni network conf triggered by fs change
// events.
type cniNetConfSyncer struct {
	// only used for lastSyncStatus
	sync.RWMutex
	lastSyncStatus error

	watcher   *fsnotify.Watcher
	confDir   string
	netPlugin cni.CNI
	loadOpts  []cni.Opt
}

// newCNINetConfSyncer creates cni network conf syncer.
func newCNINetConfSyncer(confDir string, netPlugin cni.CNI, loadOpts []cni.Opt) (*cniNetConfSyncer, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create fsnotify watcher")
	}

	if err := os.MkdirAll(confDir, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to create cni conf dir=%s for watch", confDir)
	}

	if err := watcher.Add(confDir); err != nil {
		return nil, errors.Wrapf(err, "failed to watch cni conf dir %s", confDir)
	}

	syncer := &cniNetConfSyncer{
		watcher:   watcher,
		confDir:   confDir,
		netPlugin: netPlugin,
		loadOpts:  loadOpts,
	}

	if err := syncer.netPlugin.Load(syncer.loadOpts...); err != nil {
		logrus.WithError(err).Error("failed to load cni during init, please check CRI plugin status before setting up network for pods")
		syncer.updateLastStatus(err)
	}
	return syncer, nil
}

// syncLoop monitors any fs change events from cni conf dir and tries to reload
// cni configuration.
func (syncer *cniNetConfSyncer) syncLoop() error {
	for {
		select {
		case event, ok := <-syncer.watcher.Events:
			if !ok {
				logrus.Debugf("cni watcher channel is closed")
				return nil
			}
			// Only reload config when receiving write/rename/remove
			// events
			//
			// TODO(fuweid): Might only reload target cni config
			// files to prevent no-ops.
			if event.Op&(fsnotify.Chmod|fsnotify.Create) > 0 {
				logrus.Debugf("ignore event from cni conf dir: %s", event)
				continue
			}
			logrus.Debugf("receiving change event from cni conf dir: %s", event)

			lerr := syncer.netPlugin.Load(syncer.loadOpts...)
			if lerr != nil {
				logrus.WithError(lerr).
					Errorf("failed to reload cni configuration after receiving fs change event(%s)", event)
			}
			syncer.updateLastStatus(lerr)

		case err := <-syncer.watcher.Errors:
			if err != nil {
				logrus.WithError(err).Error("failed to continue sync cni conf change")
				return err
			}
		}
	}
}

// lastStatus retrieves last sync status.
func (syncer *cniNetConfSyncer) lastStatus() error {
	syncer.RLock()
	defer syncer.RUnlock()
	return syncer.lastSyncStatus
}

// updateLastStatus will be called after every single cni load.
func (syncer *cniNetConfSyncer) updateLastStatus(err error) {
	syncer.Lock()
	defer syncer.Unlock()
	syncer.lastSyncStatus = err
}

// stop stops watcher in the syncLoop.
func (syncer *cniNetConfSyncer) stop() error {
	return syncer.watcher.Close()
}
