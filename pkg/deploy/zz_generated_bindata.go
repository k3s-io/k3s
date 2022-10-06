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

var _ccmYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xcc\x94\x4f\x8f\x13\x31\x0c\xc5\xef\xf9\x14\x51\x8f\x48\xe9\x0a\x71\x41\x73\x84\x03\xf7\x95\xe0\xee\x26\x8f\x6e\x68\x26\x8e\x62\xa7\xfc\xf9\xf4\x68\x66\x76\xc5\x30\xa3\xae\x3a\x05\xc4\xde\x2c\x2b\xfe\xf9\xf9\x39\x09\x95\xf8\x09\x55\x22\xe7\xce\xd6\x03\xf9\x3d\x35\x7d\xe0\x1a\x7f\x90\x46\xce\xfb\xd3\x5b\xd9\x47\xbe\x3b\xbf\x36\xa7\x98\x43\x67\xdf\xa7\x26\x8a\x7a\xcf\x09\xa6\x87\x52\x20\xa5\xce\x58\x9b\xa9\x47\x67\x4f\x6f\xc4\xf9\xc4\x2d\x38\xcf\x59\x2b\xa7\x84\xea\x7a\xca\x74\x44\x35\xb5\x25\x48\x67\x9c\xa5\x12\x3f\x54\x6e\x45\x86\x42\x67\x3d\x73\x0d\x31\xcf\xfb\x19\x6b\x2b\x84\x5b\xf5\x78\x3c\x94\x40\x02\x31\xd6\x9e\x51\x0f\x8f\xb9\x23\x74\x02\x54\x90\x62\x0c\x5b\x09\x43\xb8\xea\xb1\xdb\xad\x91\x38\x23\xeb\x02\x39\x43\x15\x52\xff\xb0\x19\x9a\x39\x2c\x65\xee\x5e\xed\x36\xd4\xde\x89\x92\x36\x19\x13\x82\x7a\x8e\x7e\x9e\x9b\x61\x27\x7d\x57\x81\x9f\x38\x53\x1d\x87\x0b\x3e\xa6\x28\x53\xf0\x75\x33\x9a\xbc\xe7\x76\xc9\xcd\xeb\x86\xa7\x1e\x52\xc8\x2f\xdd\x9b\x6d\x64\xd0\xb9\x62\x51\x29\xb2\xa6\x05\x42\xcf\x59\xb0\x54\x34\xee\xc2\x39\x73\xfb\xad\x7f\x17\x73\x88\xf9\xb8\xf9\xf2\x73\xc2\x3d\x3e\x0f\xa7\x9f\x06\x78\xa6\xb3\xb1\x76\xfd\xdc\xae\xea\x23\xed\xf0\x05\x5e\xc7\x77\x36\x21\x3e\x0a\xea\x75\xb5\xf6\xd7\x12\x3a\x7b\x6a\x07\x38\xf9\x2e\x8a\xfe\xbf\x38\xe6\x06\xbe\x0b\x48\x38\x92\xf2\x5f\x35\x70\x9a\xaa\x5b\x34\x78\x29\xce\xfd\xa1\x65\xc8\x1a\xfd\x48\x76\x15\x14\x9e\x13\x77\xa3\xa5\xbf\x79\x89\x6f\x8a\x3c\xcc\xe6\xa8\xc4\xe1\x33\xb8\x28\xe3\x9f\xf8\xfb\x33\x00\x00\xff\xff\x4c\x59\x10\xd5\xbe\x06\x00\x00")

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

