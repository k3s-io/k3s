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

package btrfs

/*
#include <stddef.h>
#include <btrfs/ioctl.h>
#include "btrfs.h"

static char* get_name_btrfs_ioctl_vol_args_v2(struct btrfs_ioctl_vol_args_v2* btrfs_struct) {
	return btrfs_struct->name;
}
*/
import "C"

import (
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

// maxByteSliceSize is the smallest size that Go supports on various platforms.
// On mipsle, 1<<31-1 overflows the address space.
const maxByteSliceSize = 1 << 30

// IsSubvolume returns nil if the path is a valid subvolume. An error is
// returned if the path does not exist or the path is not a valid subvolume.
func IsSubvolume(path string) error {
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}

	if err := isFileInfoSubvol(fi); err != nil {
		return err
	}

	var statfs syscall.Statfs_t
	if err := syscall.Statfs(path, &statfs); err != nil {
		return err
	}

	return isStatfsSubvol(&statfs)
}

// SubvolID returns the subvolume ID for the provided path
func SubvolID(path string) (uint64, error) {
	fp, err := openSubvolDir(path)
	if err != nil {
		return 0, err
	}
	defer fp.Close()

	return subvolID(fp.Fd())
}

// SubvolInfo returns information about the subvolume at the provided path.
func SubvolInfo(path string) (info Info, err error) {
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return info, err
	}

	fp, err := openSubvolDir(path)
	if err != nil {
		return info, err
	}
	defer fp.Close()

	id, err := subvolID(fp.Fd())
	if err != nil {
		return info, err
	}

	subvolsByID, err := subvolMap(path)
	if err != nil {
		return info, err
	}

	if info, ok := subvolsByID[id]; ok {
		return *info, nil
	}

	return info, errors.Errorf("%q not found", path)
}

