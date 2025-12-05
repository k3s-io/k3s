package bindata

import (
	"embed"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// Bindata is a wrapper around embed.FS that allows us to continue to use
// go-bindata style Asset and AssetNames functions to access the embedded FS.
type Bindata struct {
	FS     *embed.FS
	Prefix string
}

func (b Bindata) Asset(name string) ([]byte, error) {
	return b.FS.ReadFile(path.Join(b.Prefix, name))
}

func (b Bindata) AssetNames() []string {
	var assets []string
	fs.WalkDir(b.FS, ".", func(path string, entry fs.DirEntry, err error) error {
		// do not list hidden files - there is a .empty file checked in as a
		// placeholder for files that are generated at build time to satisy
		// `go vet`, but these should not be include when listing assets.
		if n := entry.Name(); entry.Type().IsRegular() && !strings.HasPrefix(n, ".") && !strings.HasPrefix(n, "_") {
			assets = append(assets, strings.TrimPrefix(path, b.Prefix))
		}
		return nil
	})
	sort.Strings(assets)
	return assets
}
