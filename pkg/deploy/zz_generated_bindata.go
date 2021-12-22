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

var _ccmYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xcc\x94\x41\x8f\x13\x31\x0c\x85\xef\xf9\x15\xd1\x5e\x56\x42\x4a\x57\x88\x0b\x9a\x23\x1c\xb8\xaf\x04\x77\x37\x79\x74\x43\x33\x71\x14\x3b\xb3\xc0\xaf\x47\xe9\x2c\x68\x99\xa1\x55\x5b\x40\x70\x8b\x2a\xfb\x7b\xcf\xcf\xf5\x50\x89\x1f\x50\x25\x72\x1e\x6c\xdd\x92\xdf\x50\xd3\x07\xae\xf1\x2b\x69\xe4\xbc\xd9\xbf\x96\x4d\xe4\xbb\xe9\xa5\xd9\xc7\x1c\x06\xfb\x36\x35\x51\xd4\x7b\x4e\x30\x23\x94\x02\x29\x0d\xc6\xda\x4c\x23\x06\xbb\x7f\x25\xce\x27\x6e\xc1\x79\xce\x5a\x39\x25\x54\x37\x52\xa6\x1d\xaa\xa9\x2d\x41\x06\xe3\x2c\x95\xf8\xae\x72\x2b\xd2\x1b\x9d\xf5\xcc\x35\xc4\xfc\x5c\xcf\x58\x5b\x21\xdc\xaa\xc7\x53\x51\x02\x09\xc4\x58\x3b\xa1\x6e\x9f\x7e\xdb\x41\x67\x40\x05\x29\x0e\xcf\x56\x42\x7f\xae\x34\x6e\x6e\xd6\x48\x4c\xc8\xba\x40\x3e\x43\x15\x52\xff\x70\x31\x34\x73\x58\xda\xbc\x7d\x71\x7b\x41\xef\x9d\x28\x69\x5b\x20\x66\x2f\x67\x41\x04\x75\x8a\x7e\xe9\x21\x45\xd1\x5f\x4f\xd5\x9f\x8f\x17\xe3\xc9\x7b\x6e\xc7\xd2\x3b\x0b\x54\xfa\x9f\x4e\x14\x59\x27\x4e\x6d\x3c\xb6\xdb\x1f\xc6\xaf\xb3\x8b\x1c\x0a\xc7\x53\x6b\x5e\x09\x3d\xae\xf6\xee\x9c\xb9\xfe\x4a\xde\xc4\x1c\x62\xde\x5d\x7c\x2c\x9c\x70\x8f\x8f\xbd\xfa\xfb\x98\x27\x94\x8d\xb5\xeb\xf3\x3c\x4b\x47\xda\xf6\x13\xbc\x1e\xee\x72\x46\xbc\x17\xd4\xf3\x7a\xe7\x22\x29\xe4\x7b\x65\xdb\xc2\xc9\x17\x51\x8c\xff\x24\x31\xd7\xf9\x2e\x20\x61\x47\xca\x7f\x34\xc0\x79\xaa\x61\x21\xf0\xbf\x24\xf7\x9b\x91\x21\x6b\xf4\x07\xb2\xab\xa0\x70\xca\xdc\x95\x91\xfe\x94\x25\x3e\x2b\x72\x9f\xcd\x51\x89\xfd\x63\x72\xd4\xc6\x5f\xc9\xf7\x5b\x00\x00\x00\xff\xff\xc2\xa7\x17\xb8\xee\x06\x00\x00")

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