var _corednsYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x57\x51\x6f\xdb\x38\x12\x7e\xf7\xaf\x20\x74\xc8\xcb\xe1\xe4\xc4\x17\xb4\x97\xea\x2d\x8d\xdd\x36\xb8\xc6\x35\x6c\xa7\x40\xb1\x58\x04\x63\x6a\x6c\x71\x43\x71\xb8\x24\xe5\xc4\xdb\xcd\x7f\x5f\x50\x92\x65\xd1\x56\xd2\x24\xdb\xd5\x8b\x2d\x91\xf3\xcd\xf0\xe3\xf0\x9b\x21\x68\xf1\x15\x8d\x15\xa4\x12\xb6\x1e\xf4\x6e\x85\x4a\x13\x36\x43\xb3\x16\x1c\xcf\x39\xa7\x42\xb9\x5e\x8e\x0e\x52\x70\x90\xf4\x18\x53\x90\x63\xc2\x38\x19\x4c\x95\xad\xdf\xad\x06\x8e\x09\xbb\x2d\x16\x18\xdb\x8d\x75\x98\xf7\xe2\x38\xee\xb5\xa1\xcd\x02\x78\x1f\x0a\x97\x91\x11\x7f\x80\x13\xa4\xfa\xb7\x67\xb6\x2f\xe8\xb8\x71\x7a\x21\x0b\xeb\xd0\x4c\x49\x62\xe0\x51\xc2\x02\xa5\xf5\xff\x58\xe9\xc2\x28\x74\x58\x9a\x2e\x88\x9c\x75\x06\xb4\x16\x6a\x55\xf9\x88\x53\x5c\x42\x21\x9d\x6d\x42\xad\x02\x4a\xb6\x11\x9b\x42\xa2\x4d\x7a\x31\x03\x2d\x3e\x1a\x2a\x74\x89\x1c\xb3\x28\xea\x31\x66\xd0\x52\x61\x38\xd6\xdf\x50\xa5\x9a\x84\x2a\xc1\x62\x66\x2b\x52\xaa\x17\x4d\x69\xf5\xa7\x59\xbf\x7f\x5d\xa3\x59\xd4\xb6\x52\x58\x57\xfe\xb9\x03\xc7\xb3\x43\x7f\xa9\xb0\x9c\xd6\x68\x36\x35\x0f\x4f\x78\x97\xe2\x87\xe8\x7f\x8b\xed\xf7\x42\xa5\x42\xad\x02\xd2\x41\x29\x72\xa5\x65\xcd\x7c\x17\x64\xb0\x19\x50\x38\x2a\x74\x0a\x0e\x13\x16\x39\x53\x60\xf4\xf3\xf7\x8e\x24\x4e\x71\x59\xc6\x57\xb3\xf9\xc4\x5a\x7b\x8c\x1d\x26\xd6\x23\xc8\xb6\x58\xfc\x86\xdc\x95\x89\xd1\x79\x04\x5e\x9d\xf8\x3b\xc2\x49\x2d\xc5\xea\x0a\xf4\x6b\x8e\xd3\x76\xfa\x05\x19\x5c\x0a\x89\x09\xfb\xb3\xe4\xb4\x9f\xbc\x39\x65\xdf\xcb\xbf\xfe\x41\x63\xc8\xd8\xe6\x35\x43\x90\x2e\x6b\x5e\x0d\x42\xba\x69\xde\x76\xdb\xc1\x8e\xbe\x5f\x7c\xbe\x9e\xcd\x47\xd3\x9b\xe1\x97\xab\xf3\xcb\xf1\xc3\x11\x13\x2a\x86\x34\x35\x7d\x30\x1a\x98\xd0\x6f\xab\x3f\x3b\x4f\xac\x3c\x01\x4c\x28\x8b\xbc\x30\xd8\xfa\xbe\x04\x29\x5d\x66\xa8\x58\x65\xdd\x28\xcd\xdc\x87\x5d\xa0\x64\x9d\x65\xc7\xe8\xf8\x71\x4d\xc5\xf1\x98\x52\xfc\x54\x7e\x6e\x3b\x75\x4e\xb2\xb7\x27\xad\x0f\x06\x25\x41\xca\x06\x6f\x6c\x77\x08\x1d\xce\xb4\xa1\x1c\x5d\x86\x85\x65\xc9\xbb\xc1\x9b\xd3\x66\x60\x49\xe6\x0e\x4c\xca\xfa\x55\x24\xfe\x38\xca\x75\x9f\x93\x5a\x36\x53\x38\xf0\x0c\xd9\xe9\x2e\x02\x49\xa4\x7b\x61\x30\xad\x31\x48\x17\x20\x41\xf1\x8a\x9f\x2a\x04\x91\x6b\x32\x2e\x5c\x2c\x2f\xac\xa3\xfc\xf8\xdf\x7d\xaf\x31\x68\x0e\x92\x08\xb4\xb6\xbb\xa3\x3b\x44\x2d\x69\x93\xe3\xeb\x94\x79\xef\x50\x9e\xd9\x18\xb4\xae\xa7\x54\x86\xfb\x47\xb5\x02\x8e\x7c\xee\x0d\xc7\xb3\xa8\x67\x35\x72\x6f\xfd\x2f\x83\x5a\x0a\x0e\x36\x61\x83\x1e\x63\xfe\x34\x3b\x5c\x6d\x2a\x60\xb7\xd1\x98\xb0\x29\x49\x29\xd4\xea\xba\xd4\x85\x4a\x47\xda\x5f\x92\x9a\xab\x1c\xee\xaf\x15\xac\x41\x48\x58\xf8\xe4\x2e\xe1\x50\x22\x77\x64\xaa\x39\xb9\xd7\xb9\xcf\xad\xc0\xbb\x43\x77\x98\x6b\xd9\x00\xb7\xd9\x29\x37\x24\xb0\x7f\x6c\xf1\xdb\xe5\x55\xb9\x22\xc8\x08\xb7\xb9\x90\x60\xed\xb8\xe2\xa1\xe2\x31\xe6\x95\xaa\xc4\xdc\x08\x27\x38\xc8\xa8\x36\xb1\x81\x70\x8c\xf7\x36\xa5\xa4\x86\x24\x9a\xb6\xb6\xfa\x27\x66\xb7\xb8\xf1\x2c\xd7\x70\xe7\x69\x4a\xca\x7e\x51\x72\x13\xb5\x32\x9b\xb4\xb7\x24\x93\xb0\x68\x74\x2f\xac\xb3\xd1\x01\x80\xa2\x14\x63\xaf\x94\x7b\xfa\xcc\x49\x39\x43\x32\xd6\x12\x14\x3e\x13\x93\x31\x5c\x2e\x91\xbb\x84\x45\x63\x9a\xf1\x0c\xd3\x42\xe2\xf3\x5d\xe6\xe0\x19\xfa\x19\xbe\xbc\x87\x59\x90\x10\xfe\x59\xa0\x83\x3d\x97\x64\x13\x26\x85\x2a\xee\x1b\xae\x35\x49\x5a\x6d\x66\xda\xab\xdf\x05\x29\x9f\xa5\xbe\xa8\xb6\x99\xcf\xe1\x7e\x76\x8b\x77\x55\xde\x6d\x9f\xad\xe5\xff\xfd\x12\x43\x27\x5e\xae\xfc\xa1\x68\xcd\xbe\xcb\x50\x5d\x2b\x0b\x4e\xd8\xa5\xa8\x92\x78\x48\x63\x72\xdb\x85\xb4\xa6\x96\x59\x78\xb8\x98\x47\xb2\xfc\xe9\x5c\x65\xcc\x6f\x2b\x08\x85\xa6\xb1\x88\x0f\x94\xa0\x7a\x44\x0e\x2b\x4c\xd8\xd1\xf7\xd9\xb7\xd9\x7c\x74\x75\x33\x1c\x7d\x38\xbf\xfe\x3c\xbf\x99\x8e\x3e\x5e\xce\xe6\xd3\x6f\x0f\x47\x06\x14\xcf\xd0\x1c\xe7\xc2\xd7\x11\x4c\xe3\x1a\x62\xfb\x9b\x0c\xfa\xef\xfa\x83\x10\x70\x52\x48\x39\x21\x29\xf8\x26\x61\x97\xcb\x31\xb9\x89\x41\x8b\x65\xc5\xac\x9e\xa0\xab\x69\x38\x10\xb9\x70\x7b\x6b\xcc\x31\x27\xb3\x49\xd8\xe0\x7f\x27\x57\x22\x90\xf8\xdf\x0b\xb4\xfb\xb3\xb9\x2e\x12\x36\x38\x39\xc9\x3b\x31\x02\x08\x30\x2b\x9b\xb0\x5f\x58\x14\x7b\x2d\x8f\xfe\xc3\xa2\x40\x7c\xb7\x35\x35\x62\xbf\x36\x26\x6b\x92\x45\x8e\x57\xfe\x04\x07\x99\xb2\x65\xd6\x97\xf2\xb8\x9a\xd4\xf2\x9f\xfb\xf9\x13\x70\x59\x12\xc8\x7b\xb0\x16\x48\xfd\x99\x4e\x98\xef\x90\x0e\x81\xcb\x3a\x10\xbf\x10\xbf\x2e\x1f\x3f\x76\xe3\x0b\x4f\xb0\x9c\x26\x79\x26\x64\x5c\xc2\x5a\xb5\x70\x5b\x4e\xc2\xf0\xb5\x21\x47\x9c\x64\xc2\xae\x87\x93\x97\xe2\xc4\x8e\xeb\x4e\xac\xf9\xc5\x13\x58\x41\x85\xde\xa2\xe5\xe8\x8c\xe0\xdd\x91\xb5\xd1\xca\xe6\xc4\xcb\x37\x29\x87\xf7\xae\x9d\x41\x20\x25\xdd\x4d\x8c\x58\x0b\x89\x2b\x1c\x59\x0e\xb2\x94\xe4\xc4\x77\x0f\xb6\xcd\x3a\x07\x0d\x0b\x21\x85\x13\xb8\x97\x83\x90\xa6\xe1\x87\x98\x8d\x47\xf3\x9b\xf7\x97\xe3\xe1\xcd\x6c\x34\xfd\x7a\x79\x31\x0a\x86\x53\x43\x7a\xdf\x00\xa4\xec\xd8\xb8\x29\x91\xfb\x20\x24\xd6\x6d\x6a\xb8\x8d\x52\xac\x51\xa1\xb5\x13\x43\x0b\x6c\xe3\x65\xce\xe9\x8f\xe8\x42\x17\xba\xca\x97\xbd\x5e\x90\xd5\xe9\x90\xb0\xb3\x93\xb3\x93\xe0\xb3\xe5\x19\x7a\x92\x3f\xcd\xe7\x93\xd6\x80\x50\xc2\x09\x90\x43\x94\xb0\x99\x21\x27\x95\xda\x24\xec\xc5\x34\x1a\x41\x69\x33\x36\x68\x8f\x39\x91\x23\x15\x6e\x37\xd8\x1a\xb3\x05\xe7\x68\xed\x3c\x33\x68\x33\x92\x69\x38\xba\x04\x21\x0b\x83\xad\xd1\xd3\xa0\xa3\x15\x2f\xa6\x22\xec\x83\x5b\x4c\x0c\xce\x06\xaf\x66\xe2\x09\x22\xfe\xfb\x0f\xf3\x90\x2a\xbb\x55\xe0\x61\x75\x83\xaa\x07\x2a\x01\x79\x81\x80\xf1\xed\x1d\x25\xe4\xad\xbb\x9e\x94\x54\x38\xcc\xed\x7e\x4a\x97\x4d\xc1\x56\x55\xf7\xca\x58\xb5\x05\x9d\x83\xb5\x61\xd3\xf8\x77\x5a\x1e\x8e\x3e\x53\x3b\x9f\xb3\xb4\xf8\x40\x48\x7d\xc7\xe2\x55\x01\x64\x7d\x06\x1f\xbd\xde\xd5\xf7\xc5\x8e\x8e\xbc\x55\xb0\x1f\x6d\xc9\x0f\xae\xdb\xbb\x4b\x8a\x6f\x38\xaa\xfc\x8c\xbc\x16\x46\x1d\xc3\x96\x1b\xd0\x8f\x5e\xbb\x9f\xd1\xe1\x6f\x7b\xd9\xba\x77\x6d\x21\x3d\xf7\x2e\x10\x76\xeb\x5d\x3e\x6b\x1f\x97\x93\xa4\x7d\xdf\x1c\xcf\x1e\x8e\x7a\xad\xca\x14\xef\xd5\x1d\xdd\x2e\x28\xfb\xe5\x27\xee\x28\x2e\x8f\x18\x54\x55\x21\xee\xa8\x1f\x3a\x2c\x33\xa1\xc9\x5f\x01\x00\x00\xff\xff\x1d\x00\xf3\xa5\x1e\x13\x00\x00")

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

