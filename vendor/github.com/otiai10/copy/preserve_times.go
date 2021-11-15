package copy

import "os"

func preserveTimes(srcinfo os.FileInfo, dest string) error {
	spec := getTimeSpec(srcinfo)
	if err := os.Chtimes(dest, spec.Atime, spec.Mtime); err != nil {
		return err
	}
	return nil
}