func subvolMap(path string) (map[uint64]*Info, error) {
	fp, err := openSubvolDir(path)
	if err != nil {
		return nil, err
	}
	defer fp.Close()

	var args C.struct_btrfs_ioctl_search_args

	args.key.tree_id = C.BTRFS_ROOT_TREE_OBJECTID
	args.key.min_type = C.BTRFS_ROOT_ITEM_KEY
	args.key.max_type = C.BTRFS_ROOT_BACKREF_KEY
	args.key.min_objectid = C.BTRFS_FS_TREE_OBJECTID
	args.key.max_objectid = C.BTRFS_LAST_FREE_OBJECTID
	args.key.max_offset = ^C.__u64(0)
	args.key.max_transid = ^C.__u64(0)

	subvolsByID := make(map[uint64]*Info)

	for {
		args.key.nr_items = 4096
		if err := ioctl(fp.Fd(), C.BTRFS_IOC_TREE_SEARCH, uintptr(unsafe.Pointer(&args))); err != nil {
			return nil, err
		}

		if args.key.nr_items == 0 {
			break
		}

		var (
			sh     C.struct_btrfs_ioctl_search_header
			shSize = unsafe.Sizeof(sh)
			buf    = (*[maxByteSliceSize]byte)(unsafe.Pointer(&args.buf[0]))[:C.BTRFS_SEARCH_ARGS_BUFSIZE]
		)

		for i := 0; i < int(args.key.nr_items); i++ {
			sh = (*(*C.struct_btrfs_ioctl_search_header)(unsafe.Pointer(&buf[0])))
			buf = buf[shSize:]

			info := subvolsByID[uint64(sh.objectid)]
			if info == nil {
				info = &Info{}
			}
			info.ID = uint64(sh.objectid)

			if sh._type == C.BTRFS_ROOT_BACKREF_KEY {
				rr := (*(*C.struct_btrfs_root_ref)(unsafe.Pointer(&buf[0])))

				// This branch processes the backrefs from the root object. We
				// get an entry of the objectid, with name, but the parent is
				// the offset.

				nname := C.btrfs_stack_root_ref_name_len(&rr)
				name := string(buf[C.sizeof_struct_btrfs_root_ref : C.sizeof_struct_btrfs_root_ref+uintptr(nname)])

				info.ID = uint64(sh.objectid)
				info.ParentID = uint64(sh.offset)
				info.Name = name
				info.DirID = uint64(C.btrfs_stack_root_ref_dirid(&rr))

				subvolsByID[uint64(sh.objectid)] = info
			} else if sh._type == C.BTRFS_ROOT_ITEM_KEY &&
				(sh.objectid >= C.BTRFS_ROOT_ITEM_KEY ||
					sh.objectid == C.BTRFS_FS_TREE_OBJECTID) {

				var (
					ri  = (*C.struct_btrfs_root_item)(unsafe.Pointer(&buf[0]))
					gri C.struct_gosafe_btrfs_root_item
				)

				C.unpack_root_item(&gri, ri)

				if gri.flags&C.BTRFS_ROOT_SUBVOL_RDONLY != 0 {
					info.Readonly = true
				}

				// in this case, the offset is the actual offset.
				info.Offset = uint64(sh.offset)

				info.UUID = uuidString(&gri.uuid)
				info.ParentUUID = uuidString(&gri.parent_uuid)
				info.ReceivedUUID = uuidString(&gri.received_uuid)

				info.Generation = uint64(gri.gen)
				info.OriginalGeneration = uint64(gri.ogen)

				subvolsByID[uint64(sh.objectid)] = info
			}

			args.key.min_objectid = sh.objectid
			args.key.min_offset = sh.offset
			args.key.min_type = sh._type //  this is very questionable.

			buf = buf[sh.len:]
		}

		args.key.min_offset++
		if args.key.min_offset == 0 {
			args.key.min_type++
		} else {
			continue
		}

		if args.key.min_type > C.BTRFS_ROOT_BACKREF_KEY {
			args.key.min_type = C.BTRFS_ROOT_ITEM_KEY
			args.key.min_objectid++
		} else {
			continue
		}

		if args.key.min_objectid > args.key.max_objectid {
			break
		}
	}

	mnt, err := findMountPoint(path)
	if err != nil {
		return nil, err
	}

	for _, sv := range subvolsByID {
		path := sv.Name
		parentID := sv.ParentID

		for parentID != 0 {
			parent, ok := subvolsByID[parentID]
			if !ok {
				break
			}

			parentID = parent.ParentID
			path = filepath.Join(parent.Name, path)
		}

		sv.Path = filepath.Join(mnt, path)
	}
	return subvolsByID, nil
}

// SubvolList will return the information for all subvolumes corresponding to
// the provided path.
func SubvolList(path string) ([]Info, error) {
	subvolsByID, err := subvolMap(path)
	if err != nil {
		return nil, err
	}

	subvols := make([]Info, 0, len(subvolsByID))
	for _, sv := range subvolsByID {
		subvols = append(subvols, *sv)
	}

	sort.Sort(infosByID(subvols))

	return subvols, nil
}

// SubvolCreate creates a subvolume at the provided path.
func SubvolCreate(path string) error {
	dir, name := filepath.Split(path)

	fp, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer fp.Close()

	var args C.struct_btrfs_ioctl_vol_args
	args.fd = C.__s64(fp.Fd())

	if len(name) > C.BTRFS_PATH_NAME_MAX {
		return errors.Errorf("%q too long for subvolume", name)
	}
	nameptr := (*[maxByteSliceSize]byte)(unsafe.Pointer(&args.name[0]))[:C.BTRFS_PATH_NAME_MAX:C.BTRFS_PATH_NAME_MAX]
	copy(nameptr[:C.BTRFS_PATH_NAME_MAX], []byte(name))

	if err := ioctl(fp.Fd(), C.BTRFS_IOC_SUBVOL_CREATE, uintptr(unsafe.Pointer(&args))); err != nil {
		return errors.Wrap(err, "btrfs subvolume create failed")
	}

	return nil
}