var _corednsYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x57\x51\x6f\xdb\x38\x12\x7e\xf7\xaf\x20\x74\xc8\xcb\xe1\xe4\xc4\x97\x6b\x2f\xab\xb7\x34\x76\xdb\x60\x1b\xd7\xb0\x9d\x02\xc5\x62\x11\x8c\xa9\xb1\xc5\x0d\xc5\xe1\x92\x94\x13\x6f\x37\xff\x7d\x41\x49\x96\x45\x5b\x49\x93\x6c\x57\x2f\xb6\x44\xce\x37\xc3\x8f\xc3\x6f\x86\xa0\xc5\x17\x34\x56\x90\x4a\xd8\x7a\xd0\xbb\x15\x2a\x4d\xd8\x0c\xcd\x5a\x70\x3c\xe7\x9c\x0a\xe5\x7a\x39\x3a\x48\xc1\x41\xd2\x63\x4c\x41\x8e\x09\xe3\x64\x30\x55\xb6\x7e\xb7\x1a\x38\x26\xec\xb6\x58\x60\x6c\x37\xd6\x61\xde\x8b\xe3\xb8\xd7\x86\x36\x0b\xe0\x7d\x28\x5c\x46\x46\xfc\x01\x4e\x90\xea\xdf\x9e\xd9\xbe\xa0\xe3\xc6\xe9\x85\x2c\xac\x43\x33\x25\x89\x81\x47\x09\x0b\x94\xd6\xff\x63\xa5\x0b\xa3\xd0\x61\x69\xba\x20\x72\xd6\x19\xd0\x5a\xa8\x55\xe5\x23\x4e\x71\x09\x85\x74\xb6\x09\xb5\x0a\x28\xd9\x46\x6c\x0a\x89\x36\xe9\xc5\x0c\xb4\xf8\x60\xa8\xd0\x25\x72\xcc\xa2\xa8\xc7\x98\x41\x4b\x85\xe1\x58\x7f\x43\x95\x6a\x12\xaa\x04\x8b\x99\xad\x48\xa9\x5e\x34\xa5\xd5\x9f\x66\xfd\xfe\x75\x8d\x66\x51\xdb\x4a\x61\x5d\xf9\xe7\x0e\x1c\xcf\x0e\xfd\xa5\xc2\x72\x5a\xa3\xd9\xd4\x3c\x3c\xe1\x5d\x8a\xef\xa2\xff\x2d\xb6\xdf\x09\x95\x0a\xb5\x0a\x48\x07\xa5\xc8\x95\x96\x35\xf3\x5d\x90\xc1\x66\x40\xe1\xa8\xd0\x29\x38\x4c\x58\xe4\x4c\x81\xd1\x8f\xdf\x3b\x92\x38\xc5\x65\x19\x5f\xcd\xe6\x13\x6b\xed\x31\x76\x98\x58\x8f\x20\xdb\x62\xf1\x1b\x72\x57\x26\x46\xe7\x11\x78\x75\xe2\xef\x08\x27\xb5\x14\xab\x2b\xd0\xaf\x39\x4e\xdb\xe9\x17\x64\x70\x29\x24\x26\xec\xcf\x92\xd3\x7e\xf2\xe6\x94\x7d\x2b\xff\xfa\x07\x8d\x21\x63\x9b\xd7\x0c\x41\xba\xac\x79\x35\x08\xe9\xa6\x79\xdb\x6d\x07\x3b\xfa\x76\xf1\xe9\x7a\x36\x1f\x4d\x6f\x86\x9f\xaf\xce\x2f\xc7\x0f\x47\x4c\xa8\x18\xd2\xd4\xf4\xc1\x68\x60\x42\xbf\xad\xfe\xec\x3c\xb1\xf2\x04\x30\xa1\x2c\xf2\xc2\x60\xeb\xfb\x12\xa4\x74\x99\xa1\x62\x95\x75\xa3\x34\x73\x1f\x76\x81\x92\x75\x96\x1d\xa3\xe3\xc7\x35\x15\xc7\x63\x4a\xf1\x63\xf9\xb9\xed\xd4\x39\xc9\xde\x9e\xb4\x3e\x18\x94\x04\x29\x1b\xbc\xb1\xdd\x21\x74\x38\xd3\x86\x72\x74\x19\x16\x96\x25\x3f\x0d\xde\x9c\x36\x03\x4b\x32\x77\x60\x52\xd6\xaf\x22\xf1\xc7\x51\xae\xfb\x9c\xd4\xb2\x99\xc2\x81\x67\xc8\x4e\x77\x11\x48\x22\xdd\x0b\x83\x69\x8d\x41\xba\x00\x09\x8a\x57\xfc\x54\x21\x88\x5c\x93\x71\xe1\x62\x79\x61\x1d\xe5\xc7\xff\xee\x7b\x8d\x41\x73\x90\x44\xa0\xb5\xdd\x1d\xdd\x21\x6a\x49\x9b\x1c\x5f\xa7\xcc\x7b\x87\xf2\xcc\xc6\xa0\x75\x3d\xa5\x32\xdc\x3f\xaa\x15\x70\xe4\x73\x6f\x38\x9e\x45\x3d\xab\x91\x7b\xeb\x7f\x19\xd4\x52\x70\xb0\x09\x1b\xf4\x18\xf3\xa7\xd9\xe1\x6a\x53\x01\xbb\x8d\xc6\x84\x4d\x49\x4a\xa1\x56\xd7\xa5\x2e\x54\x3a\xd2\xfe\x92\xd4\x5c\xe5\x70\x7f\xad\x60\x0d\x42\xc2\xc2\x27\x77\x09\x87\x12\xb9\x23\x53\xcd\xc9\xbd\xce\x7d\x6a\x05\xde\x1d\xba\xc3\x5c\xcb\x06\xb8\xcd\x4e\xb9\x21\x81\xfd\x63\x8b\xdf\x2e\xaf\xca\x15\x41\x46\xb8\xcd\x85\x04\x6b\xc7\x15\x0f\x15\x8f\x31\xaf\x54\x25\xe6\x46\x38\xc1\x41\x46\xb5\x89\x0d\x84\x63\xbc\xb7\x29\x25\x35\x24\xd1\xb4\xb5\xd5\x3f\x31\xbb\xc5\x8d\x67\xb9\x86\x3b\x4f\x53\x52\xf6\xb3\x92\x9b\xa8\x95\xd9\xa4\xbd\x25\x99\x84\x45\xa3\x7b\x61\x9d\x8d\x0e\x00\x14\xa5\x18\x7b\xa5\xdc\xd3\x67\x4e\xca\x19\x92\xb1\x96\xa0\xf0\x99\x98\x8c\xe1\x72\x89\xdc\x25\x2c\x1a\xd3\x8c\x67\x98\x16\x12\x9f\xef\x32\x07\xcf\xd0\x8f\xf0\xe5\x3d\xcc\x82\x84\xf0\xcf\x02\x1d\xec\xb9\x24\x9b\x30\x29\x54\x71\xdf\x70\xad\x49\xd2\x6a\x33\xd3\x5e\xfd\x2e\x48\xf9\x2c\xf5\x45\xb5\xcd\x7c\x0e\xf7\xb3\x5b\xbc\xab\xf2\x6e\xfb\x6c\x2d\x7f\xf6\x4b\x0c\x9d\x78\xb9\xf2\x87\xa2\x35\xfb\x2e\x43\x75\xad\x2c\x38\x61\x97\xa2\x4a\xe2\x21\x8d\xc9\x6d\x17\xd2\x9a\x5a\x66\xe1\xe1\x62\x1e\xc9\xf2\xa7\x73\x95\x31\xbf\xad\x20\x14\x9a\xc6\x22\x3e\x50\x82\xea\x11\x39\xac\x30\x61\x47\xdf\x66\x5f\x67\xf3\xd1\xd5\xcd\x70\xf4\xfe\xfc\xfa\xd3\xfc\x66\x3a\xfa\x70\x39\x9b\x4f\xbf\x3e\x1c\x19\x50\x3c\x43\x73\x9c\x0b\x5f\x47\x30\x8d\x6b\x88\xed\x6f\x32\xe8\x9f\xf5\xff\x17\x02\x4e\x0a\x29\x27\x24\x05\xdf\x24\xec\x72\x39\x26\x37\x31\x68\xb1\xac\x98\xd5\x13\x74\x35\x0d\x07\x22\x17\x6e\x6f\x8d\x39\xe6\x64\x36\x09\x1b\xfc\xff\xe4\x4a\x04\x12\xff\x7b\x81\x76\x7f\x36\xd7\x45\xc2\x06\x27\x27\x79\x27\x46\x00\x01\x66\x65\x13\xf6\x0b\x8b\x62\xaf\xe5\xd1\x7f\x58\x14\x88\xef\xb6\xa6\x46\xec\xd7\xc6\x64\x4d\xb2\xc8\xf1\xca\x9f\xe0\x20\x53\xb6\xcc\xfa\x52\x1e\x57\x93\x5a\xfe\x73\x3f\x7f\x02\x2e\x4b\x02\x79\x0f\xd6\x02\xa9\x3f\xd3\x09\xf3\x1d\xd2\x21\x70\x59\x07\xe2\x17\xe2\xd7\xe5\xe3\xfb\x6e\x7c\xe1\x09\x96\xd3\x24\xcf\x84\x8c\x4b\x58\xab\x16\x6e\xcb\x49\x18\xbe\x36\xe4\x88\x93\x4c\xd8\xf5\x70\xf2\x52\x9c\xd8\x71\xdd\x89\x35\xbf\x78\x02\x2b\xa8\xd0\x5b\xb4\x1c\x9d\x11\xbc\x3b\xb2\x36\x5a\xd9\x9c\x78\xf9\x26\xe5\xf0\xde\xb5\x33\x08\xa4\xa4\xbb\x89\x11\x6b\x21\x71\x85\x23\xcb\x41\x96\x92\x9c\xf8\xee\xc1\xb6\x59\xe7\xa0\x61\x21\xa4\x70\x02\xf7\x72\x10\xd2\x34\xfc\x10\xb3\xf1\x68\x7e\xf3\xee\x72\x3c\xbc\x99\x8d\xa6\x5f\x2e\x2f\x46\xc1\x70\x6a\x48\xef\x1b\x80\x94\x1d\x1b\x37\x25\x72\xef\x85\xc4\xba\x4d\x0d\xb7\x51\x8a\x35\x2a\xb4\x76\x62\x68\x81\x6d\xbc\xcc\x39\xfd\x01\x5d\xe8\x42\x57\xf9\xb2\xd7\x0b\xb2\x3a\x1d\x12\x76\x76\x72\x76\x12\x7c\xb6\x3c\x43\x4f\xf2\xc7\xf9\x7c\xd2\x1a\x10\x4a\x38\x01\x72\x88\x12\x36\x33\xe4\xa4\x52\x9b\x84\xbd\x98\x46\x23\x28\x6d\xc6\x06\xed\x31\x27\x72\xa4\xc2\xed\x06\x5b\x63\xb6\xe0\x1c\xad\x9d\x67\x06\x6d\x46\x32\x0d\x47\x97\x20\x64\x61\xb0\x35\x7a\x1a\x74\xb4\xe2\xc5\x54\x84\x7d\x70\x8b\x89\xc1\xd9\xe0\xd5\x4c\x3c\x41\xc4\x7f\xff\x61\x1e\x52\x65\xb7\x0a\x3c\xac\x6e\x50\xf5\x40\x25\x20\x2f\x10\x30\xbe\xbd\xa3\x84\xbc\x75\xd7\x93\x92\x0a\x87\xb9\xdd\x4f\xe9\xb2\x29\xd8\xaa\xea\x5e\x19\xab\xb6\xa0\x73\xb0\x36\x6c\x1a\xff\x4e\xcb\xc3\xd1\x67\x6a\xe7\x73\x96\x16\x1f\x08\xa9\xef\x58\xbc\x2a\x80\xac\xcf\xe0\xa3\xd7\xbb\xfa\xbe\xd8\xd1\x91\xb7\x0a\xf6\xa3\x2d\xf9\xc1\x75\x7b\x77\x49\xf1\x0d\x47\x95\x9f\x91\xd7\xc2\xa8\x63\xd8\x72\x03\xfa\xd1\x6b\xf7\x33\x3a\xfc\x6d\x2f\x5b\xf7\xae\x2d\xa4\xe7\xde\x05\xc2\x6e\xbd\xcb\x67\xed\xe3\x72\x92\xb4\xef\x9b\xe3\xd9\xc3\x51\xaf\x55\x99\xe2\xbd\xba\xa3\xdb\x05\x65\xbf\xfc\xc4\x1d\xc5\xe5\x11\x83\xaa\x2a\xc4\x1d\xf5\x43\x87\x65\x26\x34\xf9\x2b\x00\x00\xff\xff\x78\xb0\xd9\x96\x1e\x13\x00\x00")

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

