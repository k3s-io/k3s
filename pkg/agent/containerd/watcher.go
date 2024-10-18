package containerd

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/cri/constants"
	"github.com/fsnotify/fsnotify"
	"github.com/k3s-io/k3s/pkg/agent/cri"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/workqueue"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type Watcher struct {
	nodeConfig  *config.Node
	imagesMap   map[string]fs.FileInfo
	workqueue   workqueue.TypedRateLimitingInterface[fsnotify.Event]
}

func createWatcher(ctx context.Context, cfg *config.Node) *Watcher {
	return &Watcher{
		nodeConfig:  cfg,
		imagesMap:   make(map[string]fs.FileInfo),
		workqueue:   workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[fsnotify.Event]()),
	}
}

func (w *Watcher) runWorker() {
	for w.processNextEvent() {
	}
}

func (w *Watcher) processNextEvent() bool {
	_, shutdown := w.workqueue.Get()

	if shutdown {
		return false
	}

	if err := w.processEvent(event); err != nil {
		logrus.Errorf("Process event: %v", err)
	}

	return true
}

func (w *Watcher) processEvent(event fsnotify.Event) error {
	defer w.workqueue.Done(event)

	switch event.Op {
	case fsnotify.Write:
		newStateFile, err := os.Stat(event.Name)
		if err != nil {
			logrus.Errorf("Error encountered while getting file %s info for event write: %s", event.Name, err.Error())
			continue
		}

		// we do not want to handle directorys, only files
		if newStateFile.IsDir() {
			continue
		}

		lastStateFile := w.imagesMap[event.Name]
		w.imagesMap[event.Name] = newStateFile

		if (newStateFile.Size() != lastStateFile.Size()) && newStateFile.ModTime().After(lastStateFile.ModTime()) {
			logrus.Infof("File met the requirements for import to containerd image store: %s", event.Name)
			w.workqueue.Add(event)
			// start := time.Now()
			// if err := preloadFile(ctx, cfg, client, imageClient, event.Name); err != nil {
			// 	logrus.Errorf("Error encountered while importing %s: %v", event.Name, err)
			// 	continue
			// }
			// logrus.Infof("Imported images from %s in %s", event.Name, time.Since(start))
		}
	case fsnotify.Create:
		info, err := os.Stat(event.Name)
		if err != nil {
			logrus.Errorf("Error encountered while getting file %s info for event Create: %v", event.Name, err)
			continue
		}

		if info.IsDir() {
			continue
		}

		w.imagesMap[event.Name] = info
		logrus.Infof("File added to watcher controller: %s", event.Name)

		w.workqueue.Add(event)

		// start := time.Now()
		// if err := preloadFile(ctx, cfg, client, imageClient, event.Name); err != nil {
		// 	logrus.Errorf("Error encountered while importing %s: %v", event.Name, err)
		// 	continue
		// }
		// logrus.Infof("Imported images from %s in %s", event.Name, time.Since(start))
	case fsnotify.Rename:
		delete(w.imagesMap, event.Name)
		logrus.Infof("Removed file from the watcher controller: %s", event.Name)
	case fsnotify.Remove:
		w.workqueue.Add(event)
		delete(w.imagesMap, event.Name)
		logrus.Infof("Removed file from the watcher controller: %s", event.Name)
	}
	}
	// if key, ok = obj.(string); !ok {
	// 	logrus.Errorf("expected string in workqueue but got %#v", obj)
	// 	w.workqueue.Forget(event)
	// 	return nil
	// }
	// keyParts := strings.SplitN(key, "/", 2)
	// if err := k.updateStatus(keyParts[0], keyParts[1]); err != nil {
	// 	w.workqueue.AddRateLimited(event)
	// 	return fmt.Errorf("error updating LoadBalancer Status for %s: %v, requeueing", key, err)
	// }

	c.workqueue.Forget(obj)
	return nil

}

func (w *Watcher) handleCreateImages(event fsnotify.Event) {

}

// watcher is a controller that watch the agent/images folder
// to ensure that every new file is added to the watcher state
func (w *Watcher) run(ctx context.Context, cfg *config.Node) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Errorf("Error to create a watcher: %s", err.Error())
		return
	}

	// Add agent/images path to the watcher.
	err = watcher.Add(w.nodeConfig.Images)
	if err != nil {
		logrus.Errorf("Error when creating the watcher controller: %v", err)
		return
	}
	//var ImagesWorkqueue workqueue.TypedRateLimitingInterface[fsnotify.Event]

	client, err := Client(cfg.Containerd.Address)
	if err != nil {
		logrus.Errorf("Error to create containerd client: %v", err)
		return
	}

	criConn, err := cri.Connection(ctx, cfg.Containerd.Address)
	if err != nil {
		logrus.Errorf("Error to create CRI connection: %v", err)
		return
	}

	_ = runtimeapi.NewImageServiceClient(criConn)

	defer watcher.Close()
	defer client.Close()
	defer criConn.Close()

	fileInfos, err := os.ReadDir(w.nodeConfig.Images)
	if err != nil {
		logrus.Errorf("Unable to read images in %s: %v", w.nodeConfig.Images, err)
		return
	}

	// Ensure that our images are imported into the correct namespace
	ctx = namespaces.WithNamespace(ctx, constants.K8sContainerdNamespace)

	// populate watcher map with the entrys from the directory
	for _, dirEntry := range fileInfos {
		if dirEntry.IsDir() {
			continue
		}

		// get the file info to add to the state map
		fileInfo, err := dirEntry.Info()
		if err != nil {
			logrus.Errorf("Error while getting the info from file: %v", err)
			continue
		}

		// insert the file into the state map that will have the state from the file
		w.imagesMap[filepath.Join(w.nodeConfig.Images, dirEntry.Name())] = fileInfo
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			w.workqueue.Add(event)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logrus.Errorf("error in watcher controller: %v", err)
		}
	}
}
