package manifests

import (
	"embed"
	"io/fs"
	"sort"
)

//go:embed *
var manifestFS embed.FS

// List returns all manifests file paths, sorted.
func List() ([]string, error) {
	var out []string

	err := fs.WalkDir(manifestFS, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		// manifestFS also contains embed.go but, it's not a manifest so skipping that
		if path == "embed.go" {
			return nil
		}

		out = append(out, path)
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Strings(out)
	return out, nil
}

func ReadContent(name string) ([]byte, error) {
	return manifestFS.ReadFile(name)
}
