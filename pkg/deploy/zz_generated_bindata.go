// Code generated for package deploy by go-bindata DO NOT EDIT. (@generated)
// sources:
// manifests/ccm.yaml
// manifests/coredns.yaml
// manifests/local-storage.yaml
// manifests/metrics-server/aggregated-metrics-reader.yaml
// manifests/metrics-server/auth-delegator.yaml
// manifests/metrics-server/auth-reader.yaml
// manifests/metrics-server/metrics-apiservice.yaml
// manifests/metrics-server/metrics-server-deployment.yaml
// manifests/metrics-server/metrics-server-service.yaml
// manifests/metrics-server/resource-reader.yaml
// manifests/rolebindings.yaml
// manifests/traefik.yaml
//go:build !no_stage
// +build !no_stage

package deploy

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func bindataRead(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	clErr := gz.Close()

	if err != nil {
		return nil, fmt.Errorf("Read %q: %v", name, err)
	}
	if clErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _ccmYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xcc\x94\x4f\x8f\x13\x31\x0c\xc5\xef\xf9\x14\x51\x8f\x48\xe9\x0a\x71\x41\x73\x84\x03\xf7\x95\xe0\xee\x26\x8f\x6e\x68\x26\x8e\x62\xa7\xfc\xf9\xf4\x68\x66\xba\x62\xe8\xa8\x55\xa7\x80\xd8\x9b\x65\xc5\x3f\x3f\x3f\xcb\xa1\x12\x3f\xa1\x4a\xe4\xdc\xd9\xba\x23\xbf\xa5\xa6\x4f\x5c\xe3\x0f\xd2\xc8\x79\x7b\x78\x2b\xdb\xc8\x0f\xc7\xd7\xe6\x10\x73\xe8\xec\xfb\xd4\x44\x51\x1f\x39\xc1\xf4\x50\x0a\xa4\xd4\x19\x6b\x33\xf5\xe8\xec\xe1\x8d\x38\x9f\xb8\x05\xe7\x39\x6b\xe5\x94\x50\x5d\x4f\x99\xf6\xa8\xa6\xb6\x04\xe9\x8c\xb3\x54\xe2\x87\xca\xad\xc8\x50\xe8\xac\x67\xae\x21\xe6\x79\x3f\x63\x6d\x85\x70\xab\x1e\xa7\x47\x09\x24\x10\x63\xed\x11\x75\x77\xca\xed\xa1\x13\xa0\x82\x14\x63\xd8\x4a\x18\xc2\x45\x8f\xcd\x66\x89\xc4\x11\x59\xcf\x90\x33\x54\x21\xf5\x4f\xab\xa1\x99\xc3\xb9\xcc\xcd\xab\xcd\x8a\xda\x07\x51\xd2\x26\x63\x42\x50\x8f\xd1\xcf\x73\x33\xec\xa4\xef\x26\xf0\x33\x67\xaa\xe3\x70\xc1\xc7\x14\x65\x0a\xbe\xde\x85\x5e\x68\x5b\xeb\xdd\x89\x45\xde\x73\xbb\xb4\x99\xdb\x8c\xa4\x1e\x52\x68\x21\x6b\xb6\xdd\x61\xe6\x05\x8b\x4a\x91\x25\x2d\x10\x7a\xce\x82\x73\x45\xe3\x5e\x9d\x33\xf7\x5f\xd0\xbb\x98\x43\xcc\xfb\xd5\x87\xc4\x09\x8f\xf8\x3c\xbc\x7e\x1e\xe0\x4a\x67\x63\xed\xf2\x74\x6f\xea\x23\x6d\xf7\x05\x5e\xc7\x9b\x9d\x10\x1f\x05\xf5\xb6\x5a\xfb\x6b\x09\x9d\x3d\xb4\x1d\x9c\x7c\x17\x45\xff\x5f\x1c\x73\x03\xdf\x05\x24\xec\x49\xf9\xaf\x1a\x38\x4d\xd5\x9d\x35\x78\x29\xce\xfd\xa1\x65\xc8\x1a\xfd\x48\x76\x15\x14\xae\x89\xbb\xd3\xd2\xdf\xbc\xc4\x37\x45\x1e\x66\x73\x54\xe2\xf0\x19\x5c\x94\xf1\x4f\xfc\xfd\x19\x00\x00\xff\xff\x2f\x06\x3f\x61\x0a\x07\x00\x00")

func ccmYamlBytes() ([]byte, error) {
	return bindataRead(
		_ccmYaml,
		"ccm.yaml",
	)
}

func ccmYaml() (*asset, error) {
	bytes, err := ccmYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "ccm.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _corednsYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x57\x6f\x6f\xdb\xb6\x13\x7e\xef\x4f\x41\x08\xc8\x9b\x1f\x7e\x72\xe2\x65\xed\x52\xbe\x4b\x63\xb7\x0d\x96\xb8\x86\xed\x14\x28\x86\x21\xa0\xa9\xb3\xc5\x85\xe2\x71\x24\xe5\xc4\xeb\xf2\xdd\x07\xea\x9f\x45\x5b\x4e\x93\xac\xd3\x1b\x5b\x3a\xde\x73\xe4\xc3\xe3\x73\x47\xa6\xc5\x17\x30\x56\xa0\xa2\x64\x3d\xe8\xdd\x09\x95\x50\x32\x03\xb3\x16\x1c\xce\x39\xc7\x5c\xb9\x5e\x06\x8e\x25\xcc\x31\xda\x23\x44\xb1\x0c\x28\xe1\x68\x20\x51\xb6\x7a\xb7\x9a\x71\xa0\xe4\x2e\x5f\x40\x6c\x37\xd6\x41\xd6\x8b\xe3\xb8\xd7\x86\x36\x0b\xc6\xfb\x2c\x77\x29\x1a\xf1\x17\x73\x02\x55\xff\xee\xcc\xf6\x05\x1e\x37\x41\x2f\x64\x6e\x1d\x98\x29\x4a\x08\x22\x4a\xb6\x00\x69\xfd\x3f\x52\x84\x30\x0a\x1c\x14\xae\x0b\x44\x67\x9d\x61\x5a\x0b\xb5\x2a\x63\xc4\x09\x2c\x59\x2e\x9d\x6d\xa6\x5a\x4e\x88\xd6\x33\x36\xb9\x04\x4b\x7b\x31\x61\x5a\x7c\x34\x98\xeb\x02\x39\x26\x51\xd4\x23\xc4\x80\xc5\xdc\x70\xa8\xbe\x81\x4a\x34\x0a\x55\x80\xc5\xc4\x96\xa4\x94\x2f\x1a\x93\xf2\x4f\xb3\x7e\xff\xba\x06\xb3\xa8\x7c\xa5\xb0\xae\xf8\x73\xcf\x1c\x4f\xf7\xe3\x25\xc2\x72\x5c\x83\xd9\x54\x3c\x3c\x11\x5d\x8a\xef\xa2\xff\x2b\xb6\xdf\x0b\x95\x08\xb5\x0a\x48\x67\x4a\xa1\x2b\x3c\x2b\xe6\xbb\x20\x83\xcd\x60\xb9\xc3\x5c\x27\xcc\x01\x25\x91\x33\x39\x44\x3f\x7e\xef\x50\xc2\x14\x96\xc5\xfc\x2a\x36\x9f\x58\x6b\x8f\x90\xfd\xc4\x3a\x80\x6c\xf3\xc5\x1f\xc0\x5d\x91\x18\x9d\x47\xe0\xd5\x89\xbf\x25\x1c\xd5\x52\xac\xae\x99\x7e\xcd\x71\xaa\x87\x5f\xa0\x81\xa5\x90\x40\xc9\xdf\x05\xa7\x7d\xfa\xe6\x94\x7c\x2b\xfe\xfa\x07\x8c\x41\x63\x9b\xd7\x14\x98\x74\x69\xf3\x6a\x80\x25\x9b\xe6\x6d\xbb\x1d\xe4\xe8\xdb\xc5\xd5\xcd\x6c\x3e\x9a\xde\x0e\x3f\x5f\x9f\x5f\x8e\x1f\x8f\x88\x50\x31\x4b\x12\xd3\x67\x46\x33\x22\xf4\xdb\xf2\xcf\x36\x12\x29\x4e\x00\x11\xca\x02\xcf\x0d\xb4\xbe\x2f\x99\x94\x2e\x35\x98\xaf\xd2\x6e\x94\x66\xec\xe3\x76\xa2\x68\x9d\x25\xc7\xe0\xf8\x71\x45\xc5\xf1\x18\x13\xf8\x54\x7c\x6e\x07\x75\x4e\x92\xb7\x27\xad\x0f\x06\x24\xb2\x84\x0c\xde\xd8\xee\x29\x74\x04\xd3\x06\x33\x70\x29\xe4\x96\xd0\x77\x83\x37\xa7\x8d\x61\x89\xe6\x9e\x99\x84\xf4\xcb\x99\xf8\xe3\x28\xd7\x7d\x8e\x6a\xd9\x0c\xe1\x8c\xa7\x40\x4e\xb7\x33\x90\x88\xba\x17\x4e\xa6\x65\x63\xc9\x82\x49\xa6\x78\xc9\x4f\x39\x05\x91\x69\x34\x2e\x5c\x2c\xcf\xad\xc3\xec\xf8\x7f\x7d\xaf\x31\x60\xf6\x92\x88\x69\x6d\xb7\x47\x77\x08\x5a\xe2\x26\x83\xd7\x29\xf3\xce\xa1\x3c\xb3\x31\xd3\xba\x1a\x52\x3a\xee\x1e\xd5\x12\x38\xf2\xb9\x37\x1c\xcf\xa2\x9e\xd5\xc0\x69\xa1\x57\x6b\xe1\xe7\xf7\x49\x58\x87\x66\x73\x25\x32\xe1\x28\xf1\xdc\xf8\x83\xed\x60\xb5\x29\x63\xb8\x8d\x06\x4a\xa6\x28\xa5\x50\xab\x9b\x42\x22\x4a\x49\x69\x7f\xa1\x15\x6d\x19\x7b\xb8\x51\x6c\xcd\x84\x64\x0b\x9f\xe7\x03\x0f\x07\x12\xb8\x43\x53\x8e\xc9\xbc\xe4\x5d\xb5\xd6\xd0\xbd\x0a\x07\x99\x96\x0d\x70\x9b\xa8\x62\x6f\x02\xff\x43\x3c\xd4\x2b\x2d\xd3\x46\xa0\x11\x6e\x73\x21\x99\xb5\xe3\x92\x92\x92\xd2\x98\x97\x02\x13\x73\x23\x9c\xe0\x4c\x46\x95\x8b\x0d\x34\x64\xbc\xb3\x3f\x05\x35\x28\xc1\xb4\x65\xd6\x3f\x31\xb9\x83\x8d\x27\xbc\x82\x3b\x4f\x12\x54\xf6\xb3\x92\x9b\xa8\x95\xe4\xa8\xbd\x27\x1a\x4a\xa2\xd1\x83\xb0\xce\x46\x7b\x00\x0a\x13\x88\xbd\x68\xee\x48\x35\x47\xe5\x0c\xca\x58\x4b\xa6\xe0\x99\x98\x84\xc0\x72\x09\xdc\x51\x12\x8d\x71\xc6\x53\x48\x72\x09\xcf\x0f\x99\x31\xcf\xd0\x8f\x88\xe5\x23\xcc\x82\x84\xf0\xcf\x02\x1c\xdb\x09\x89\x96\x12\x29\x54\xfe\xd0\x70\xad\x51\xe2\x6a\x33\xd3\x5e\x08\x2f\x50\xf9\x2c\xf5\xf5\xb5\xcd\x7c\xc6\x1e\x66\x77\x70\x5f\xe6\x5d\xfd\xd4\x9e\xbf\xfa\x25\x86\x41\xbc\x72\xf9\xf3\xd1\x1a\x7d\x9f\x82\xba\x51\x96\x39\x61\x97\xa2\x4c\xe2\x21\x8e\xd1\xd5\x0b\x69\x0d\x2d\xb2\x70\x7f\x31\x07\xb2\xfc\xe9\x5c\x25\xc4\x6f\x2b\x13\x0a\x4c\xe3\x11\xef\x89\x42\xf9\x88\x8c\xad\x80\x92\xa3\x6f\xb3\xaf\xb3\xf9\xe8\xfa\x76\x38\xfa\x70\x7e\x73\x35\xbf\x9d\x8e\x3e\x5e\xce\xe6\xd3\xaf\x8f\x47\x86\x29\x9e\x82\x39\xce\x84\x2f\x29\x90\xc4\x15\x44\xfd\x4b\x07\xfd\x77\xfd\x9f\x43\xc0\x49\x2e\xe5\x04\xa5\xe0\x1b\x4a\x2e\x97\x63\x74\x13\x03\x16\x8a\xe2\x59\x3e\x41\x83\xd3\x70\xe0\x65\x63\x67\x8d\x19\x64\x68\x36\x94\x0c\x7e\x39\xb9\x16\x81\xda\xff\x99\x83\xdd\x1d\xcd\x75\x4e\xc9\xe0\xe4\x24\xeb\xc4\x08\x20\x98\x59\x59\x4a\x7e\x23\x51\xec\x65\x3d\xfa\x3f\x89\x02\x1d\xae\xcb\x6b\x44\x7e\x6f\x5c\xd6\x28\xf3\x0c\xae\xfd\x09\x0e\x32\xa5\x66\xd6\x57\xf5\xb8\x1c\xd4\x8a\x9f\xf9\xf1\x13\xe6\x52\x1a\x28\x7d\xb0\x16\x96\xf8\x33\x4d\x89\x6f\x96\xf6\x81\x8b\x92\x10\xbf\x10\xbf\xaa\x24\xdf\x0f\xe3\x6b\x50\xb0\x9c\x26\x79\x26\x68\x1c\x25\xad\xb2\x58\x57\x96\x70\xfa\xda\xa0\x43\x8e\x92\x92\x9b\xe1\xe4\xa5\x38\xb1\xe3\xba\x13\x6b\x7e\xf1\x04\x56\x50\xac\x6b\xb4\x0c\x9c\x11\xbc\x7b\x66\x6d\xb4\xa2\x4f\xf1\xf2\x8d\xca\xc1\x83\x6b\x67\x10\x93\x12\xef\x27\x46\xac\x85\x84\x15\x8c\x2c\x67\xb2\x90\x64\xea\x1b\x09\xdb\x66\x9d\x33\xcd\x16\x42\x0a\x27\x60\x27\x07\x59\x92\x84\x1f\x62\x32\x1e\xcd\x6f\xdf\x5f\x8e\x87\xb7\xb3\xd1\xf4\xcb\xe5\xc5\x28\x30\x27\x06\xf5\xae\x03\x93\xb2\x63\xe3\xa6\x88\xee\x83\x90\x50\x75\xac\xe1\x36\x4a\xb1\x06\x05\xd6\x4e\x0c\x2e\xa0\x8d\x97\x3a\xa7\x3f\x82\x0b\x43\xe8\x32\x5f\x76\xda\x42\x52\xa5\x03\x25\x67\x27\x67\x27\xc1\x67\xcb\x53\xf0\x24\x7f\x9a\xcf\x27\x2d\x83\x50\xc2\x09\x26\x87\x20\xd9\x66\x06\x1c\x55\x62\x69\xd8\x96\x69\x30\x02\x93\xc6\x36\x68\xdb\x9c\xc8\x00\x73\xb7\x35\xb6\x6c\x36\xe7\x1c\xac\x9d\xa7\x06\x6c\x8a\x32\x09\xad\x4b\x26\x64\x6e\xa0\x65\x3d\x0d\x9a\x5b\xf1\x62\x2a\xc2\x96\xb8\xc5\xc4\xe0\x6c\xf0\x6a\x26\x9e\x20\xe2\xa7\xff\x98\x87\x44\xd9\x5a\x81\x87\xe5\x65\xaa\x32\x94\x02\xf2\x02\x01\xe3\xf5\x75\x25\xe4\xad\xbb\x9e\x14\x54\x38\xc8\xec\x6e\x4a\x17\x4d\x41\xad\xaa\x3b\x65\xac\xdc\x82\x4e\x63\xe5\xd8\xdc\x01\x3a\x3d\xf7\xad\xcf\xd4\xce\xe7\x2c\x2d\xde\x13\x52\xdf\xb1\x78\x55\x60\xb2\x3a\x83\x07\x6f\x7a\xd5\xd5\xb1\xa3\x39\x6f\x15\xec\x83\xdd\xf9\xde\xcd\x7b\x7b\x5f\xf1\x0d\x47\x99\x9f\x91\xd7\xc2\xa8\xc3\x6c\xb9\x61\xfa\xe0\x0d\xfc\x19\xcd\x7e\xdd\xcb\x56\xbd\x6b\x0b\xe9\xb9\xd7\x82\xb0\x5b\xef\x8a\x59\xc5\xb8\x9c\xd0\xf6\xd5\x73\x3c\x7b\x3c\xea\xb5\x2a\x53\xbc\x53\x77\x74\xbb\xa0\xec\x96\x9f\xb8\xa3\xb8\x1c\x70\x28\xab\x42\xdc\x51\x3f\x74\x58\x66\x42\x97\x7f\x02\x00\x00\xff\xff\x83\xc0\x3b\xcb\x29\x13\x00\x00")

func corednsYamlBytes() ([]byte, error) {
	return bindataRead(
		_corednsYaml,
		"coredns.yaml",
	)
}

func corednsYaml() (*asset, error) {
	bytes, err := corednsYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "coredns.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _localStorageYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xec\x56\x5f\x6f\xdb\xb6\x16\x7f\xd7\xa7\x38\x57\xb7\x79\xb8\x17\xa5\x9d\xac\x03\x32\xb0\xd8\x83\x9b\x38\x69\x80\xc4\x36\x6c\x77\x43\x51\x14\x06\x2d\x1d\xdb\x6c\x28\x92\x20\x29\xb7\x6a\x96\xef\x3e\x90\x94\x1d\xc9\x71\x13\x07\xdb\xde\xa6\x17\x81\x87\xe7\xef\xef\xfc\x23\xd3\xfc\x37\x34\x96\x2b\x49\x61\x7d\x92\xdc\x72\x99\x53\x98\xa0\x59\xf3\x0c\x7b\x59\xa6\x4a\xe9\x92\x02\x1d\xcb\x99\x63\x34\x01\x90\xac\x40\x0a\x42\x65\x4c\x10\xcd\xdc\x8a\x68\xa3\xd6\xdc\xcb\xa3\x21\x36\xca\x11\x56\x0b\x46\x76\xab\x59\x86\x14\x6e\xcb\x39\x12\x5b\x59\x87\x45\x42\x08\x49\x9a\x96\xcd\x9c\x65\x1d\x56\xba\x95\x32\xfc\x3b\x73\x5c\xc9\xce\xed\x2f\xb6\xc3\x55\x77\xeb\xd3\x99\x28\xad\x43\x33\x56\x02\x0f\x77\xc8\x78\x6e\x53\x0a\xb4\x34\x21\xc0\x34\xbf\x34\xaa\xd4\x96\xc2\xa7\x34\xfd\x9c\x00\x18\xb4\xaa\x34\x19\x06\x8a\x54\x39\xda\xf4\x35\xa4\xda\xbb\x65\x1d\x4a\xb7\x56\xa2\x2c\x30\x13\x8c\x17\xe1\x26\x53\x72\xc1\x97\x05\xd3\x36\x88\xaf\xd1\xcc\x83\xe8\x12\x9d\xbf\x16\xdc\x86\xff\x57\xe6\xb2\x55\xfa\xf9\x79\x93\x28\x73\xad\xb8\x74\x7b\xcd\x46\xa2\xca\x77\x6c\xfd\xff\x20\xc5\x6b\xf4\x5a\x5b\x82\x99\x41\xe6\x30\x28\xdd\xef\x9f\x75\xca\xb0\x25\xd6\xd0\x3f\x56\x5a\xdf\x67\x82\x59\x8b\x07\x22\xf0\x97\x12\xfd\x8e\xcb\x9c\xcb\xe5\xe1\xf9\x9e\x73\x99\x27\x3e\xe9\x63\x5c\x78\xe6\x4d\x78\x4f\x18\x4e\x00\x1e\x17\xd8\x21\x65\x65\xcb\xf9\x17\xcc\x5c\xa8\xac\xbd\x6d\xf3\x4f\x35\x0b\xd3\xda\x3e\xc0\x75\x8e\x5a\xa8\xaa\xc0\x17\xf4\xe9\x8f\x4d\x59\x8d\x19\x0d\x69\x8f\xbc\xef\xb9\xcf\x79\x75\xcd\x0b\xee\x28\x1c\x27\x00\xd6\x19\xe6\x70\x59\x79\x2e\x00\x57\x69\xa4\x30\x56\x42\x70\xb9\xfc\xa0\x73\xe6\x30\xd0\x4d\x93\x12\x59\x01\x0a\xf6\xed\x83\x64\x6b\xc6\x05\x9b\x0b\xa4\x70\xe2\xd5\xa1\xc0\xcc\x29\x13\x79\x0a\x5f\x35\xd7\x6c\x8e\xc2\x6e\x84\x98\xd6\x4f\x84\xe1\xb0\xd0\x62\x6b\xa2\x19\xbf\xff\x44\x4b\xd3\x73\xba\x00\x36\xd1\xfb\x4f\x1b\xae\x0c\x77\xd5\x99\x2f\xf6\x41\x00\x33\x8d\x20\x11\x3f\x27\x48\x66\xb8\xe3\x19\x13\x69\xcd\x6f\x5b\xb9\x1f\xbc\x2c\xf1\x01\x4a\x25\xd0\x84\xc2\x6c\x78\x0c\x40\xe0\x16\x2b\x0a\xe9\x59\x6d\xaf\x97\xe7\x4a\xda\xa1\x14\x55\xda\xe0\x02\x50\xda\x4b\x2b\x43\x21\xed\x7f\xe3\xd6\xd9\x74\x8f\x92\xe0\xb9\x2f\xde\x8e\x4f\xba\x91\xe8\x30\xf4\x5e\xa6\xa4\x33\x4a\x10\x2d\x98\xc4\x17\xe8\x05\xc0\xc5\x02\x33\x47\x21\x1d\xa8\x49\xb6\xc2\xbc\x14\xf8\x12\xc3\x05\xf3\x2d\xf7\x77\x59\xf4\x61\x30\x2e\xd1\x6c\x11\x24\xcf\xf5\x41\xfc\x78\xc1\x96\x48\xe1\xe8\x6e\xf2\x71\x32\xed\xdf\xcc\xce\xfb\x17\xbd\x0f\xd7\xd3\xd9\xb8\x7f\x79\x35\x99\x8e\x3f\xde\x1f\x19\x26\xb3\x15\x9a\xee\x7e\x45\x74\x7d\xdc\x39\xee\xfc\xf4\xa6\xad\x70\x54\x0a\x31\x52\x82\x67\x15\x85\xab\xc5\x40\xb9\x91\x41\x8b\xdb\x84\x7b\x7f\x8b\x82\xc9\xfc\x21\xdd\xe4\x39\x47\x09\x58\xc7\x8c\x6b\x9c\x09\x89\x3b\xa9\x41\xea\xa2\xcb\xba\x91\x5a\xff\x3a\x5f\xac\x92\x5b\x8e\xb8\x5d\x6e\x7c\xed\xd9\xa6\xed\x08\x55\x94\x20\x91\xa9\x81\x7c\xe1\xf9\x47\xcc\xad\x68\xcb\xc0\x96\x03\xe5\xfa\xb1\xb2\xd1\xf0\x7c\x36\xe8\xdd\xf4\x27\xa3\xde\x59\xbf\xa1\x6c\xcd\x44\x89\x17\x46\x15\xb4\x95\xdb\x05\x47\x91\xd7\xa3\xfb\x11\x3d\xda\xde\xf4\x78\x67\x3b\xc1\x92\x66\x54\x2f\x08\x28\xd2\x6f\x98\x6e\x5b\x7b\x54\x30\x35\xbe\xbb\x53\xb8\xbd\x2c\x1f\xe6\xf1\x24\xd2\xc3\xdc\x78\x72\x22\xfb\xf5\x24\xa5\x72\xcd\x9e\x6f\x6e\xd8\x9d\x56\xe1\x96\xe4\xb8\x60\xa5\x70\x24\x5c\x53\x48\x9d\x29\x31\x4d\x9a\x75\x08\x75\x9d\x7a\x81\x86\xa5\x18\x7b\xbd\x4d\x6f\x54\x8e\x14\x7e\x67\xdc\x5d\x28\x73\xc1\x8d\x75\x67\x4a\xda\xb2\x40\x93\x98\xf8\xd4\xd9\x14\xed\x39\x0a\x74\x18\x22\xaf\x57\xe4\x06\xb2\x64\xe7\xd9\xf8\xe4\xe6\xd9\x16\xe8\x0f\x96\xce\x46\xb0\x51\xab\x14\xfe\x20\x01\x90\xbb\x3a\x37\x61\x82\xf8\x0a\xb8\x61\x3a\xa5\x9f\x6a\xea\xdd\x36\x73\xe1\x3e\xa5\xe9\xa6\x73\x47\xbd\xe9\xfb\xd9\xc5\x70\x3c\x1b\x0c\x07\xb3\xeb\xab\xc9\xb4\x7f\x3e\x1b\x0c\xcf\xfb\x93\xf4\xf5\x83\x8c\xf7\xce\xa6\xf4\x53\x7a\x74\xb7\x91\xbb\x1e\x9e\xf5\xae\x67\x93\xe9\x70\xdc\xbb\xec\x07\x2d\xf7\x47\xe1\xa1\xe3\xbf\xfb\xfa\x1f\xcf\xf7\x61\x7d\x39\xff\xb8\xa8\x9d\xfd\xef\x7f\xba\x73\x2e\xbb\x76\x15\x4e\x5f\x57\x5c\x20\x2c\xd1\x29\xed\x2c\xa4\x05\xb5\x54\xd3\x14\x94\x8e\xed\x9b\xab\x87\x39\xc0\x2c\xc2\x2b\xa5\x1d\x70\xd9\xaa\x45\xfd\xbf\xd6\x91\xcd\xad\x12\xa5\x0b\x38\xfc\xfa\x6a\x38\x9a\xf6\xc6\x97\x2d\x86\xb7\x6f\x5b\x47\xdb\x16\xb7\xfc\x3b\x5e\xc9\x77\x95\x43\x7b\x88\x74\xd1\x96\x5e\x2b\xe1\x2b\xe7\x39\x49\xb4\x2c\xab\xe3\x93\xb1\xdb\x8a\xdb\x9c\x1b\x20\x05\x1c\x9f\x9e\x9e\x02\xd1\xf0\xea\xae\x19\x48\x04\x35\x5b\x15\x2a\x87\xd3\xe3\x93\xdd\xdb\x6e\xa7\x13\xf6\x3c\x33\xb9\xfa\x2a\xff\x85\xfa\x49\xa8\x4d\x01\xc4\x2c\xf6\x00\xbc\x42\xa1\xd1\x8c\x54\xde\xa9\x58\x21\xb6\x28\xee\x74\xb1\x27\xc5\x46\x1f\xa9\x7c\xef\x8b\x2a\xf6\x76\xd4\x46\x74\xcd\xd4\x7c\x36\xfd\x78\x05\xef\x08\xc1\x8b\xd6\x6e\xc1\x8d\x51\x06\x73\x22\xf8\xdc\x30\x53\x91\x79\x69\xab\xb9\xfa\x46\x4f\x3a\x6f\x7e\xee\x9c\x1c\xb8\x77\xff\x0c\x00\x00\xff\xff\x7b\xe8\x43\xdf\xec\x0e\x00\x00")

func localStorageYamlBytes() ([]byte, error) {
	return bindataRead(
		_localStorageYaml,
		"local-storage.yaml",
	)
}

func localStorageYaml() (*asset, error) {
	bytes, err := localStorageYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "local-storage.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricsServerAggregatedMetricsReaderYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x9c\xcf\x31\x6b\xf4\x30\x0c\xc6\xf1\xdd\x9f\x42\x78\x7e\x93\x97\x6e\xc5\x6b\x87\xee\x1d\xba\x94\x1b\x94\xf8\x21\x27\xce\xb1\x83\x24\xe7\x68\x3f\x7d\xb9\x70\xdc\x58\x68\x27\x0d\x7f\x7e\x0f\xe8\x22\x35\x27\x7a\x29\xdd\x1c\xfa\xd6\x0a\x02\x6f\xf2\x0e\x35\x69\x35\x91\x4e\x3c\x8f\xdc\xfd\xdc\x54\xbe\xd8\xa5\xd5\xf1\xf2\x6c\xa3\xb4\xff\xfb\x53\x58\xe1\x9c\xd9\x39\x05\xa2\xca\x2b\x12\xd9\xa7\x39\xd6\xc4\xcb\xa2\x58\xd8\x91\x87\x15\xae\x32\xdb\xa0\xe0\x0c\x0d\x44\x85\x27\x14\xbb\x11\xfa\x61\xfd\xb1\x30\x78\x1b\x76\xc1\x35\x51\x74\xed\x88\xbf\x71\xc8\xe2\x7f\x71\x9c\x57\xa9\x0f\xa8\xbd\xc0\x52\x18\x88\x37\x79\xd5\xd6\x37\x4b\xf4\x11\xef\x7f\xdd\x7d\x3c\x05\x22\x85\xb5\xae\x33\x8e\xbe\xb5\x6c\xf1\x1f\xc5\xda\x32\xec\xc8\x3b\x74\x3a\xd2\x02\xbf\x95\x22\x76\xdc\x2b\xfb\x7c\x8e\xa7\xf0\x1d\x00\x00\xff\xff\xe5\x1d\x7a\x17\x89\x01\x00\x00")

func metricsServerAggregatedMetricsReaderYamlBytes() ([]byte, error) {
	return bindataRead(
		_metricsServerAggregatedMetricsReaderYaml,
		"metrics-server/aggregated-metrics-reader.yaml",
	)
}

func metricsServerAggregatedMetricsReaderYaml() (*asset, error) {
	bytes, err := metricsServerAggregatedMetricsReaderYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metrics-server/aggregated-metrics-reader.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricsServerAuthDelegatorYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x7c\x8e\x31\x4e\xc4\x40\x0c\x45\xfb\x39\xc5\x5c\x60\x82\xe8\x90\x3b\xa0\xa0\x5f\x24\x7a\x67\xf2\x59\x4c\x92\x71\x64\x7b\x22\xc1\xe9\xd1\x8a\x88\x06\xd8\xf6\x4b\xff\xbd\x57\x4a\x49\xbc\xc9\x0b\xcc\x45\x1b\x65\x1b\xb9\x0e\xdc\xe3\x4d\x4d\x3e\x39\x44\xdb\x30\xdf\xf9\x20\x7a\xb3\xdf\xa6\x59\xda\x44\xf9\x71\xe9\x1e\xb0\x93\x2e\x78\x90\x36\x49\x3b\xa7\x15\xc1\x13\x07\x53\xca\xb9\xf1\x0a\xca\x2b\xc2\xa4\x7a\x71\xd8\x0e\x23\xff\xf0\xc0\x4a\x17\x70\x99\xb0\xe0\xcc\xa1\x96\x4c\x17\x9c\xf0\x7a\x79\xf1\x26\x4f\xa6\x7d\xbb\x52\x90\x72\xfe\x15\xf0\xe3\xfb\x5b\xe0\x7d\x7c\x47\x0d\xa7\x54\x8e\xef\x33\x6c\x97\x8a\xfb\x5a\xb5\xb7\xf8\x27\xf7\x98\x7d\xe3\x0a\xca\x73\x1f\x51\xbe\xf9\xe9\x2b\x00\x00\xff\xff\xa5\xb5\x26\x22\x2f\x01\x00\x00")

func metricsServerAuthDelegatorYamlBytes() ([]byte, error) {
	return bindataRead(
		_metricsServerAuthDelegatorYaml,
		"metrics-server/auth-delegator.yaml",
	)
}

func metricsServerAuthDelegatorYaml() (*asset, error) {
	bytes, err := metricsServerAuthDelegatorYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metrics-server/auth-delegator.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricsServerAuthReaderYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x7c\x8f\x3d\x4e\x04\x31\x0c\x46\xfb\x9c\x22\x17\xf0\x22\x3a\x94\x0e\x1a\xfa\x45\xa2\xf7\x64\x3e\xc0\xcc\x8e\x13\xd9\xce\x08\x38\x3d\x1a\xb4\xfc\x34\x4b\x6f\xbf\xef\x3d\x22\x4a\xdc\xe5\x11\xe6\xd2\xb4\x64\x9b\xb8\x1e\x78\xc4\x4b\x33\xf9\xe0\x90\xa6\x87\xe5\xc6\x0f\xd2\xae\xb6\xeb\xb4\x88\xce\x25\x1f\xdb\x09\x77\xa2\xb3\xe8\x73\x5a\x11\x3c\x73\x70\x49\x39\x2b\xaf\x28\x79\x45\x98\x54\x27\x87\x6d\x30\xda\x51\x64\xe0\x19\x76\x3e\xf1\xce\x15\x25\x2f\x63\x02\xf9\xbb\x07\xd6\x64\xed\x84\x23\x9e\x76\x08\x77\xb9\xb7\x36\xfa\x3f\x26\x29\xe7\x5f\x91\x9f\x5d\xbc\x05\x74\x6f\x20\xee\xf2\x67\x1c\x1a\x52\xbf\xde\xbf\x35\x7c\x4c\xaf\xa8\xe1\x25\xd1\x19\xf4\x00\xdb\xa4\xe2\xb6\xd6\x36\x34\x2e\xa4\x5c\xd6\xff\x0c\x00\x00\xff\xff\x2a\x39\xe6\xe4\x44\x01\x00\x00")

func metricsServerAuthReaderYamlBytes() ([]byte, error) {
	return bindataRead(
		_metricsServerAuthReaderYaml,
		"metrics-server/auth-reader.yaml",
	)
}

func metricsServerAuthReaderYaml() (*asset, error) {
	bytes, err := metricsServerAuthReaderYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metrics-server/auth-reader.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricsServerMetricsApiserviceYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x44\x8e\x4d\x6a\xc4\x30\x0c\x85\xf7\x3e\x85\x2e\xe0\x74\xb2\x2b\xde\x75\x59\x68\x61\x20\x65\xf6\x1a\x8f\x3a\x88\xe0\x1f\x24\xd9\x90\xdb\x97\x30\x71\xba\x13\x7a\xef\xfb\x24\xef\xbd\xc3\xca\x37\x12\xe5\x92\x03\x60\x65\xa1\x27\xab\x09\x1a\x97\x3c\xad\xef\x3a\x71\x79\xeb\xb3\x5b\x39\x3f\x02\x7c\x5c\x3f\x17\x92\xce\x91\x5c\x22\xc3\x07\x1a\x06\x07\x90\x31\x51\x80\x3e\xdf\xc9\x70\x9e\x12\x99\x70\xd4\x03\x76\x5a\x29\xee\x25\x7d\x81\xfb\x38\x88\xa3\xe9\xf7\x88\xe4\x0c\xb4\x62\xa4\x00\x6b\xbb\x93\xd7\x4d\x8d\x92\x03\x78\x4a\x69\xf5\x44\x86\x1c\xa0\x8f\xdf\x8f\xf3\x0e\x80\xb3\x52\x6c\x42\xcb\xca\xf5\xe7\x6b\xb9\x91\xf0\xef\x16\xc0\xa4\xd1\x10\x5d\x85\x8b\xb0\x6d\xdf\x9c\x39\xb5\x14\x60\xbe\x5c\xfe\x65\x23\x7d\xad\xff\x02\x00\x00\xff\xff\x14\x74\xa9\x1b\x25\x01\x00\x00")

func metricsServerMetricsApiserviceYamlBytes() ([]byte, error) {
	return bindataRead(
		_metricsServerMetricsApiserviceYaml,
		"metrics-server/metrics-apiservice.yaml",
	)
}

func metricsServerMetricsApiserviceYaml() (*asset, error) {
	bytes, err := metricsServerMetricsApiserviceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metrics-server/metrics-apiservice.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricsServerMetricsServerDeploymentYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x55\x4f\x6f\xdb\xc6\x13\xbd\xeb\x53\x0c\xf4\x83\x8f\xb4\xa4\xfc\x90\xb4\x58\xc0\x07\xc3\x92\x93\x02\xb6\x2b\x88\x72\x01\x9f\x8c\xf5\x72\x24\x2d\xbc\xff\x3a\x33\x54\xcc\x1a\xfe\xee\xc5\x8a\x0e\x43\x3a\x76\x90\xa2\x0d\x8f\xf3\x66\xde\xbc\x7d\xbb\x33\x2c\x8a\x62\xa4\x93\xfd\x03\x89\x6d\x0c\x0a\xf6\xb3\xd1\xbd\x0d\x95\x82\x12\x69\x6f\x0d\x9e\x1a\x13\xeb\x20\x23\x8f\xa2\x2b\x2d\x5a\x8d\x00\x82\xf6\xa8\xc0\xa3\x90\x35\x5c\x30\xd2\x1e\xe9\x39\xcc\x49\x1b\x54\x70\x5f\xdf\x61\xc1\x0d\x0b\xfa\xd1\xcb\x0e\x3a\x25\x9e\x74\x6d\xe6\x98\x5c\x6c\x3c\xfe\xab\x16\x00\x4e\xdf\xa1\xe3\x5c\x09\x70\xff\x2b\x17\x3a\xa5\x6f\xca\x39\xa1\xc9\x19\x84\x7b\x9b\xa5\x7c\xb2\x2c\x91\x9a\x0b\xeb\xad\x28\x98\x8e\x00\x58\x48\x0b\x6e\x9b\x96\x47\x9a\x84\x0a\x56\xd1\x39\x1b\xb6\xd7\xa9\xd2\x82\x87\x38\xf5\x23\x6d\x2a\x80\xd7\x0f\xd7\x41\xef\xb5\x75\xfa\xce\xa1\x82\x59\xa6\x43\x87\x46\x22\xb5\x39\x5e\x8b\xd9\x5d\xf4\x74\xbe\xad\x14\x40\xd0\x27\xd7\xd1\xf7\x9d\xc9\xdf\x1b\xee\xe4\xcf\x0d\x1a\x7c\xaf\x05\xc0\x17\x43\xf2\x97\xc8\x46\xb2\xd2\x9c\x39\xcd\x7c\x75\xe0\x1f\xb7\xee\x16\x21\x56\x58\x18\xb2\x62\x8d\x76\xe3\xe7\x7c\x1e\x3c\x8f\xab\xb7\x05\x49\x74\x48\x5a\x6c\x0c\x3d\x55\x05\xdc\x63\xa3\x60\x7c\xf6\xcc\x7a\x5a\x55\x31\xf0\xef\xc1\x35\xe3\x2e\x07\x20\xa6\x5c\x19\x49\xc1\x78\xf1\x60\x59\x78\xfc\x0d\xc1\x41\x1b\x45\x87\xc7\xf9\x3d\x50\x40\x41\x3e\xb6\x71\x62\x62\x10\x8a\xae\x48\x4e\x07\xfc\x41\x4e\x00\xdc\x6c\xd0\x88\x82\xf1\x55\x2c\xcd\x0e\xab\xda\xe1\x8f\xb7\xf4\x9a\x05\xe9\xbf\xe8\xb5\x8f\xae\xf6\xd8\xd9\xf5\x3f\xf0\xd9\x63\xb0\x01\xc4\x27\xe0\x08\x9f\x11\x8c\x0e\xc0\x7a\x83\xae\x81\x9a\x11\x36\x14\x7d\xc1\x86\xf2\x1b\x03\xeb\xf5\x16\x19\x74\xa8\x26\x91\x80\x50\x57\x45\x0c\xae\x81\x6c\x8a\xb6\x01\x89\x47\x5f\x8e\xd4\xbe\x24\xf1\xa9\xa8\x2c\x75\xea\xd0\x27\x69\xe6\x96\x14\x3c\x3e\x3d\x07\xbf\xd6\xaa\x17\xc5\xaf\xde\x3a\xb4\x22\x14\x1c\x3d\x96\x37\xe5\x7a\x71\x79\x3b\x5f\x9c\x9f\x5e\x5f\xac\x6f\x57\x8b\x8f\xbf\x95\xeb\xd5\xcd\xd3\x11\xe9\x60\x76\x48\x13\x6f\x89\x22\x61\x55\x0c\x99\xd4\x7e\x7a\xfc\xe1\x78\xd6\x11\x6a\xda\x0e\x5e\x50\x51\x18\x24\xc9\xba\x4f\x26\xe2\xd3\x00\x61\x34\x35\x61\x91\x22\xc9\xc9\x6c\xfa\xee\xfd\x74\x80\xe6\x7b\x73\x28\x45\x22\xdc\x20\xe5\xce\xba\xaa\x08\x99\x8b\x3c\xf2\x7c\x72\xf4\xb8\x5c\x2d\xce\x17\xab\xd5\x62\x7e\x7b\x3a\x9f\xaf\x16\x65\x79\xbb\xbe\x59\x2e\xca\xa7\xa3\x57\x79\x6a\xc6\x76\x48\x58\xb4\xd4\x7c\x68\x3b\x48\x6c\x0f\x56\x10\x72\x74\x75\x1e\x85\x93\xd9\x7b\xee\x32\x72\xb8\x26\x83\xbd\xd3\xe5\xe0\x9f\x35\xb2\x0c\x62\x00\x26\xd5\x0a\x66\xd3\xa9\x1f\x44\x3d\xfa\x48\x8d\x82\x5f\xa6\x97\xb6\x03\xb2\x88\x81\x5f\xed\x6d\xed\x44\x12\xf7\xaa\xbb\x7b\x5d\x46\x92\xcc\xdd\x37\x2b\xaf\x85\x28\xd1\x44\xa7\x60\x7d\xb6\xec\x29\xd6\x95\x0d\xc8\xbc\xa4\x78\x87\x7d\x89\x99\xfe\x23\xca\x50\x75\xd2\xb2\x53\x30\xc9\x55\xcd\x5f\x43\xe4\xd0\xf4\xa5\x26\x00\x36\x3b\xcc\x6a\x3f\xad\xd7\xcb\xb2\x87\xd8\x60\xc5\x6a\x37\x47\xa7\x9b\x12\x4d\x0c\x15\xb7\x9b\xbb\x23\x44\xb2\xb1\xea\xa0\x77\x3d\x48\xac\xc7\x58\x4b\x87\xcd\x7a\x18\xd7\xc6\x20\xf3\x7a\x47\xc8\xbb\xe8\xaa\x21\xba\xd1\xd6\xd5\x84\x3d\xf4\xff\x1d\xea\xec\x1e\xff\xb1\x13\xb9\xe8\x27\x18\xf1\xe1\x3b\x4e\xcc\xa6\x3f\xdd\x8a\xc3\xd0\xe5\x5f\x48\x0c\x82\x0f\x32\x7c\xcd\xba\xca\xdb\x7d\x15\xa3\x9c\x5b\x87\xed\x9f\x45\x81\x50\x8d\xfd\xb4\x3a\x9c\xf2\x55\x0c\x39\xed\x75\xf0\x9a\x91\x0e\x13\xd0\x3f\x8e\x76\x2e\x7e\x5e\x92\xdd\x5b\x87\x5b\x5c\xb0\xd1\xee\xf0\xc3\x51\xb0\xd1\x8e\xbf\x72\xb4\x7b\xf5\x32\x2f\xd3\x57\x26\xe3\xe5\x12\x84\x76\xed\x2e\xdb\x2b\xcb\x1b\xe6\xef\x00\x00\x00\xff\xff\x93\xff\xf7\x9b\x2c\x09\x00\x00")

func metricsServerMetricsServerDeploymentYamlBytes() ([]byte, error) {
	return bindataRead(
		_metricsServerMetricsServerDeploymentYaml,
		"metrics-server/metrics-server-deployment.yaml",
	)
}

func metricsServerMetricsServerDeploymentYaml() (*asset, error) {
	bytes, err := metricsServerMetricsServerDeploymentYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metrics-server/metrics-server-deployment.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricsServerMetricsServerServiceYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x6c\x8e\x3f\x4b\x04\x31\x10\xc5\xfb\x7c\x8a\x61\xfb\x28\xe2\x15\x92\xd6\x5a\x38\x50\xec\xe7\x72\x0f\x0d\x97\x4d\xc2\xcc\xec\x82\xdf\x5e\x76\xf6\x9a\x83\xed\x92\x37\xef\xcf\x2f\xc6\x18\x78\x94\x6f\x88\x96\xde\x12\xad\x2f\xe1\x56\xda\x35\xd1\x27\x64\x2d\x19\x61\x86\xf1\x95\x8d\x53\x20\x6a\x3c\x23\xd1\x0c\x93\x92\x35\x2a\x64\x85\xdc\x65\x1d\x9c\x91\xe8\xb6\x5c\x10\xf5\x4f\x0d\x73\x20\xaa\x7c\x41\xd5\x2d\x49\x7e\x91\x06\x83\x3e\x95\xfe\xbc\x37\x4d\x1f\x0f\x55\xd3\x81\x31\xd7\x45\x0d\xe2\x8e\xb2\x2d\x4c\x26\x0b\xa6\xa0\x03\x79\x2b\x56\x54\x64\xeb\x72\x1f\x79\xd3\xc8\x63\x1c\x30\x8e\x2e\xe6\x24\xd1\x9f\x89\x4e\xa7\x57\x8f\xec\x24\xbf\x66\x43\xfd\x3f\xa4\x5b\xcf\xbd\x26\xfa\x7a\x3f\xbb\x62\x2c\x3f\xb0\xb3\xa7\x76\xdf\x7f\x00\x00\x00\xff\xff\x7e\x3b\x1f\x83\x35\x01\x00\x00")

func metricsServerMetricsServerServiceYamlBytes() ([]byte, error) {
	return bindataRead(
		_metricsServerMetricsServerServiceYaml,
		"metrics-server/metrics-server-service.yaml",
	)
}

func metricsServerMetricsServerServiceYaml() (*asset, error) {
	bytes, err := metricsServerMetricsServerServiceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metrics-server/metrics-server-service.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _metricsServerResourceReaderYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xa4\x90\x41\x4b\xc4\x30\x10\x85\xef\xf9\x15\xc3\xde\xd3\xc5\x9b\xe4\xa6\x1e\xbc\xaf\xe0\x3d\x4d\x9f\xbb\x63\xdb\xa4\xcc\x4c\x2a\xfa\xeb\xa5\xb6\x2a\xb8\xb0\x2c\x78\x4a\x98\xe4\x7d\x8f\xf9\xbc\xf7\x2e\x4e\xfc\x0c\x51\x2e\x39\x90\xb4\x31\x35\xb1\xda\xa9\x08\x7f\x44\xe3\x92\x9b\xfe\x56\x1b\x2e\xfb\xf9\xc6\xf5\x9c\xbb\x40\x0f\x43\x55\x83\x1c\xca\x00\x37\xc2\x62\x17\x2d\x06\x47\x94\xe3\x88\x40\xfa\xae\x86\x31\x8c\x30\xe1\xa4\x5e\x21\x33\xc4\x49\x1d\xa0\xc1\x79\x8a\x13\x3f\x4a\xa9\x93\x2e\x09\x4f\xbb\x9d\x23\x12\x68\xa9\x92\xb0\xcd\x72\xe9\xa0\xfb\x0d\xe0\x88\x66\x48\xbb\x3d\x1d\x61\xd7\x31\xa6\xd2\xe9\x2f\xec\x1c\xb2\x9c\x03\xeb\x7a\x79\x8b\x96\x4e\xee\x7f\x26\xee\x39\x77\x9c\x8f\xd7\x0b\x29\x03\x0e\x78\x59\xbe\x7d\xaf\x73\xa1\xd2\x11\x9d\xbb\xbf\x5c\xa0\xb5\x7d\x45\xb2\x2f\xe9\x6b\xf6\x09\x32\x73\xc2\x5d\x4a\xa5\x66\xfb\x89\xff\xc9\xad\x63\x9d\x62\x42\xa0\xbe\xb6\xf0\x2b\xdf\x7d\x06\x00\x00\xff\xff\xdb\x55\x9e\x61\x2a\x02\x00\x00")

func metricsServerResourceReaderYamlBytes() ([]byte, error) {
	return bindataRead(
		_metricsServerResourceReaderYaml,
		"metrics-server/resource-reader.yaml",
	)
}

func metricsServerResourceReaderYaml() (*asset, error) {
	bytes, err := metricsServerResourceReaderYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "metrics-server/resource-reader.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _rolebindingsYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xac\x92\x31\x6f\xe3\x30\x0c\x85\x77\xfd\x0a\x21\xbb\x72\x38\xdc\x72\xf0\xd8\x0e\xdd\x03\xb4\x3b\x6d\xb3\x09\x6b\x59\x14\x48\x2a\x41\xfb\xeb\x0b\xa7\x6e\x82\xa4\x76\x90\xb4\xdd\x24\x41\x7c\x1f\x1f\xf9\x20\xd3\x13\x8a\x12\xa7\xca\x4b\x0d\xcd\x12\x8a\x6d\x58\xe8\x0d\x8c\x38\x2d\xbb\xff\xba\x24\xfe\xb3\xfd\xeb\x3a\x4a\x6d\xe5\xef\x63\x51\x43\x59\x71\xc4\x3b\x4a\x2d\xa5\xb5\xeb\xd1\xa0\x05\x83\xca\x79\x9f\xa0\xc7\xca\x77\xa5\xc6\x00\x99\x14\x65\x8b\x12\x86\x6b\x44\x0b\xd0\xf6\x94\x9c\x70\xc4\x15\x3e\x0f\xbf\x21\xd3\x83\x70\xc9\x17\xc8\xce\xfb\x2f\xe0\x03\x47\x5f\xd5\xb0\xaf\x0e\xfa\x99\x46\x86\x96\xfa\x05\x1b\xd3\xca\x85\x9b\x20\x8f\x8a\x32\xe3\xc2\xb9\x10\x82\xfb\xfe\xb4\x26\xc6\xf4\xd9\xfe\x3f\x0d\x0d\x27\x13\x8e\x11\xc5\x49\x89\x78\xd2\xb8\x0e\x15\xc1\x2f\x16\xce\x7b\x41\xe5\x22\x0d\x8e\x6f\x89\x5b\x54\xe7\xfd\x16\xa5\x1e\x9f\xd6\x68\x57\xd6\x42\x8f\x9a\xa1\x39\x17\x88\xa4\xb6\x3f\xec\xc0\x9a\xcd\x84\x56\x42\xdb\xb1\x74\x94\xd6\xa3\xdf\x29\xf1\x8f\x3f\x99\x23\x35\x74\x33\x61\x42\x10\x53\x9b\x99\x92\xe9\xfe\x96\xb9\x9d\xd3\x1c\xfc\x1f\xb5\x7f\xb8\xb4\xf9\x88\xcf\xec\xee\xf7\xb3\x7d\x0a\x38\x06\x7b\xf0\x78\x1d\xe3\x2c\xdc\x97\x01\xef\x01\x00\x00\xff\xff\x46\xd3\x6d\x9d\x0f\x04\x00\x00")

func rolebindingsYamlBytes() ([]byte, error) {
	return bindataRead(
		_rolebindingsYaml,
		"rolebindings.yaml",
	)
}

func rolebindingsYaml() (*asset, error) {
	bytes, err := rolebindingsYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "rolebindings.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _traefikYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x91\x4f\x6f\xd3\x40\x10\xc5\xef\xfe\x14\x23\x4b\x39\xa1\x75\xda\xc2\xa1\xf8\x16\x52\x17\x2a\xa0\x54\x71\x0a\xea\x29\x9a\xac\x27\xf1\x2a\xeb\xdd\xd5\xec\x38\x22\x94\x7e\x77\xb4\x71\xfa\x4f\xaa\x04\x42\x70\x5b\xed\xcc\xfc\xde\xcc\x7b\x4a\xa9\x0c\x83\xf9\x4a\x1c\x8d\x77\x25\xb4\x64\xbb\x42\xa3\x88\xa5\xc2\xf8\xf1\xf6\x38\xdb\x18\xd7\x94\xf0\x81\x6c\x37\x6d\x91\x25\xeb\x48\xb0\x41\xc1\x32\x03\x70\xd8\x51\x09\xc2\x48\x2b\xb3\x51\x9a\x9b\xc3\x5f\x0c\xa8\xa9\x84\x4d\xbf\x24\x15\x77\x51\xa8\xcb\x62\x20\x9d\x46\x74\x82\x94\xd0\x8a\x84\x58\x8e\xc7\xa3\xdb\x8f\xd7\xef\xaa\xd9\x65\x35\xaf\xea\xc5\xe4\xea\xe2\x6e\x34\x8e\x82\x62\xf4\x78\xdf\x18\xc7\x4f\xe0\xea\xe4\xa8\x78\x5d\x1c\xbf\xea\xc3\xfe\x71\x54\xc8\xfa\x47\xf6\x0f\x0f\xf8\x7f\xcb\xbf\xb4\x38\x40\x24\x49\x50\x80\xb5\xf5\x4b\xb4\xc5\x20\x76\x46\x2b\xec\xad\xcc\x68\x6d\xa2\xf0\xae\x84\x7c\x74\x5b\xdf\xd4\xf3\xea\xf3\xe2\xac\x3a\x9f\x5c\x7f\x9a\x2f\x66\xd5\xfb\x8b\x7a\x3e\xbb\x59\xcc\x26\xdf\xee\x46\x79\x06\xb0\x45\xdb\x53\x9c\x7a\x27\xe4\xa4\x84\x9f\x6a\xcf\x0d\xbe\x99\x38\xe7\xd3\x4a\xde\xc5\x41\x0b\x20\xb0\xef\x48\x5a\xea\x63\x32\x28\xf8\x74\x51\x7e\x7a\x74\x7a\x92\xbf\xd8\x10\x35\x63\xa0\x12\x72\xe1\x9e\x86\x96\xc0\x7e\x6b\x1a\xe2\x07\x64\xf2\x8a\x1d\x09\xc5\x0b\xb7\x66\x8a\x0f\x05\x80\xd0\x2f\xad\x89\x2d\x35\x35\xf1\xd6\x68\x7a\xac\x00\x90\xc3\xa5\xa5\x26\x05\xd0\xd3\x81\x6c\x3c\x1b\xd9\x4d\x2d\xc6\x78\xb9\x0f\x27\x1f\x6c\x51\xda\xf6\x51\x88\x95\x66\x23\x46\xa3\x1d\x56\x31\x1d\xae\x1f\x98\x43\x9a\x39\xa3\xd3\x2d\xf1\xb8\x33\xcc\x9e\xa9\x51\xd6\x2c\x19\x79\xa7\x0e\x71\xdc\xdf\x29\xb8\x2e\x21\x3f\x29\xde\x16\x6f\x86\x2f\xf1\x96\xf8\xa9\x59\x0a\x36\x94\x12\x98\x1e\x34\x27\x4d\xe3\x5d\xfc\xe2\xec\xee\x9e\xe1\x43\x9a\xf0\x5c\x42\x5e\x7d\x37\x51\x62\xfe\x6c\xd0\xf9\x86\x14\x7b\x4b\xc5\xa3\x45\xc9\x54\xed\x9d\xb0\xb7\x2a\x58\x74\xf4\x1b\x16\x00\xad\x56\xa4\x53\x4a\x97\xbe\xd6\x2d\x35\xbd\xa5\x3f\x93\xe9\x30\x59\xf6\xf7\xfc\xf8\x3c\x33\x13\xce\xb1\x33\x76\x77\xe5\xad\xd1\x49\xf7\x8a\x69\x45\x7c\xd6\xa3\xad\x05\xf5\x26\xcf\x7e\x05\x00\x00\xff\xff\x38\x7c\x74\xdd\x4f\x04\x00\x00")

func traefikYamlBytes() ([]byte, error) {
	return bindataRead(
		_traefikYaml,
		"traefik.yaml",
	)
}

func traefikYaml() (*asset, error) {
	bytes, err := traefikYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "traefik.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"ccm.yaml":           ccmYaml,
	"coredns.yaml":       corednsYaml,
	"local-storage.yaml": localStorageYaml,
	"metrics-server/aggregated-metrics-reader.yaml": metricsServerAggregatedMetricsReaderYaml,
	"metrics-server/auth-delegator.yaml":            metricsServerAuthDelegatorYaml,
	"metrics-server/auth-reader.yaml":               metricsServerAuthReaderYaml,
	"metrics-server/metrics-apiservice.yaml":        metricsServerMetricsApiserviceYaml,
	"metrics-server/metrics-server-deployment.yaml": metricsServerMetricsServerDeploymentYaml,
	"metrics-server/metrics-server-service.yaml":    metricsServerMetricsServerServiceYaml,
	"metrics-server/resource-reader.yaml":           metricsServerResourceReaderYaml,
	"rolebindings.yaml":                             rolebindingsYaml,
	"traefik.yaml":                                  traefikYaml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//
//	data/
//	  foo.txt
//	  img/
//	    a.png
//	    b.png
//
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"ccm.yaml":           &bintree{ccmYaml, map[string]*bintree{}},
	"coredns.yaml":       &bintree{corednsYaml, map[string]*bintree{}},
	"local-storage.yaml": &bintree{localStorageYaml, map[string]*bintree{}},
	"metrics-server": &bintree{nil, map[string]*bintree{
		"aggregated-metrics-reader.yaml": &bintree{metricsServerAggregatedMetricsReaderYaml, map[string]*bintree{}},
		"auth-delegator.yaml":            &bintree{metricsServerAuthDelegatorYaml, map[string]*bintree{}},
		"auth-reader.yaml":               &bintree{metricsServerAuthReaderYaml, map[string]*bintree{}},
		"metrics-apiservice.yaml":        &bintree{metricsServerMetricsApiserviceYaml, map[string]*bintree{}},
		"metrics-server-deployment.yaml": &bintree{metricsServerMetricsServerDeploymentYaml, map[string]*bintree{}},
		"metrics-server-service.yaml":    &bintree{metricsServerMetricsServerServiceYaml, map[string]*bintree{}},
		"resource-reader.yaml":           &bintree{metricsServerResourceReaderYaml, map[string]*bintree{}},
	}},
	"rolebindings.yaml": &bintree{rolebindingsYaml, map[string]*bintree{}},
	"traefik.yaml":      &bintree{traefikYaml, map[string]*bintree{}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
