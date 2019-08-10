// Code generated for package main by go-bindata DO NOT EDIT.
// sources:
// in/a/test.asset
// in/b/test.asset
// in/c/test.asset
// in/test.asset

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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

func (fi bindataFileInfo) Name() string {
	return fi.name
}
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}
func (fi bindataFileInfo) IsDir() bool {
	return false
}
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _inATestAsset = []byte(`// sample file
`)

func inATestAssetBytes() ([]byte, error) {
	return _inATestAsset, nil
}

func inATestAsset() (*asset, error) {
	bytes, err := inATestAssetBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "in/a/test.asset", size: 15, mode: os.FileMode(436), modTime: time.Unix(1445582844, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _inBTestAsset = []byte(`// sample file
`)

func inBTestAssetBytes() ([]byte, error) {
	return _inBTestAsset, nil
}

func inBTestAsset() (*asset, error) {
	bytes, err := inBTestAssetBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "in/b/test.asset", size: 15, mode: os.FileMode(436), modTime: time.Unix(1445582844, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _inCTestAsset = []byte(`// sample file
`)

func inCTestAssetBytes() ([]byte, error) {
	return _inCTestAsset, nil
}

func inCTestAsset() (*asset, error) {
	bytes, err := inCTestAssetBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "in/c/test.asset", size: 15, mode: os.FileMode(436), modTime: time.Unix(1445582844, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _inTestAsset = []byte(`// sample file
`)

func inTestAssetBytes() ([]byte, error) {
	return _inTestAsset, nil
}

func inTestAsset() (*asset, error) {
	bytes, err := inTestAssetBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "in/test.asset", size: 15, mode: os.FileMode(436), modTime: time.Unix(1445582844, 0)}
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
	"in/a/test.asset": inATestAsset,
	"in/b/test.asset": inBTestAsset,
	"in/c/test.asset": inCTestAsset,
	"in/test.asset":   inTestAsset,
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
	"in": &bintree{nil, map[string]*bintree{
		"a": &bintree{nil, map[string]*bintree{
			"test.asset": &bintree{inATestAsset, map[string]*bintree{}},
		}},
		"b": &bintree{nil, map[string]*bintree{
			"test.asset": &bintree{inBTestAsset, map[string]*bintree{}},
		}},
		"c": &bintree{nil, map[string]*bintree{
			"test.asset": &bintree{inCTestAsset, map[string]*bintree{}},
		}},
		"test.asset": &bintree{inTestAsset, map[string]*bintree{}},
	}},
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