var _localStorageYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xec\x56\x5d\x6f\xdb\x36\x17\xbe\xd7\xaf\x38\xaf\xde\xe6\x62\x43\x29\x27\xdd\x45\x06\x16\xbb\x70\x13\x27\x0b\x90\xd8\x86\xed\x6d\x28\x8a\xc2\xa0\xa9\xe3\x98\x0d\x45\x12\x24\xe5\xd6\xcd\xf2\xdf\x07\x8a\xb2\x23\x39\x4e\x62\x63\xdb\xdd\x74\x23\xf0\xf0\x3c\xe7\xfb\x83\xcc\x88\xdf\xd1\x3a\xa1\x15\x85\xe5\x49\x72\x27\x54\x4e\x61\x8c\x76\x29\x38\x76\x39\xd7\xa5\xf2\x49\x81\x9e\xe5\xcc\x33\x9a\x00\x28\x56\x20\x05\xa9\x39\x93\xc4\x30\xbf\x20\xc6\xea\xa5\x08\x78\xb4\xc4\x45\x1c\x61\x35\x30\xb2\x3b\xc3\x38\x52\xb8\x2b\x67\x48\xdc\xca\x79\x2c\x12\x42\x48\xd2\xd4\x6c\x67\x8c\x67\xac\xf4\x0b\x6d\xc5\x77\xe6\x85\x56\xd9\xdd\xcf\x2e\x13\xba\xb3\xb1\xe9\x4c\x96\xce\xa3\x1d\x69\x89\xfb\x1b\x64\x03\xb7\x2d\x25\x3a\x9a\x10\x60\x46\x5c\x5a\x5d\x1a\x47\xe1\x53\x9a\x7e\x4e\x00\x2c\x3a\x5d\x5a\x8e\x15\x45\xe9\x1c\x5d\xfa\x16\x52\x13\xcc\x72\x1e\x95\x5f\x6a\x59\x16\xc8\x25\x13\x45\x75\xc3\xb5\x9a\x8b\xdb\x82\x19\x57\xc1\x97\x68\x67\x15\xf4\x16\x7d\xb8\x96\xc2\x55\xff\xaf\xcc\xf3\x45\xfa\xf9\x75\x95\xa8\x72\xa3\x85\xf2\x3b\xd5\x46\xa2\xce\xb7\x74\xfd\xb8\x97\xe0\x25\x06\xa9\x2d\x20\xb7\xc8\x3c\x56\x42\x77\xdb\xe7\xbc\xb6\xec\x16\xeb\xd0\x3f\x15\x5a\xdf\x73\xc9\x9c\xc3\x3d\x23\xf0\xb7\x12\xfd\x41\xa8\x5c\xa8\xdb\xfd\xf3\x3d\x13\x2a\x4f\x42\xd2\x47\x38\x0f\xcc\x6b\xf7\x5e\x50\x9c\x00\x3c\x2d\xb0\x7d\xca\xca\x95\xb3\x2f\xc8\x7d\x55\x59\x3b\xdb\xe6\xdf\x6a\x16\x66\x8c\x7b\x0c\xd7\x39\x1a\xa9\x57\x05\x1e\xd0\xa7\xcf\xab\x72\x06\x39\xad\xd2\x6e\xa4\xe0\xcc\x51\x38\x49\x00\x1c\x4a\xe4\x5e\xdb\x70\x03\x50\x84\xd4\x5e\xb3\x19\x4a\x17\x09\x21\xcc\xe6\x05\x5d\x1e\x0b\x23\x99\xc7\x1a\xde\x30\x32\x7c\xb2\x25\xe9\x35\x59\x00\x6b\x13\xc3\x67\xac\xd0\x56\xf8\xd5\x59\xa8\xc8\x7e\xe5\x71\x1a\x3d\x21\xa1\x99\x09\xb7\xc2\x0b\xce\x64\x5a\xf3\xbb\x56\x82\xfa\x87\x65\x27\x7c\x5e\x4b\xb4\x55\xf5\x34\x2c\x06\x20\x70\x87\x2b\x0a\xe9\x59\xad\xaf\x9b\xe7\x5a\xb9\x81\x92\xab\xb4\xc1\x05\xa0\x4d\x40\x6b\x4b\x21\xed\x7d\x13\xce\xbb\x74\x87\x90\xca\xf2\x50\x61\x59\xc8\x8c\x55\xe8\xb1\x6a\x10\xae\x95\xb7\x5a\x12\x23\x99\xc2\x03\xe4\x02\xe0\x7c\x8e\xdc\x53\x48\xfb\x7a\xcc\x17\x98\x97\x12\x0f\x51\x5c\xb0\xd0\x17\xff\x94\xc6\xe0\x06\x13\x0a\xed\x26\x82\xe4\xb5\x62\x8d\x9f\x28\xd8\x2d\x52\x38\xba\x1f\x7f\x1c\x4f\x7a\x37\xd3\xf3\xde\x45\xf7\xb7\xeb\xc9\x74\xd4\xbb\xbc\x1a\x4f\x46\x1f\x1f\x8e\x2c\x53\x7c\x81\xb6\xb3\x5b\x10\x5d\x1e\x67\xc7\xd9\xbb\xe3\xb6\xc0\x61\x29\xe5\x50\x4b\xc1\x57\x14\xae\xe6\x7d\xed\x87\x16\x1d\x6e\x12\x1e\xec\x2d\x0a\xa6\xf2\xc7\x74\x93\xd7\x0c\x25\xe0\x3c\xb3\xbe\x71\x26\x24\x2e\x8e\x06\xa9\x83\x9e\x77\x22\xb5\xfe\x65\x5f\x9c\x56\x1b\x8e\xb8\x02\x6e\x42\xed\xb9\xa6\xee\x18\xaa\x88\x20\x91\xa9\x11\xf9\x22\xf0\x0f\x99\x5f\xd0\x96\x82\x0d\x07\xaa\xe5\x53\x61\xc3\xc1\xf9\xb4\xdf\xbd\xe9\x8d\x87\xdd\xb3\x5e\x43\xd8\x92\xc9\x12\x2f\xac\x2e\x68\x2b\xb7\x73\x81\x32\xaf\xe7\xeb\x13\x7a\xd4\xbd\xee\xf1\x6c\x33\x66\x92\xa6\x57\x07\x38\x14\xe9\x37\xcc\xb4\xb5\x3d\x29\x98\x3a\xbe\xdb\xa3\xb2\xbd\xd1\x1e\x87\xe6\x38\xd2\xab\xb9\xf1\xe2\xd8\x0c\x3b\x44\x29\xed\x9b\x3d\xdf\x5c\x83\x5b\xad\x22\x1c\xc9\x71\xce\x4a\xe9\x49\x75\x4d\x21\xf5\xb6\xc4\x34\x69\xd6\x21\xd4\x75\x1a\x00\x0d\x4d\xd1\xf7\x7a\xe5\xdd\xe8\x1c\x29\xfc\xc1\x84\xbf\xd0\xf6\x42\x58\xe7\xcf\xb4\x72\x65\x81\x36\xb1\xf1\x3d\xb2\x2e\xda\x73\x94\xe8\xb1\xf2\xbc\xde\x63\xeb\x90\x25\x5b\x6f\xbb\x17\xd7\xc3\xa6\x40\x9f\xd9\x0c\x6b\x60\xa3\x56\x29\xfc\x49\xaa\x80\xdc\xd7\xb9\xa9\x26\x48\xa8\x80\x1b\x66\x52\xfa\xa9\xa6\xde\x6f\x32\x57\xdd\xa7\x34\x5d\x77\xee\xb0\x3b\xf9\x75\x7a\x31\x18\x4d\xfb\x83\xfe\xf4\xfa\x6a\x3c\xe9\x9d\x4f\xfb\x83\xf3\xde\x38\x7d\xfb\x88\x09\xd6\xb9\x94\x7e\x4a\x8f\xee\xd7\xb8\xeb\xc1\x59\xf7\x7a\x3a\x9e\x0c\x46\xdd\xcb\x5e\x25\xe5\xe1\xa8\x7a\x8d\x84\xef\xa1\xfe\xc7\xf3\x43\xb5\xbe\x7c\x78\x01\xd4\xc6\xfe\xff\x7f\x9d\x99\x50\x1d\xb7\xa8\x4e\x5f\x17\x42\x22\xdc\xa2\xd7\xc6\x3b\x48\x0b\xea\xa8\xa1\x29\x68\x13\xdb\x37\xd7\x8f\x73\x80\x39\x84\x37\xda\x78\x10\xaa\x55\x8b\xe6\x87\xd6\x91\xcd\x9c\x96\xa5\xaf\xe2\xf0\xcb\x9b\xc1\x70\xd2\x1d\x5d\xb6\x18\xde\xbf\x6f\x1d\x5d\x1b\xee\xc4\x77\xbc\x52\x1f\x56\x1e\xdd\x3e\xe8\xa2\x8d\x5e\x6a\x19\x2a\xe7\x35\x24\x3a\xc6\x6b\xff\x54\xec\xb6\xe2\x2e\x17\x16\x48\x01\xc7\xa7\xa7\xa7\x40\x0c\xbc\xb9\x6f\x3a\x12\x83\xca\x17\x85\xce\xe1\xf4\xf8\x64\xfb\xb6\x93\x65\xd5\x9e\x67\x36\xd7\x5f\xd5\x7f\xa1\x7e\x31\xd4\xb6\x00\x62\xe7\x3b\x02\xbc\x40\x69\xd0\x0e\x75\x9e\xad\x58\x21\x37\x51\xdc\xea\xe2\x40\x8a\x8d\x3e\xd4\xf9\xce\x17\x55\xec\xed\x28\x8d\x98\x9a\xa9\xf9\x6c\x7a\x7e\x05\x6f\x81\xe0\xa0\xb5\x5b\x08\x6b\xb5\xc5\x9c\x48\x31\xb3\xcc\xae\xc8\xac\x74\xab\x99\xfe\x46\x4f\xb2\x9f\xde\x65\x27\xc9\x5f\x01\x00\x00\xff\xff\x46\x2c\x38\x08\x6b\x0e\x00\x00")

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

var _metricsServerMetricsServerDeploymentYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xac\x52\x5d\x4b\x5b\x4d\x10\xbe\x3f\xbf\x62\x38\x2f\x92\x9b\x77\x13\x7b\x51\x28\x7b\x27\x9a\x96\x82\xda\x62\x6c\x41\x4a\x91\x71\xcf\xc4\x2c\xd9\x2f\x66\x26\x69\x0f\xe2\x7f\x2f\x6b\xe2\xa9\xf1\xa3\x08\xed\x5e\x2e\xcf\xd7\x3c\x33\xc6\x98\x06\x8b\xff\x4a\x2c\x3e\x27\x0b\xeb\x37\xcd\xd2\xa7\xce\xc2\x8c\x78\xed\x1d\x1d\x38\x97\x57\x49\x9b\x48\x8a\x1d\x2a\xda\x06\x20\x61\x24\x0b\x91\x94\xbd\x13\x23\xc4\x6b\xe2\xed\xb7\x14\x74\x64\x61\xb9\xba\x22\x23\xbd\x28\xc5\xe6\xb1\x03\x96\x22\x93\xc1\xe6\x88\x4a\xc8\x7d\xa4\xbf\xb2\x00\x08\x78\x45\x41\x2a\x13\x60\xf9\x4e\x0c\x96\xf2\x84\x2e\x85\x5c\x45\x08\x05\x72\x9a\x79\x83\x8e\xa8\x6e\x71\xfc\x80\xfe\xb2\x00\x80\x52\x2c\x01\x95\xb6\xd4\x07\x81\xeb\x7b\x21\x74\x7d\x61\xc7\xe0\x4f\x16\x00\xf7\x39\xeb\x2b\xec\x33\x7b\xed\x0f\x03\x8a\x9c\xde\xe9\xb7\x9b\xa1\x4d\xca\x1d\x19\xc7\x5e\xbd\xc3\xd0\x6e\xf1\xb2\xb3\xb5\xd3\x97\x03\x69\x0e\xc4\xa8\x3e\xa7\x07\xa9\x0c\x2c\xa9\xb7\xd0\x1e\x6e\x55\x0f\xba\x2e\x27\xf9\x94\x42\xdf\x0e\x18\x80\x5c\x2a\x33\xb3\x85\x76\xfa\xd3\x8b\x4a\xfb\x44\xe0\x2e\x1b\xe7\x40\xe3\xba\x26\x4e\xa4\x24\x63\x9f\x27\x2e\x27\xe5\x1c\x4c\x09\x98\xe8\x95\x9a\x00\x34\x9f\x93\x53\x0b\xed\x69\x9e\xb9\x05\x75\xab\x40\xaf\xb7\x8c\x28\x4a\xfc\x2f\xbc\xd6\x39\xac\x22\x0d\x75\xfd\x07\xb1\x76\x0c\x3e\x81\xc6\x02\x92\xe1\x07\x81\xc3\x04\x82\x73\x0a\x3d\xac\x84\x60\xce\x39\x1a\x71\x5c\x6f\x0c\x7c\xc4\x6b\x12\xc0\xd4\x4d\x32\x03\x13\x76\x26\xa7\xd0\x43\x2d\x05\x7d\x22\x96\xe6\x7e\xa4\xcd\x25\x69\x2c\xa6\xf3\x3c\xa4\xa3\x58\xb4\x3f\xf2\x6c\xe1\xe6\x76\xfb\xf9\x9b\x6b\x1f\x91\x9f\xdd\x3a\x6c\x42\x58\xd8\xbb\x99\x5d\xcc\xce\xa7\x27\x97\x47\xd3\xf7\x07\x5f\x8e\xcf\x2f\xcf\xa6\x1f\x3e\xce\xce\xcf\x2e\x6e\xf7\x18\x93\x5b\x10\x4f\xa2\x67\xce\x4c\x9d\xd9\x55\xb2\xeb\xfd\xf1\xdb\xf1\xfe\x20\x88\x7c\x2d\x16\xbe\x8d\x8c\x71\xc4\x5a\xf3\x8e\xfe\x87\xd1\x44\x63\x19\x7d\x1f\x40\x9b\xea\x4e\x6a\x5f\x3b\xe7\xf6\xfc\x9c\xb0\x69\xf6\x33\xea\xc2\x42\x55\x6a\x9a\x5f\x01\x00\x00\xff\xff\xbc\xb5\xfd\xd0\xa7\x04\x00\x00")

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

var _metricsServerMetricsServerServiceYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\x6c\x8e\xcd\x4a\x04\x31\x10\x84\xef\x79\x8a\x26\xf7\x28\xe2\x1e\x24\x57\xcf\xc2\x82\xe2\xbd\x37\x5b\x48\x98\xfc\xd1\xdd\x33\xe0\xdb\xcb\xc4\x41\x10\xe6\xd6\x54\x57\x7d\x55\x21\x04\xc7\x23\x7f\x42\x34\xf7\x16\x69\x7b\x72\x4b\x6e\xf7\x48\xef\x90\x2d\x27\xb8\x0a\xe3\x3b\x1b\x47\x47\xd4\xb8\x22\x52\x85\x49\x4e\x1a\x14\xb2\x41\x0e\x59\x07\x27\x44\x5a\xd6\x1b\x82\x7e\xab\xa1\x3a\xa2\xc2\x37\x14\xdd\x93\x34\x3f\xd2\x60\xd0\x87\xdc\x1f\x7f\x49\xfe\xed\x1f\xca\x9f\x18\x53\x59\xd5\x20\xd3\x91\xf7\x06\x6f\xb2\xc2\x3b\x1d\x48\x3b\x58\x51\x90\xac\xcb\x51\xf2\xa2\x81\xc7\x38\xd9\x38\xba\xd8\x5c\x12\xe6\x19\xe9\x72\x79\x9e\x91\x21\xdd\x7a\xea\x25\xd2\xc7\xeb\x75\x2a\xc6\xf2\x05\xbb\xfe\xb9\x7e\x02\x00\x00\xff\xff\x92\x19\xf9\x3e\x23\x01\x00\x00")

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

