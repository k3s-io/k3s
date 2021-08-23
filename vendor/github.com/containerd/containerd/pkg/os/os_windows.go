/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package os

import (
	"os"
	"strings"
	"sync"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// openPath takes a path, opens it, and returns the resulting handle.
// It works for both file and directory paths.
//
// We are not able to use builtin Go functionality for opening a directory path:
// - os.Open on a directory returns a os.File where Fd() is a search handle from FindFirstFile.
// - syscall.Open does not provide a way to specify FILE_FLAG_BACKUP_SEMANTICS, which is needed to
//   open a directory.
// We could use os.Open if the path is a file, but it's easier to just use the same code for both.
// Therefore, we call windows.CreateFile directly.
func openPath(path string) (windows.Handle, error) {
	u16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	h, err := windows.CreateFile(
		u16,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS, // Needed to open a directory handle.
		0)
	if err != nil {
		return 0, &os.PathError{
			Op:   "CreateFile",
			Path: path,
			Err:  err,
		}
	}
	return h, nil
}

// GetFinalPathNameByHandle flags.
//nolint:golint
const (
	cFILE_NAME_OPENED = 0x8

	cVOLUME_NAME_DOS  = 0x0
	cVOLUME_NAME_GUID = 0x1
)

var pool = sync.Pool{
	New: func() interface{} {
		// Size of buffer chosen somewhat arbitrarily to accommodate a large number of path strings.
		// MAX_PATH (260) + size of volume GUID prefix (49) + null terminator = 310.
		b := make([]uint16, 310)
		return &b
	},
}

// getFinalPathNameByHandle facilitates calling the Windows API GetFinalPathNameByHandle
// with the given handle and flags. It transparently takes care of creating a buffer of the
// correct size for the call.
func getFinalPathNameByHandle(h windows.Handle, flags uint32) (string, error) {
	b := *(pool.Get().(*[]uint16))
	defer func() { pool.Put(&b) }()
	for {
		n, err := windows.GetFinalPathNameByHandle(h, &b[0], uint32(len(b)), flags)
		if err != nil {
			return "", err
		}
		// If the buffer wasn't large enough, n will be the total size needed (including null terminator).
		// Resize and try again.
		if n > uint32(len(b)) {
			b = make([]uint16, n)
			continue
		}
		// If the buffer is large enough, n will be the size not including the null terminator.
		// Convert to a Go string and return.
		return string(utf16.Decode(b[:n])), nil
	}
}

// resolvePath implements path resolution for Windows. It attempts to return the "real" path to the
// file or directory represented by the given path.
// The resolution works by using the Windows API GetFinalPathNameByHandle, which takes a handle and
// returns the final path to that file.
func resolvePath(path string) (string, error) {
	h, err := openPath(path)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(h)

	// We use the Windows API GetFinalPathNameByHandle to handle path resolution. GetFinalPathNameByHandle
	// returns a resolved path name for a file or directory. The returned path can be in several different
	// formats, based on the flags passed. There are several goals behind the design here:
	// - Do as little manual path manipulation as possible. Since Windows path formatting can be quite
	//   complex, we try to just let the Windows APIs handle that for us.
	// - Retain as much compatibility with existing Go path functions as we can. In particular, we try to
	//   ensure paths returned from resolvePath can be passed to EvalSymlinks.
	//
	// First, we query for the VOLUME_NAME_GUID path of the file. This will return a path in the form
	// "\\?\Volume{8a25748f-cf34-4ac6-9ee2-c89400e886db}\dir\file.txt". If the path is a UNC share
	// (e.g. "\\server\share\dir\file.txt"), then the VOLUME_NAME_GUID query will fail with ERROR_PATH_NOT_FOUND.
	// In this case, we will next try a VOLUME_NAME_DOS query. This query will return a path for a UNC share
	// in the form "\\?\UNC\server\share\dir\file.txt". This path will work with most functions, but EvalSymlinks
	// fails on it. Therefore, we rewrite the path to the form "\\server\share\dir\file.txt" before returning it.
	// This path rewrite may not be valid in all cases (see the notes in the next paragraph), but those should
	// be very rare edge cases, and this case wouldn't have worked with EvalSymlinks anyways.
	//
	// The "\\?\" prefix indicates that no path parsing or normalization should be performed by Windows.
	// Instead the path is passed directly to the object manager. The lack of parsing means that "." and ".." are
	// interpreted literally and "\"" must be used as a path separator. Additionally, because normalization is
	// not done, certain paths can only be represented in this format. For instance, "\\?\C:\foo." (with a trailing .)
	// cannot be written as "C:\foo.", because path normalization will remove the trailing ".".
	//
	// We use FILE_NAME_OPENED instead of FILE_NAME_NORMALIZED, as FILE_NAME_NORMALIZED can fail on some
	// UNC paths based on access restrictions. The additional normalization done is also quite minimal in
	// most cases.
	//
	// Querying for VOLUME_NAME_DOS first instead of VOLUME_NAME_GUID would yield a "nicer looking" path in some cases.
	// For instance, it could return "\\?\C:\dir\file.txt" instead of "\\?\Volume{8a25748f-cf34-4ac6-9ee2-c89400e886db}\dir\file.txt".
	// However, we query for VOLUME_NAME_GUID first for two reasons:
	// - The volume GUID path is more stable. A volume's mount point can change when it is remounted, but its
	//   volume GUID should not change.
	// - If the volume is mounted at a non-drive letter path (e.g. mounted to "C:\mnt"), then VOLUME_NAME_DOS
	//   will return the mount path. EvalSymlinks fails on a path like this due to a bug.
	//
	// References:
	// - GetFinalPathNameByHandle: https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfinalpathnamebyhandlea
	// - Naming Files, Paths, and Namespaces: https://docs.microsoft.com/en-us/windows/win32/fileio/naming-a-file
	// - Naming a Volume: https://docs.microsoft.com/en-us/windows/win32/fileio/naming-a-volume

	rPath, err := getFinalPathNameByHandle(h, cFILE_NAME_OPENED|cVOLUME_NAME_GUID)
	if err == windows.ERROR_PATH_NOT_FOUND {
		// ERROR_PATH_NOT_FOUND is returned from the VOLUME_NAME_GUID query if the path is a
		// network share (UNC path). In this case, query for the DOS name instead, then translate
		// the returned path to make it more palatable to other path functions.
		rPath, err = getFinalPathNameByHandle(h, cFILE_NAME_OPENED|cVOLUME_NAME_DOS)
		if err != nil {
			return "", err
		}
		if strings.HasPrefix(rPath, `\\?\UNC\`) {
			// Convert \\?\UNC\server\share -> \\server\share. The \\?\UNC syntax does not work with
			// some Go filepath functions such as EvalSymlinks. In the future if other components
			// move away from EvalSymlinks and use GetFinalPathNameByHandle instead, we could remove
			// this path munging.
			rPath = `\\` + rPath[len(`\\?\UNC\`):]
		}
	} else if err != nil {
		return "", err
	}
	return rPath, nil
}

// ResolveSymbolicLink will follow any symbolic links
func (RealOS) ResolveSymbolicLink(path string) (string, error) {
	// filepath.EvalSymlinks does not work very well on Windows, so instead we resolve the path
	// via resolvePath which uses GetFinalPathNameByHandle. This returns either a path prefixed with `\\?\`,
	// or a remote share path in the form \\server\share. These should work with most Go and Windows APIs.
	return resolvePath(path)
}
