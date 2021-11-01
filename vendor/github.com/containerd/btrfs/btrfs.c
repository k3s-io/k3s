/*
  Copyright The containerd Authors

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

#include <stddef.h>
#include <linux/magic.h>
#include <btrfs/ioctl.h>
#include <btrfs/ctree.h>

#include "btrfs.h"

void unpack_root_item(struct gosafe_btrfs_root_item* dst, struct btrfs_root_item* src) {
	memcpy(dst->uuid, src->uuid, BTRFS_UUID_SIZE);
	memcpy(dst->parent_uuid, src->parent_uuid, BTRFS_UUID_SIZE);
	memcpy(dst->received_uuid, src->received_uuid, BTRFS_UUID_SIZE);
	dst->gen = btrfs_root_generation(src);
	dst->ogen = btrfs_root_otransid(src);
	dst->flags = btrfs_root_flags(src);
}

/* unpack_root_ref(struct gosafe_btrfs_root_ref* dst, struct btrfs_root_ref* src) { */