var _metricsServerResourceReaderYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xa4\x90\x3f\x4f\xc4\x30\x0c\xc5\xf7\x7c\x0a\xeb\xf6\xf4\xc4\x86\xb2\x01\x03\xfb\x21\xb1\xbb\xa9\xb9\x33\x6d\xe3\xca\x76\x8a\xe0\xd3\xa3\x6b\xcb\x1f\x81\x74\x42\x62\xca\x4b\xe2\x9f\x9f\xde\x8b\x31\x06\x9c\xf8\x91\xd4\x58\x4a\x02\x6d\x31\x37\x58\xfd\x24\xca\x6f\xe8\x2c\xa5\xe9\xaf\xad\x61\xd9\xcf\x57\xa1\xe7\xd2\x25\xb8\x1b\xaa\x39\xe9\x41\x06\x0a\x23\x39\x76\xe8\x98\x02\x40\xc1\x91\x12\xd8\xab\x39\x8d\x69\x24\x57\xce\x16\x8d\x74\x26\x0d\x5a\x07\xb2\x14\x22\xe0\xc4\xf7\x2a\x75\xb2\x33\x11\x61\xb7\x0b\x00\x4a\x26\x55\x33\x6d\x6f\x93\x74\xb6\x88\x22\x1d\x7d\x53\x7b\x73\xf4\xed\x8e\x23\xd9\x84\x79\xf9\x9e\x49\xdb\x0d\x3d\x92\x2f\xe7\xc0\xb6\x8a\x17\xf4\x7c\x0a\xff\x0b\x79\xcb\xa5\xe3\x72\xfc\x7b\x56\x19\xe8\x40\x4f\xe7\xb1\x8f\xb4\x17\x2c\x03\xc0\xef\x5a\x2f\x1b\x58\x6d\x9f\x29\xfb\xd2\xe7\xca\x3e\x90\xce\x9c\xe9\x26\x67\xa9\xc5\x3f\xf1\x1f\x1c\x7c\xf5\x96\xa0\xaf\x2d\xc5\x75\x7f\x78\x0f\x00\x00\xff\xff\x39\x82\xcc\x46\x05\x02\x00\x00")

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

