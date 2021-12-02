package copy

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"
)

const (
	// tmpPermissionForDirectory makes the destination directory writable,
	// so that stuff can be copied recursively even if any original directory is NOT writable.
	// See https://github.com/otiai10/copy/pull/9 for more information.
	tmpPermissionForDirectory = os.FileMode(0755)
)

type timespec struct {
	Mtime time.Time
	Atime time.Time
	Ctime time.Time
}

// Copy copies src to dest, doesn't matter if src is a directory or a file.
func Copy(src, dest string, opt ...Options) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	return switchboard(src, dest, info, assure(src, dest, opt...))
}

// switchboard switches proper copy functions regarding file type, etc...
// If there would be anything else here, add a case to this switchboard.
func switchboard(src, dest string, info os.FileInfo, opt Options) (err error) {
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		err = onsymlink(src, dest, info, opt)
	case info.IsDir():
		err = dcopy(src, dest, info, opt)
	case info.Mode()&os.ModeNamedPipe != 0:
		err = pcopy(dest, info)
	default:
		err = fcopy(src, dest, info, opt)
	}

	return err
}

// copyNextOrSkip decide if this src should be copied or not.
// Because this "copy" could be called recursively,
// "info" MUST be given here, NOT nil.
func copyNextOrSkip(src, dest string, info os.FileInfo, opt Options) error {
	skip, err := opt.Skip(src)
	if err != nil {
		return err
	}
	if skip {
		return nil
	}
	return switchboard(src, dest, info, opt)
}

// fcopy is for just a file,
// with considering existence of parent directory
// and file permission.
func fcopy(src, dest string, info os.FileInfo, opt Options) (err error) {

	if err = os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return
	}

	f, err := os.Create(dest)
	if err != nil {
		return
	}
	defer fclose(f, &err)

	if err = os.Chmod(f.Name(), info.Mode()|opt.AddPermission); err != nil {
		return
	}

	s, err := os.Open(src)
	if err != nil {
		return
	}
	defer fclose(s, &err)

	var buf []byte = nil
	var w io.Writer = f
	// var r io.Reader = s
	if opt.CopyBufferSize != 0 {
		buf = make([]byte, opt.CopyBufferSize)
		// Disable using `ReadFrom` by io.CopyBuffer.
		// See https://github.com/otiai10/copy/pull/60#discussion_r627320811 for more details.
		w = struct{ io.Writer }{f}
		// r = struct{ io.Reader }{s}
	}
	if _, err = io.CopyBuffer(w, s, buf); err != nil {
		return err
	}

	if opt.Sync {
		err = f.Sync()
	}

	if opt.PreserveTimes {
		return preserveTimes(info, dest)
	}

	return
}

// dcopy is for a directory,
// with scanning contents inside the directory
// and pass everything to "copy" recursively.
func dcopy(srcdir, destdir string, info os.FileInfo, opt Options) (err error) {

	_, err = os.Stat(destdir)
	if err == nil && opt.OnDirExists != nil && destdir != opt.intent.dest {
		switch opt.OnDirExists(srcdir, destdir) {
		case Replace:
			if err := os.RemoveAll(destdir); err != nil {
				return err
			}
		case Untouchable:
			return nil
		} // case "Merge" is default behaviour. Go through.
	} else if err != nil && !os.IsNotExist(err) {
		return err // Unwelcome error type...!
	}

	originalMode := info.Mode()

	// Make dest dir with 0755 so that everything writable.
	if err = os.MkdirAll(destdir, tmpPermissionForDirectory); err != nil {
		return
	}
	// Recover dir mode with original one.
	defer chmod(destdir, originalMode|opt.AddPermission, &err)

	contents, err := ioutil.ReadDir(srcdir)
	if err != nil {
		return
	}

	for _, content := range contents {
		cs, cd := filepath.Join(srcdir, content.Name()), filepath.Join(destdir, content.Name())

		if err = copyNextOrSkip(cs, cd, content, opt); err != nil {
			// If any error, exit immediately
			return
		}
	}

	if opt.PreserveTimes {
		return preserveTimes(info, destdir)
	}

	return
}

func onsymlink(src, dest string, info os.FileInfo, opt Options) error {
	switch opt.OnSymlink(src) {
	case Shallow:
		return lcopy(src, dest)
	case Deep:
		orig, err := os.Readlink(src)
		if err != nil {
			return err
		}
		info, err = os.Lstat(orig)
		if err != nil {
			return err
		}
		return copyNextOrSkip(orig, dest, info, opt)
	case Skip:
		fallthrough
	default:
		return nil // do nothing
	}
}

// lcopy is for a symlink,
// with just creating a new symlink by replicating src symlink.
func lcopy(src, dest string) error {
	src, err := os.Readlink(src)
	if err != nil {
		return err
	}
	return os.Symlink(src, dest)
}

// fclose ANYHOW closes file,
// with asiging error raised during Close,
// BUT respecting the error already reported.
func fclose(f *os.File, reported *error) {
	if err := f.Close(); *reported == nil {
		*reported = err
	}
}

// chmod ANYHOW changes file mode,
// with asiging error raised during Chmod,
// BUT respecting the error already reported.
func chmod(dir string, mode os.FileMode, reported *error) {
	if err := os.Chmod(dir, mode); *reported == nil {
		*reported = err
	}
}

// assure Options struct, should be called only once.
// All optional values MUST NOT BE nil/zero after assured.
func assure(src, dest string, opts ...Options) Options {
	defopt := getDefaultOptions(src, dest)
	if len(opts) == 0 {
		return defopt
	}
	if opts[0].OnSymlink == nil {
		opts[0].OnSymlink = defopt.OnSymlink
	}
	if opts[0].Skip == nil {
		opts[0].Skip = defopt.Skip
	}
	opts[0].intent.src = defopt.intent.src
	opts[0].intent.dest = defopt.intent.dest
	return opts[0]
}
