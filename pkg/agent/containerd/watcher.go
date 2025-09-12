package containerd

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/fsnotify/fsnotify"
	"github.com/k3s-io/k3s/pkg/agent/cri"
	"github.com/k3s-io/k3s/pkg/daemons/config"
	pkgerrors "github.com/pkg/errors"
	"github.com/rancher/wharfie/pkg/tarfile"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

type fileInfo struct {
	Size    int64       `json:"size"`
	ModTime metav1.Time `json:"modTime"`
	seen    bool        // field is not serialized, and can be used to track if a file has been seen since the last restart
}

type watchqueue struct {
	cfg        *config.Node
	watcher    *fsnotify.Watcher
	filesCache map[string]*fileInfo
	workqueue  workqueue.TypedDelayingInterface[string]
}

func createWatcher(path string) (*fsnotify.Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := watcher.Add(path); err != nil {
		return nil, err
	}

	return watcher, nil
}

func mustCreateWatcher(path string) *fsnotify.Watcher {
	watcher, err := createWatcher(path)
	if err != nil {
		panic("Failed to create image import watcher:" + err.Error())
	}
	return watcher
}

func isFileSupported(path string) bool {
	for _, ext := range append(tarfile.SupportedExtensions, ".txt") {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}

	return false
}

// runWorkerForImages connects to containerd and calls processNextEventForImages to process items from the workqueue.
// This blocks until the workqueue is shut down.
func (w *watchqueue) runWorkerForImages(ctx context.Context) {
	// create the connections to not create every time when processing a event
	client, err := Client(w.cfg.Containerd.Address)
	if err != nil {
		logrus.Errorf("Failed to create containerd client: %v", err)
		w.watcher.Close()
		return
	}

	defer client.Close()

	criConn, err := cri.Connection(ctx, w.cfg.Containerd.Address)
	if err != nil {
		logrus.Errorf("Failed to create CRI connection: %v", err)
		w.watcher.Close()
		return
	}

	defer criConn.Close()

	imageClient := runtimeapi.NewImageServiceClient(criConn)

	for w.processNextEventForImages(ctx, client, imageClient) {
	}
}

// processNextEventForImages retrieves a single event from the workqueue and processes it.
// It returns a boolean that is true if the workqueue is still open and this function should be called again.
func (w *watchqueue) processNextEventForImages(ctx context.Context, client *containerd.Client, imageClient runtimeapi.ImageServiceClient) bool {
	key, shutdown := w.workqueue.Get()

	if shutdown {
		return false
	}

	if err := w.processImageEvent(ctx, key, client, imageClient); err != nil {
		logrus.Errorf("Failed to process image event: %v", err)
	}

	return true
}

// processImageEvent processes a single item from the workqueue.
func (w *watchqueue) processImageEvent(ctx context.Context, key string, client *containerd.Client, imageClient runtimeapi.ImageServiceClient) error {
	defer w.workqueue.Done(key)

	// Watch is rooted at the parent dir of the images dir, but we only need to handle things within the images dir
	if !strings.HasPrefix(key, w.cfg.Images) {
		return nil
	}

	file, err := os.Stat(key)

	// if the file does not exists, we assume that the event was RENAMED or REMOVED
	if os.IsNotExist(err) {
		// if the whole images dir was removed, reset the fileinfo cache
		if key == w.cfg.Images {
			w.filesCache = make(map[string]*fileInfo)
			defer w.syncCache()
			return nil
		}

		if !isFileSupported(key) {
			return nil
		}

		delete(w.filesCache, key)
		defer w.syncCache()
		return nil
	} else if err != nil {
		return pkgerrors.Wrapf(err, "failed to get fileinfo for image event %s", key)
	}

	if file.IsDir() {
		// Add to watch and list+enqueue directory contents, as notify is not recursive
		if err := w.watcher.Add(key); err != nil {
			return pkgerrors.Wrapf(err, "failed to add watch of %s", key)
		}

		fileInfos, err := os.ReadDir(key)
		if err != nil {
			return pkgerrors.Wrapf(err, "unable to list contents of %s", key)
		}

		for _, fileInfo := range fileInfos {
			w.workqueue.Add(filepath.Join(key, fileInfo.Name()))
		}
		return nil
	}

	if !isFileSupported(key) {
		return nil
	}

	if lastFileState := w.filesCache[key]; lastFileState == nil || (file.Size() != lastFileState.Size && file.ModTime().After(lastFileState.ModTime.Time)) {
		start := time.Now()
		if err := preloadFile(ctx, w.cfg, client, imageClient, key); err != nil {
			return pkgerrors.Wrapf(err, "failed to import %s", key)
		}
		logrus.Infof("Imported images from %s in %s", key, time.Since(start))
		w.filesCache[key] = &fileInfo{Size: file.Size(), ModTime: metav1.NewTime(file.ModTime()), seen: true}
		defer w.syncCache()
	} else if lastFileState != nil && !lastFileState.seen {
		lastFileState.seen = true
		// no need to sync as the field is not serialized
	}

	return nil
}