var _localStorageYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xec\x56\x5d\x6f\xdb\x36\x17\xbe\xd7\xaf\x38\xaf\xde\xe6\x62\x43\x29\x27\xdb\x80\x0c\x2c\x76\xe1\x26\x4e\x16\x20\xb1\x0d\xdb\xdb\x50\x14\x85\x41\x53\xc7\x31\x1b\x8a\x24\x48\xca\xad\x9b\xe5\xbf\x0f\x14\x65\x47\x72\x9c\xc4\xc6\xb6\xbb\xe9\x46\xe0\xe1\x79\xce\xf7\x07\x99\x11\xbf\xa3\x75\x42\x2b\x0a\xcb\x93\xe4\x4e\xa8\x9c\xc2\x18\xed\x52\x70\xec\x72\xae\x4b\xe5\x93\x02\x3d\xcb\x99\x67\x34\x01\x50\xac\x40\x0a\x52\x73\x26\x89\x61\x7e\x41\x8c\xd5\x4b\x11\xf0\x68\x89\x8b\x38\xc2\x6a\x60\x64\x77\x86\x71\xa4\x70\x57\xce\x90\xb8\x95\xf3\x58\x24\x84\x90\xa4\xa9\xd9\xce\x18\xcf\x58\xe9\x17\xda\x8a\x6f\xcc\x0b\xad\xb2\xbb\x9f\x5d\x26\x74\x67\x63\xd3\x99\x2c\x9d\x47\x3b\xd2\x12\xf7\x37\xc8\x06\x6e\x5b\x4a\x74\x34\x21\xc0\x8c\xb8\xb4\xba\x34\x8e\xc2\xc7\x34\xfd\x94\x00\x58\x74\xba\xb4\x1c\x2b\x8a\xd2\x39\xba\xf4\x2d\xa4\x26\x98\xe5\x3c\x2a\xbf\xd4\xb2\x2c\x90\x4b\x26\x8a\xea\x86\x6b\x35\x17\xb7\x05\x33\xae\x82\x2f\xd1\xce\x2a\xe8\x2d\xfa\x70\x2d\x85\xab\xfe\x5f\x98\xe7\x8b\xf4\xd3\xeb\x2a\x51\xe5\x46\x0b\xe5\x77\xaa\x8d\x44\x9d\x6f\xe9\xfa\x7e\x2f\xc1\x4b\x0c\x52\x5b\x40\x6e\x91\x79\xac\x84\xee\xb6\xcf\x79\x6d\xd9\x2d\xd6\xa1\x7f\x2a\xb4\xbe\xe7\x92\x39\x87\x7b\x46\xe0\x6f\x25\xfa\xbd\x50\xb9\x50\xb7\xfb\xe7\x7b\x26\x54\x9e\x84\xa4\x8f\x70\x1e\x98\xd7\xee\xbd\xa0\x38\x01\x78\x5a\x60\xfb\x94\x95\x2b\x67\x9f\x91\xfb\xaa\xb2\x76\xb6\xcd\xbf\xd5\x2c\xcc\x18\xf7\x18\xae\x73\x34\x52\xaf\x0a\x3c\xa0\x4f\x9f\x57\xe5\x0c\x72\x5a\xa5\xdd\x48\xc1\x99\xa3\x70\x92\x00\x38\x94\xc8\xbd\xb6\xe1\x06\xa0\x08\xa9\xbd\x66\x33\x94\x2e\x12\x42\x98\xcd\x0b\xba\x3c\x16\x46\x32\x8f\x35\xbc\x61\x64\xf8\x64\x4b\xd2\x6b\xb2\x00\xd6\x26\x86\xcf\x58\xa1\xad\xf0\xab\xb3\x50\x91\xfd\xca\xe3\x34\x7a\x42\x42\x33\x13\x6e\x85\x17\x9c\xc9\xb4\xe6\x77\xad\x04\xf5\x0f\xcb\x4e\xf8\xbc\x96\x68\xab\xea\x69\x58\x0c\x40\xe0\x0e\x57\x14\xd2\xb3\x5a\x5f\x37\xcf\xb5\x72\x03\x25\x57\x69\x83\x0b\x40\x9b\x80\xd6\x96\x42\xda\xfb\x2a\x9c\x77\xe9\x0e\x21\x95\xe5\xa1\xc2\xb2\x90\x19\xab\xd0\x63\xd5\x20\x5c\x2b\x6f\xb5\x24\x46\x32\x85\x07\xc8\x05\xc0\xf9\x1c\xb9\xa7\x90\xf6\xf5\x98\x2f\x30\x2f\x25\x1e\xa2\xb8\x60\xa1\x2f\xfe\x29\x8d\xc1\x0d\x26\x14\xda\x4d\x04\xc9\x6b\xc5\x1a\x3f\x51\xb0\x5b\xa4\x70\x74\x3f\xfe\x30\x9e\xf4\x6e\xa6\xe7\xbd\x8b\xee\x6f\xd7\x93\xe9\xa8\x77\x79\x35\x9e\x8c\x3e\x3c\x1c\x59\xa6\xf8\x02\x6d\x67\xb7\x20\xba\x3c\xce\x8e\xb3\x1f\x4e\xda\x02\x87\xa5\x94\x43\x2d\x05\x5f\x51\xb8\x9a\xf7\xb5\x1f\x5a\x74\xb8\x49\x78\xb0\xb7\x28\x98\xca\x1f\xd3\x4d\x5e\x33\x94\x80\xf3\xcc\xfa\xc6\x99\x90\xb8\x38\x1a\xa4\x0e\x7a\xde\x89\xd4\xfa\x97\x7d\x76\x5a\x6d\x38\xe2\x0a\xb8\x09\xb5\xe7\x9a\xba\x63\xa8\x22\x82\x44\xa6\x46\xe4\x8b\xc0\x3f\x64\x7e\x41\x5b\x0a\x36\x1c\xa8\x96\x4f\x85\x0d\x07\xe7\xd3\x7e\xf7\xa6\x37\x1e\x76\xcf\x7a\x0d\x61\x4b\x26\x4b\xbc\xb0\xba\xa0\xad\xdc\xce\x05\xca\xbc\x9e\xaf\x4f\xe8\x51\xf7\xba\xc7\xb3\xcd\x98\x49\x9a\x5e\x1d\xe0\x50\xa4\xdf\x30\xd3\xd6\xf6\xa4\x60\xea\xf8\x6e\x8f\xca\xf6\x46\x7b\x1c\x9a\xe3\x48\xaf\xe6\xc6\x8b\x63\x33\xec\x10\xa5\xb4\x6f\xf6\x7c\x73\x0d\x6e\xb5\x8a\x70\x24\xc7\x39\x2b\xa5\x27\xd5\x35\x85\xd4\xdb\x12\xd3\xa4\x59\x87\x50\xd7\x69\x00\x34\x34\x45\xdf\xeb\x95\x77\xa3\x73\xa4\xf0\x07\x13\xfe\x42\xdb\x0b\x61\x9d\x3f\xd3\xca\x95\x05\xda\xc4\xc6\xf7\xc8\xba\x68\xcf\x51\xa2\xc7\xca\xf3\x7a\x8f\xad\x43\x96\x6c\xbd\xed\x5e\x5c\x0f\x9b\x02\x7d\x66\x33\xac\x81\x8d\x5a\xa5\xf0\x27\xa9\x02\x72\x5f\xe7\xa6\x9a\x20\xa1\x02\x6e\x98\x49\xe9\xc7\x9a\x7a\xbf\xc9\x5c\x75\x9f\xd2\x74\xdd\xb9\xc3\xee\xe4\xd7\xe9\xc5\x60\x34\xed\x0f\xfa\xd3\xeb\xab\xf1\xa4\x77\x3e\xed\x0f\xce\x7b\xe3\xf4\xed\x23\x26\x58\xe7\x52\xfa\x31\x3d\xba\x5f\xe3\xae\x07\x67\xdd\xeb\xe9\x78\x32\x18\x75\x2f\x7b\x95\x94\x87\xa3\xea\x35\x12\xbe\x87\xfa\x1f\xcf\x0f\xd5\xfa\xf2\xe1\x05\x50\x1b\xfb\xff\xff\x75\x66\x42\x75\xdc\xa2\x3a\x7d\x59\x08\x89\x70\x8b\x5e\x1b\xef\x20\x2d\xa8\xa3\x86\xa6\xa0\x4d\x6c\xdf\x5c\x3f\xce\x01\xe6\x10\xde\x68\xe3\x41\xa8\x56\x2d\x9a\xef\x5a\x47\x36\x73\x5a\x96\xbe\x8a\xc3\x2f\x6f\x06\xc3\x49\x77\x74\xd9\x62\x78\xf7\xae\x75\x74\x6d\xb8\x13\xdf\xf0\x4a\xbd\x5f\x79\x74\xfb\xa0\x8b\x36\x7a\xa9\x65\xa8\x9c\xd7\x90\xe8\x18\xaf\xfd\x53\xb1\xdb\x8a\xbb\x5c\x58\x20\x05\x1c\x9f\x9e\x9e\x02\x31\xf0\xe6\xbe\xe9\x48\x0c\x2a\x5f\x14\x3a\x87\xd3\xe3\x93\xed\xdb\x4e\x96\x55\x7b\x9e\xd9\x5c\x7f\x51\xff\x85\xfa\xc5\x50\xdb\x02\x88\x9d\xef\x08\xf0\x02\xa5\x41\x3b\xd4\x79\xb6\x62\x85\xdc\x44\x71\xab\x8b\x03\x29\x36\xfa\x50\xe7\x3b\x5f\x54\xb1\xb7\xa3\x34\x62\x6a\xa6\xe6\xb3\xe9\xf9\x15\xbc\x05\x82\x83\xd6\x6e\x21\xac\xd5\x16\x73\x22\xc5\xcc\x32\xbb\x22\xb3\xd2\xad\x66\xfa\x2b\x3d\xc9\x7e\xfc\x29\x3b\x49\xfe\x0a\x00\x00\xff\xff\x4b\xe3\xeb\x3d\x6b\x0e\x00\x00")

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

var _metricsServerMetricsServerDeploymentYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x55\x41\x6f\x1b\x39\x0f\xbd\xfb\x57\x10\xfe\xd0\xdb\xa7\xd8\xee\xa2\xdd\xc5\x00\x39\x04\x89\xdb\x06\x68\xb2\x46\xec\x2e\xd0\x53\xa1\x68\xe8\x58\x88\x24\x6a\x49\x8e\x9b\xd9\xa2\xff\x7d\x21\x4f\x3a\x9d\x49\x93\xa2\x8b\xdd\xce\x69\xc0\x47\x3e\x3e\x92\x12\x65\x8c\x99\xd8\xec\xff\x40\x16\x4f\xa9\x82\xfd\x62\x72\xeb\x53\x5d\xc1\x1a\x79\xef\x1d\x9e\x38\x47\x4d\xd2\x49\x44\xb5\xb5\x55\x5b\x4d\x00\x92\x8d\x58\x41\x44\x65\xef\xc4\x08\xf2\x1e\xf9\xde\x2c\xd9\x3a\xac\xe0\xb6\xb9\x46\x23\xad\x28\xc6\xc9\xc3\x0c\x36\x67\x99\xf5\x69\xce\x30\x07\x6a\x23\xfe\xab\x14\x00\xc1\x5e\x63\x90\x12\x09\x70\xfb\x9b\x18\x9b\xf3\x37\xe1\x92\xd1\x15\x0f\xc1\x80\x4e\x89\x3b\xef\x68\xd5\xed\xde\x0e\xc2\x9f\x26\x00\x50\x8c\x39\x58\xc5\xfb\xd0\x81\xe0\xf2\x3d\x21\xba\x7c\x61\x94\xe0\x7b\x29\x00\xbe\xe8\x2c\x5f\x66\x4f\xec\xb5\x3d\x0d\x56\xe4\xf2\xc0\x3f\xed\x8a\x36\x89\x6a\x34\x8e\xbd\x7a\x67\xc3\xf4\xde\x5f\x46\x53\xbb\x7c\x5a\x90\x52\x40\xb6\xea\x29\x0d\x54\x19\xb8\xc5\xb6\x82\xe9\xe9\x3d\xeb\x49\x5d\x53\x92\xdf\x53\x68\xa7\xbd\x0f\x00\xe5\x12\x49\x5c\xc1\x74\x79\xe7\x45\x65\xfa\x0d\xc1\x41\x1b\x53\xc0\xa3\x32\x26\x4e\xa8\x28\x47\x9e\x66\x8e\x92\x32\x05\x93\x83\x4d\xf8\x83\x9c\x00\xb8\xdd\xa2\xd3\x0a\xa6\x97\xb4\x76\x3b\xac\x9b\x80\x3f\x9e\x32\x5a\x51\xe4\xff\x22\xd7\x9e\x42\x13\xb1\x6f\xd7\xff\x20\x96\x1e\x83\x4f\xa0\x31\x83\x10\x7c\x44\x70\x36\x81\xd8\x2d\x86\x16\x1a\x41\xd8\x32\x45\x23\x8e\xcb\x19\x03\x1f\xed\x0d\x0a\xd8\x54\xcf\x88\x81\xd1\xd6\x86\x52\x68\xa1\x34\xc5\xfa\x84\x2c\x93\x2f\x25\x75\x27\x49\x63\x36\xb5\xe7\x5e\x1d\xc6\xac\xed\x99\xe7\x0a\x3e\x7d\xbe\x37\x7e\x8d\xad\x1e\x04\x3f\x3a\x75\xe8\x44\x54\xf0\xec\xd3\xfa\xfd\x7a\xb3\xbc\xf8\x70\xb6\x7c\x75\xf2\xee\xed\xe6\xc3\xd5\xf2\xf5\xf9\x7a\x73\xf5\xfe\xf3\x33\xb6\xc9\xed\x90\x67\xd1\x33\x13\x63\x6d\xc6\x4c\xd5\x7e\x7e\xf4\xf2\x68\xd1\x13\x5a\xbe\x19\x9d\x20\x63\x1c\xb2\x16\xdd\xc7\x33\x8d\x79\x84\x08\xba\x86\xd1\x64\x62\x3d\x5e\xcc\x9f\xbf\x98\x8f\xd0\x32\xb7\x80\x6a\x32\xe3\x16\xb9\x64\xb6\x75\xcd\x28\x62\xb4\xcd\x28\xc7\xe7\x49\x91\x93\x0d\xe7\xab\xff\x2f\xef\xfa\xdf\x37\x24\x5a\x2a\x7e\x94\xaa\x11\xec\xee\x89\xa8\xd5\x46\x0e\x99\x47\x8e\x5d\x6d\x86\x51\x28\x34\xe5\x36\x1c\x2f\x5e\x48\xef\x51\xcc\x0d\x3b\x1c\x14\x58\x8c\x7f\x36\x28\x3a\xb2\x01\xb8\xdc\x54\xb0\x98\xcf\xe3\xc8\x1a\x31\x12\xb7\x15\xfc\x3a\xbf\xf0\x3d\x50\x44\x8c\x5a\xd6\x0d\x6c\xa7\x9a\x65\x10\xdd\x8f\x76\x45\xac\x85\x7b\xd8\xaf\xb2\x19\x48\xc9\x51\xa8\x60\x73\xba\x1a\x28\xb6\xb5\x4f\x28\xb2\x62\xba\xc6\xa1\xc4\x42\xff\x1a\x75\xac\x3a\x5b\xdd\x55\x30\x2b\x51\xed\x5f\x63\xe4\x90\xf4\xa1\x26\x00\x71\x3b\x2c\x6a\xdf\x6c\x36\xab\xf5\x00\xf1\xc9\xab\xb7\xe1\x0c\x83\x6d\xd7\xe8\x28\xd5\x52\xc1\x48\x2f\xb2\xa7\xba\x87\x9e\x0f\x20\xf5\x11\xa9\xd1\x1e\x5b\x0c\x30\x69\x9c\x43\x91\xcd\x8e\x51\x76\x14\xea\x31\xba\xb5\x3e\x34\x8c\x03\xf4\x97\x1e\x0d\x7e\x8f\xff\xb8\x13\x25\xe8\x27\x34\xe2\xe5\x77\x3a\xb1\x98\xff\xf4\x56\x1c\xee\x5d\x79\x45\x28\x29\xde\xe9\xf8\x34\xdb\xba\x2c\xf8\x2b\x22\x7d\xe5\x03\x76\x8f\x4b\x05\xca\x0d\x0e\xdd\x9a\x74\x22\x97\x94\x8a\xdb\xe3\xe0\x3b\x41\x3e\xdc\x80\x61\x39\x36\x04\xfa\xb8\x62\xbf\xf7\x01\x6f\x70\x29\xce\x86\xc3\x9b\x53\xc1\xd6\x06\xf9\xca\xd1\xad\xd6\x8b\xb2\x4f\x1f\xb9\x19\x0f\xf7\x20\x74\x9b\x77\xd5\x8d\xac\x2c\x99\xbf\x03\x00\x00\xff\xff\x74\x6c\x64\x11\xc6\x08\x00\x00")

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

