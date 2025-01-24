package containerd

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/fsnotify/fsnotify"
	"github.com/k3s-io/k3s/pkg/agent/cri"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/pkg/errors"
	"github.com/rancher/wharfie/pkg/tarfile"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/util/workqueue"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type Watcher struct {
	watcher    *fsnotify.Watcher
	filesCache map[string]fs.FileInfo
	workqueue  workqueue.TypedDelayingInterface[string]
}

func CreateWatcher() (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	return &Watcher{
		watcher:    watcher,
		filesCache: make(map[string]fs.FileInfo),
		workqueue:  workqueue.TypedNewDelayingQueue[string](),
	}, nil
}

func isFileSupported(path string) bool {
	for _, ext := range append(tarfile.SupportedExtensions, ".txt") {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}

	return false
}

func (w *Watcher) HandleWatch(path string) error {
	if err := w.watcher.Add(path); err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to watch from %s directory: %v", path, err))
	}

	return nil
}

// Populate the state of the files in the directory
// for the watcher to have infos about the file changing
// this function need to break
func (w *Watcher) Populate(path string) error {
	var errs []error

	fileInfos, err := os.ReadDir(path)
	if err != nil {
		logrus.Errorf("Unable to read files in %s: %v", path, err)
		return err
	}

	for _, dirEntry := range fileInfos {
		if dirEntry.IsDir() {
			continue
		}

		// get the file info to add to the state map
		fileInfo, err := dirEntry.Info()
		if err != nil {
			logrus.Errorf("Failed while getting the info from file: %v", err)
			errs = append(errs, err)
			continue
		}

		if isFileSupported(dirEntry.Name()) {
			// insert the file into the state map that will have the state from the file
			w.filesCache[filepath.Join(path, dirEntry.Name())] = fileInfo
		}
	}

	return merr.NewErrors(errs...)
}

func (w *Watcher) ClearMap() {
	w.filesCache = make(map[string]fs.FileInfo)
}

func (w *Watcher) runWorkerForImages(ctx context.Context, cfg *config.Node) {
	// create the connections to not create every time when processing a event
	client, err := Client(cfg.Containerd.Address)
	if err != nil {
		logrus.Errorf("Failed to create containerd client: %v", err)
		w.watcher.Close()
		return
	}

	defer client.Close()

	criConn, err := cri.Connection(ctx, cfg.Containerd.Address)
	if err != nil {
		logrus.Errorf("Failed to create CRI connection: %v", err)
		w.watcher.Close()
		return
	}

	defer criConn.Close()

	imageClient := runtimeapi.NewImageServiceClient(criConn)

	for w.processNextEventForImages(ctx, cfg, client, imageClient) {
	}
}

func (w *Watcher) processNextEventForImages(ctx context.Context, cfg *config.Node, client *containerd.Client, imageClient runtimeapi.ImageServiceClient) bool {
	key, shutdown := w.workqueue.Get()

	if shutdown {
		return false
	}

	if err := w.processImageEvent(ctx, key, cfg, client, imageClient); err != nil {
		logrus.Errorf("Failed to process image event: %v", err)
	}

	return true
}

func (w *Watcher) processImageEvent(ctx context.Context, key string, cfg *config.Node, client *containerd.Client, imageClient runtimeapi.ImageServiceClient) error {
	defer w.workqueue.Done(key)

	file, err := os.Stat(key)
	// if the file does not exists, we assume that the event was RENAMED or REMOVED
	if os.IsNotExist(err) {
		if key == cfg.Images {
			w.ClearMap()
			return nil
		}

		if !isFileSupported(key) {
			return nil
		}

		delete(w.filesCache, key)
		logrus.Debugf("File removed from the image watcher controller: %s", key)
		return nil
	} else if err != nil {
		logrus.Errorf("Failed to get file %s info for image event: %v", key, err)
		return err
	}

	if file.IsDir() {
		// only add the image watcher, populate and search for images when it is the images folder
		if key == cfg.Images {
			if err := w.HandleWatch(cfg.Images); err != nil {
				logrus.Errorf("Failed to watch %s: %v", cfg.Images, err)
				return err
			}

			if err := w.Populate(cfg.Images); err != nil {
				logrus.Errorf("Failed to populate %s files: %v", cfg.Images, err)
				return err
			}

			// Read the directory to see if the created folder has files inside
			fileInfos, err := os.ReadDir(cfg.Images)
			if err != nil {
				logrus.Errorf("Unable to read images in %s: %v", cfg.Images, err)
				return err
			}

			for _, fileInfo := range fileInfos {
				if fileInfo.IsDir() {
					continue
				}

				start := time.Now()
				filePath := filepath.Join(cfg.Images, fileInfo.Name())

				if err := preloadFile(ctx, cfg, client, imageClient, filePath); err != nil {
					logrus.Errorf("Error encountered while importing %s: %v", filePath, err)
					continue
				}
				logrus.Infof("Imported images from %s in %s", filePath, time.Since(start))
			}
		}

		return nil
	}

	if !isFileSupported(key) {
		return nil
	}

	lastStateFile := w.filesCache[key]
	w.filesCache[key] = file
	if lastStateFile == nil || (file.Size() != lastStateFile.Size()) && file.ModTime().After(lastStateFile.ModTime()) {
		logrus.Debugf("File met the requirements for import to containerd image store: %s", key)
		start := time.Now()
		if err := preloadFile(ctx, cfg, client, imageClient, key); err != nil {
			logrus.Errorf("Failed to import %s: %v", key, err)
			return err
		}
		logrus.Infof("Imported images from %s in %s", key, time.Since(start))
	}

	return nil
}

func watchImages(ctx context.Context, cfg *config.Node) {
	w, err := CreateWatcher()
	if err != nil {
		logrus.Errorf("Failed to create image watcher: %v", err)
		return
	}

	logrus.Debugf("Image Watcher created")
	defer w.watcher.Close()

	if err := w.HandleWatch(filepath.Dir(cfg.Images)); err != nil {
		logrus.Errorf("Failed to watch %s: %v", filepath.Dir(cfg.Images), err)
		return
	}

	_, err = os.Stat(cfg.Images)
	if err == nil {
		if err := w.HandleWatch(cfg.Images); err != nil {
			logrus.Errorf("Failed to watch %s: %v", cfg.Images, err)
			return
		}

		if err := w.Populate(cfg.Images); err != nil {
			logrus.Errorf("Failed to populate %s files: %v", cfg.Images, err)
			return
		}
	} else if os.IsNotExist(err) {
		logrus.Debugf("Image dir %s does not exist", cfg.Images)
	} else {
		logrus.Debugf("Failed to stat image dir %s: %v", cfg.Images, err)
	}

	go w.runWorkerForImages(ctx, cfg)

	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				logrus.Info("Image watcher channel closed, shutting down workqueue and retrying in 5 seconds")
				w.workqueue.ShutDown()
				select {
				case <-time.After(time.Second * 5):
					go watchImages(ctx, cfg)
					return
				case <-ctx.Done():
					return
				}

			}

			// this part is to specify to only get events that were from /agent/images
			if strings.Contains(event.Name, "/agent/images") {
				w.workqueue.AddAfter(event.Name, 2*time.Second)
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				logrus.Info("Image watcher channel closed, shutting down workqueue and retrying in 5 seconds")
				w.workqueue.ShutDown()
				select {
				case <-time.After(time.Second * 5):
					go watchImages(ctx, cfg)
					return
				case <-ctx.Done():
					return
				}
			}
			logrus.Errorf("Image watcher received an error: %v", err)
		}
	}
}