// pruneCache removes entries for all files that have not been seen since the last restart,
// and syncs the cache to disk. This is done to ensure that the cache file does not grow without
// bounds by continuing to track files that do not exist.
func (w *watchqueue) pruneCache() {
	for path, fileState := range w.filesCache {
		if !fileState.seen {
			delete(w.filesCache, path)
		}
	}
	w.syncCache()
}

// syncCache writes the fileinfo cache to disk.
// if the cache file does not exist, this is a no-op. The file must be manually
// created by the user in order for the cache to be persisted across restarts.
func (w *watchqueue) syncCache() {
	filePath := filepath.Join(w.cfg.Images, ".cache.json")
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Errorf("Failed to truncate image import fileinfo cache: %v", err)
		}
		return
	}
	defer f.Close()

	b, err := json.Marshal(&w.filesCache)
	if err != nil {
		logrus.Errorf("Failed to marshal image import fileinfo cache: %v", err)
		return
	}

	if _, err := f.Write(b); err != nil {
		logrus.Errorf("Failed to write image import fileinfo cache: %v", err)
	}
}

// loadCaache reads the fileinfo cache from disk.
// It is not an error if this file exists or is empty.
func (w *watchqueue) loadCache() {
	filePath := filepath.Join(w.cfg.Images, ".cache.json")
	f, err := os.OpenFile(filePath, os.O_RDONLY, 0664)
	if err != nil {
		if !os.IsNotExist(err) {
			logrus.Errorf("Failed to open image import fileinfo cache: %v", err)
		}
		return
	}
	defer f.Close()

	b, err := io.ReadAll(f)
	if err != nil {
		logrus.Errorf("Failed to read image import fileinfo cache: %v", err)
		return
	}

	// 0 byte file is fine, but don't try to load it - allows users to simply
	// touch the file to enable it for use.
	if len(b) == 0 {
		return
	}

	if err := json.Unmarshal(b, &w.filesCache); err != nil {
		logrus.Errorf("Failed to unmarshal image import fileinfo cache: %v", err)
	}
}

// importAndWatchImages starts the image watcher and workqueue.
// This function block until the workqueue is empty, indicating that all images
// that currently exist have been imported.
func importAndWatchImages(ctx context.Context, cfg *config.Node) error {
	w, err := watchImages(ctx, cfg)
	if err != nil {
		return err
	}

	// Add images dir to workqueue; if it exists and contains images
	// they will be recursively listed and enqueued.
	w.workqueue.Add(cfg.Images)

	// wait for the workqueue to empty before returning
	for w.workqueue.Len() > 0 {
		time.Sleep(500 * time.Millisecond)
	}

	// prune unseen entries from last run once all existing files have been processed
	w.pruneCache()

	return nil
}

// watchImages starts a watcher on the parent of the images dir, and a workqueue to process events
// from the watch stream.
func watchImages(ctx context.Context, cfg *config.Node) (*watchqueue, error) {
	// watch the directory above the images dir, as it may not exist yet when the watch is started.
	watcher, err := createWatcher(filepath.Dir(cfg.Images))
	if err != nil {
		return nil, pkgerrors.Wrapf(err, "failed to create image import watcher for %s", filepath.Dir(cfg.Images))
	}

	w := &watchqueue{
		cfg:        cfg,
		watcher:    watcher,
		filesCache: make(map[string]*fileInfo),
		workqueue:  workqueue.TypedNewDelayingQueue[string](),
	}
	logrus.Debugf("Image import watcher created")

	w.loadCache()

	go func() {
		<-ctx.Done()
		w.watcher.Close()
	}()

	go w.runWorkerForImages(ctx)

	go func() {
		for {
			select {
			case event, ok := <-w.watcher.Events:
				if !ok {
					logrus.Info("Image import watcher event channel closed; retrying in 5 seconds")
					select {
					case <-time.After(time.Second * 5):
						w.watcher = mustCreateWatcher(filepath.Dir(cfg.Images))
					case <-ctx.Done():
						return
					}
				}

				// only enqueue event if it is for a path within the images dir - not the parent dir that we are watching
				if strings.HasPrefix(event.Name, cfg.Images) {
					w.workqueue.AddAfter(event.Name, time.Second*2)
				}

			case err, ok := <-w.watcher.Errors:
				if !ok {
					logrus.Info("Image import watcher error channel closed; retrying in 5 seconds")
					select {
					case <-time.After(time.Second * 5):
						w.watcher = mustCreateWatcher(filepath.Dir(cfg.Images))
					case <-ctx.Done():
						return
					}
				}
				logrus.Errorf("Image import watcher received an error: %v", err)
			}
		}
	}()

	return w, nil
}
