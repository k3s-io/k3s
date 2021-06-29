package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Cleanup(path string) error {
	return filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		fmt.Println(path)

		if err != nil {
			return err
		}

		if strings.Contains(path, "vendor") {
			return filepath.SkipDir
		}

		if strings.HasPrefix(info.Name(), "zz_generated") {
			fmt.Println("Removing", path)
			if err := os.Remove(path); err != nil {
				return err
			}
		}

		return nil
	})
}
