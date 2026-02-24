package fixtures

import (
	"archive/zip"
	"bytes"
	_ "embed"
	"io/fs"

	"github.com/spf13/afero"
	"github.com/spf13/afero/zipfs"
)

//go:embed etcd/member.zip
var member []byte

var ETCD fs.FS

func init() {
	if r, err := zip.NewReader(bytes.NewReader(member), int64(len(member))); err == nil {
		ETCD = afero.NewIOFS(zipfs.New(r))
	}
}
