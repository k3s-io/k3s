package tarfile

import (
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4"
	"github.com/pkg/errors"
	"github.com/rancher/wharfie/pkg/util"
	"github.com/sirupsen/logrus"
)

var (
	ErrNotFound = errors.New("image not found")
	// This needs to be kept in sync with the decompressor list
	SupportedExtensions = []string{".tar", ".tar.lz4", ".tar.bz2", ".tbz", ".tar.gz", ".tgz", ".tar.zst", ".tzst"}
	// The zstd decoder will attempt to use up to 1GB memory for streaming operations by default,
	// which is excessive and will OOM low-memory devices.
	// NOTE: This must be at least as large as the window size used when compressing tarballs, or you
	// will see a "window size exceeded" error when decompressing. The zstd CLI tool uses 4MB by
	// default; the --long option defaults to 27 or 128M, which is still too much for a Pi3. 32MB
	// (--long=25) has been tested to work acceptably while still compressing by an additional 3-6% on
	// our datasets.
	MaxDecoderMemory = uint64(1 << 25)
)

// FindImage checks tarball files in a given directory for a copy of the referenced image. The image reference must be a Tag, not a Digest.
// The image is retrieved from the first file (ordered by name) that it is found in; there is no preference in terms of compression format.
// If the image is not found in any file in the given directory, a NotFoundError is returned.
func FindImage(imagesDir string, imageRef name.Reference) (v1.Image, error) {
	imageTag, ok := imageRef.(name.Tag)
	if !ok {
		return nil, fmt.Errorf("no local image available for %s: reference is not a tag", imageRef.Name())
	}

	if _, err := os.Stat(imagesDir); err != nil {
		if os.IsNotExist(err) {
			return nil, errors.Wrapf(ErrNotFound, "no local image available for %s: directory %s does not exist", imageTag.Name(), imagesDir)
		}
		return nil, err
	}

	logrus.Infof("Checking local image archives in %s for %s", imagesDir, imageTag.Name())

	// Walk the images dir to get a list of tar files.
	// dotfiles and files with unsupported extensions are ignored.
	files := map[string]os.FileInfo{}
	if err := filepath.Walk(imagesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		base := filepath.Base(info.Name())
		if !info.IsDir() && !strings.HasPrefix(base, ".") && util.HasSuffixI(base, SupportedExtensions...) {
			files[path] = info
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Try to find the requested tag in each file, moving on to the next if there's an error
	for fileName := range files {
		img, err := findImage(fileName, imageTag)
		if err != nil {
			logrus.Infof("Failed to find %s in %s: %v", imageTag.Name(), fileName, err)
		}
		if img != nil {
			logrus.Debugf("Found %s in %s", imageTag.Name(), fileName)
			return img, nil
		}
	}
	return nil, errors.Wrapf(ErrNotFound, "no local image available for %s: not found in any file in %s", imageTag.Name(), imagesDir)
}

// findImage returns a handle to an image in a tarfile on disk.
// If the image is not found in the file, an error is returned.
func findImage(fileName string, imageTag name.Tag) (v1.Image, error) {
	opener, err := GetOpener(fileName)
	if err != nil {
		return nil, err
	}
	return tarball.Image(opener, &imageTag)
}

// GetOpener returns a function implementing the tarball.Opener interface.
// This is required because compressed tarballs are not seekable, and the image
// reader may need to seek backwards in the file to find a required layer.
// Instead of seeking backwards, it just closes and reopens the file.
// If the file format is not supported, an error is returned.
func GetOpener(fileName string) (tarball.Opener, error) {
	var opener tarball.Opener
	switch {
	case util.HasSuffixI(fileName, ".tar"):
		opener = func() (io.ReadCloser, error) {
			return os.Open(fileName)
		}
	case util.HasSuffixI(fileName, ".tar.lz4"):
		opener = func() (io.ReadCloser, error) {
			file, err := os.Open(fileName)
			if err != nil {
				return nil, err
			}
			zr := lz4.NewReader(file)
			return SplitReadCloser(zr, file), nil
		}
	case util.HasSuffixI(fileName, ".tar.bz2", ".tbz"):
		opener = func() (io.ReadCloser, error) {
			file, err := os.Open(fileName)
			if err != nil {
				return nil, err
			}
			zr := bzip2.NewReader(file)
			return SplitReadCloser(zr, file), nil
		}
	case util.HasSuffixI(fileName, ".tar.gz", ".tgz"):
		opener = func() (io.ReadCloser, error) {
			file, err := os.Open(fileName)
			if err != nil {
				return nil, err
			}
			zr, err := gzip.NewReader(file)
			if err != nil {
				return nil, err
			}
			return MultiReadCloser(zr, file), nil
		}
	case util.HasSuffixI(fileName, "tar.zst", ".tzst"):
		opener = func() (io.ReadCloser, error) {
			file, err := os.Open(fileName)
			if err != nil {
				return nil, err
			}
			zr, err := zstd.NewReader(file, zstd.WithDecoderMaxMemory(MaxDecoderMemory))
			if err != nil {
				return nil, err
			}
			return ZstdReadCloser(zr, file), nil
		}
	default:
		return nil, fmt.Errorf("unhandled file type; supported extensions: " + strings.Join(SupportedExtensions, " "))
	}
	return opener, nil
}