var _traefikYaml = []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xb4\x92\x4f\x6f\xd3\x40\x10\xc5\xef\xfe\x14\x23\x4b\x39\xae\x9d\xc2\xa5\xf2\x2d\xa4\x06\x2a\xa0\xa0\x24\x05\xf5\x14\xad\x77\x27\xf1\x2a\xeb\x5d\x6b\x66\x1c\x08\xa5\xdf\x1d\x6d\xdc\xbf\x52\xa5\x22\x04\xc7\x4c\x66\x7e\xef\xed\x7b\x56\x4a\x65\xba\x77\x5f\x91\xd8\xc5\x50\x41\x8b\xbe\x2b\x8c\x16\xf1\x58\xb8\x58\xee\x4f\xb2\x9d\x0b\xb6\x82\xf7\xe8\xbb\x79\xab\x49\xb2\x0e\x45\x5b\x2d\xba\xca\x00\x82\xee\xb0\x02\x21\x8d\x1b\xb7\x53\x86\xec\xed\x8c\x7b\x6d\xb0\x82\xdd\xd0\xa0\xe2\x03\x0b\x76\x19\xf7\x68\xd2\x89\x49\x90\x0a\x5a\x91\x9e\xab\xb2\x9c\x5c\x7f\xb8\x7c\x53\x2f\x2e\xea\x55\xbd\x5c\xcf\xbe\x9c\xdf\x4c\x4a\x16\x2d\xce\x94\xc7\x45\x2e\x1f\xc1\xd5\xc9\xb4\x78\x5d\x4c\xa7\x27\x85\x6c\x7f\x66\xff\xd0\xf9\xff\x73\xfd\xc4\x31\x00\xa3\x24\x1a\xc0\xd6\xc7\x46\xfb\x62\x54\x39\xc3\x8d\x1e\xbc\x2c\x70\xeb\x58\xe8\x50\x41\x3e\xb9\x5e\x5e\x2d\x57\xf5\xa7\xf5\x59\xfd\x76\x76\xf9\x71\xb5\x5e\xd4\xef\xce\x97\xab\xc5\xd5\x7a\x31\xfb\x76\x33\xc9\x33\x80\xbd\xf6\x03\xf2\x3c\x06\xc1\x20\x15\xfc\x52\x47\x2e\x35\xda\x8c\x0a\x00\x18\x74\xe3\xd1\xa6\x57\x0e\x78\x9c\xf5\x91\x84\xef\xfe\xfe\x8e\x0d\xa3\x19\x08\xef\x06\x00\xe2\xf9\xe1\xc7\xf3\x00\x3b\x0b\x21\xa6\xa7\xc6\x70\xbf\xdb\x53\xec\x50\x5a\x1c\x38\x05\x9f\x44\x2a\xc8\x4f\xa7\xa7\xaf\xf2\x67\x17\xd8\x90\xee\xb1\x82\x3c\x61\xc7\x95\x9e\xe2\xde\x59\xa4\x7b\x64\xea\x80\x02\x0a\xf2\x79\xd8\x12\xf2\x23\x5f\xfd\xd0\x78\xc7\x2d\xda\x25\xd2\xde\x19\x7c\xc1\x31\xb9\x48\x4e\x0e\x73\xaf\x99\x2f\x8e\xa5\xe7\x63\xea\xca\xf8\x81\x05\x49\x19\x72\xe2\x8c\xf6\xa3\x15\xd7\xe9\xed\x3d\x73\xfc\x4a\x72\xd2\xc1\xb4\x48\x65\xe7\x88\x22\xa1\x55\xde\x35\xa4\xe9\xa0\x6e\x6b\x1e\x2f\x25\x7a\xa4\xc7\xc9\x28\xd8\x61\x6a\x73\x7e\x2b\x30\xb3\x36\x06\xfe\x1c\xfc\xe1\x2e\x98\xd8\xa7\x8b\x48\x15\xe4\xf5\x0f\xc7\xc2\xf9\x93\xc3\x10\x2d\x2a\x8a\x1e\x8b\x87\x3c\x52\x82\x26\x06\xa1\xe8\x55\xef\x75\xc0\x17\x58\x00\xb8\xd9\xa0\x49\x95\x5c\xc4\xa5\x69\xd1\x0e\x1e\xff\x4c\xa6\xd3\x29\x9f\xbf\xe3\xff\x0e\x00\x00\xff\xff\xb1\x90\x7b\x74\x58\x04\x00\x00")

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
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
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