// SubvolSnapshot creates a snapshot in dst from src. If readonly is true, the
// snapshot will be readonly.
func SubvolSnapshot(dst, src string, readonly bool) error {
	dstdir, dstname := filepath.Split(dst)

	dstfp, err := openSubvolDir(dstdir)
	if err != nil {
		return errors.Wrapf(err, "opening snapshot destination subvolume failed")
	}
	defer dstfp.Close()

	srcfp, err := openSubvolDir(src)
	if err != nil {
		return errors.Wrapf(err, "opening snapshot source subvolume failed")
	}
	defer srcfp.Close()

	// dstdir is the ioctl arg, wile srcdir gets set on the args
	var args C.struct_btrfs_ioctl_vol_args_v2
	args.fd = C.__s64(srcfp.Fd())
	name := C.get_name_btrfs_ioctl_vol_args_v2(&args)

	if len(dstname) > C.BTRFS_SUBVOL_NAME_MAX {
		return errors.Errorf("%q too long for subvolume", dstname)
	}

	nameptr := (*[maxByteSliceSize]byte)(unsafe.Pointer(name))[:C.BTRFS_SUBVOL_NAME_MAX:C.BTRFS_SUBVOL_NAME_MAX]
	copy(nameptr[:C.BTRFS_SUBVOL_NAME_MAX], []byte(dstname))

	if readonly {
		args.flags |= C.BTRFS_SUBVOL_RDONLY
	}

	if err := ioctl(dstfp.Fd(), C.BTRFS_IOC_SNAP_CREATE_V2, uintptr(unsafe.Pointer(&args))); err != nil {
		return errors.Wrapf(err, "snapshot create failed")
	}

	return nil
}

// SubvolDelete deletes the subvolumes under the given path.
func SubvolDelete(path string) error {
	dir, name := filepath.Split(path)
	fp, err := openSubvolDir(dir)
	if err != nil {
		return errors.Wrapf(err, "failed opening %v", path)
	}
	defer fp.Close()

	// remove child subvolumes
	if err := filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) || p == path {
				return nil
			}

			return errors.Wrapf(err, "failed walking subvolume %v", p)
		}

		if !fi.IsDir() {
			return nil // just ignore it!
		}

		if p == path {
			return nil
		}

		if err := isFileInfoSubvol(fi); err != nil {
			return nil
		}

		if err := SubvolDelete(p); err != nil {
			return errors.Wrapf(err, "recursive delete of %v failed", p)
		}

		return filepath.SkipDir // children get walked by call above.
	}); err != nil {
		return err
	}

	var args C.struct_btrfs_ioctl_vol_args
	if len(name) > C.BTRFS_SUBVOL_NAME_MAX {
		return errors.Errorf("%q too long for subvolume", name)
	}

	nameptr := (*[maxByteSliceSize]byte)(unsafe.Pointer(&args.name[0]))[:C.BTRFS_SUBVOL_NAME_MAX:C.BTRFS_SUBVOL_NAME_MAX]
	copy(nameptr[:C.BTRFS_SUBVOL_NAME_MAX], []byte(name))

	if err := ioctl(fp.Fd(), C.BTRFS_IOC_SNAP_DESTROY, uintptr(unsafe.Pointer(&args))); err != nil {
		return errors.Wrapf(err, "failed removing subvolume %v", path)
	}

	return nil
}

func openSubvolDir(path string) (*os.File, error) {
	fp, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "opening %v as subvolume failed", path)
	}

	return fp, nil
}

func isStatfsSubvol(statfs *syscall.Statfs_t) error {
	if int64(statfs.Type) != int64(C.BTRFS_SUPER_MAGIC) {
		return errors.Errorf("not a btrfs filesystem")
	}

	return nil
}

func isFileInfoSubvol(fi os.FileInfo) error {
	if !fi.IsDir() {
		errors.Errorf("must be a directory")
	}

	stat := fi.Sys().(*syscall.Stat_t)

	if stat.Ino != C.BTRFS_FIRST_FREE_OBJECTID {
		return errors.Errorf("incorrect inode type")
	}

	return nil
}