var _traefikYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x92\xcf\x6e\xdb\x3c\x10\xc4\xef\x7a\x0a\x42\x80\x8f\x94\x9c\x9c\xf2\xe9\xe6\xcf\x51\xda\xa0\x6d\x1a\x58\x4e\x8b\x9c\x8c\x15\xb5\xb6\x08\x53\xa4\xb0\xbb\x72\xeb\xa6\x79\xf7\x82\x56\x9c\x3f\x40\x80\x14\x45\x7b\xf4\x7a\xf9\x9b\xd9\x19\x69\xad\x13\xe8\xed\x17\x24\xb6\xc1\x17\xaa\x45\xd7\x65\x06\x44\x1c\x66\x36\xe4\xbb\x93\x64\x6b\x7d\x53\xa8\xf7\xe8\xba\x79\x0b\x24\x49\x87\x02\x0d\x08\x14\x89\x52\x1e\x3a\x2c\x94\x10\xe0\xda\x6e\xb5\xa1\xe6\x61\xc6\x3d\x18\x2c\xd4\x76\xa8\x51\xf3\x9e\x05\xbb\x84\x7b\x34\xf1\x89\x89\x90\x42\xb5\x22\x3d\x17\x79\x3e\xb9\xfb\x70\xf3\x7f\xb9\xb8\x2a\x97\x65\xb5\x9a\x5d\x5f\xde\x4f\x72\x16\x10\x6b\xf2\xc3\x22\xe7\xcf\xe0\xfa\xe4\x34\x9b\x66\xd3\xe9\x34\x93\xcd\x8f\xe4\x2f\x3a\xff\x77\xae\x5f\x38\x56\x8a\x51\x22\x4d\xa9\x8d\x0b\x35\xb8\x6c\x54\x39\xc7\x35\x0c\x4e\x16\xb8\xb1\x2c\xb4\x2f\x54\x3a\xb9\xab\x6e\xab\x65\xf9\x69\x75\x5e\x5e\xcc\x6e\x3e\x2e\x57\x8b\xf2\xdd\x65\xb5\x5c\xdc\xae\x16\xb3\xaf\xf7\x93\x34\x51\x6a\x07\x6e\x40\x9e\x07\x2f\xe8\xa5\x50\x3f\xf5\x81\x4b\x35\x98\x51\x41\x29\xf4\x50\x3b\x6c\xe2\x95\x03\x1e\x66\x7d\x20\xe1\xe3\xdf\xdf\xb0\x66\x34\x03\xe1\x71\xa0\x94\x38\x7e\xfa\xf1\x3a\xa0\x99\x79\x1f\xe2\xa9\xc1\x3f\xee\xf6\x14\x3a\x94\x16\x07\x8e\xc1\x47\x91\x42\xa5\x67\xd3\xb3\xd3\xf4\xd5\x05\x36\x04\x3d\x16\x2a\x8d\xd8\x71\xa5\xa7\xb0\xb3\x0d\xd2\x23\x32\x76\x40\x1e\x05\xf9\xd2\x6f\x08\xf9\x99\xaf\x7e\xa8\x9d\xe5\x16\x9b\x0a\x69\x67\x0d\xbe\xe1\x98\x6c\x20\x2b\xfb\xb9\x03\xe6\xab\x43\xe9\xe9\x98\xba\x36\x6e\x60\x41\xd2\x86\xac\x58\x03\x6e\xb4\x62\x3b\xd8\x3c\x32\xc7\xaf\x24\x25\xf0\xa6\x45\xca\x3b\x4b\x14\x08\x1b\xed\x6c\x4d\x40\x7b\xfd\x50\xf3\xf1\x4e\x81\x4d\xa1\xd2\xd3\xec\xbf\xec\x64\x1c\x49\x70\x48\xcf\xc3\xd2\x6a\x8b\xb1\xe0\xf9\x83\xe6\xac\x69\x82\xe7\xcf\xde\xed\x8f\x8c\xd0\xc7\x17\x81\x0a\x95\x96\xdf\x2d\x0b\xa7\x2f\x1e\xfa\xd0\xa0\xa6\xe0\x30\x7b\x8a\x28\x86\x6a\x82\x17\x0a\x4e\xf7\x0e\x3c\xbe\xc1\x52\x0a\xd7\x6b\x34\xb1\xa5\xab\x50\x99\x16\x9b\xc1\xe1\xef\xc9\x74\x10\x23\xfb\x73\x3e\xbf\xec\xcc\xf6\x17\xd0\x59\xb7\xbf\x0e\xce\x9a\xa8\x7b\x4d\xb8\x46\x3a\x1f\xc0\x55\x02\x66\x9b\x26\xbf\x02\x00\x00\xff\xff\xb0\xd6\x94\xa9\xa0\x04\x00\x00")

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
