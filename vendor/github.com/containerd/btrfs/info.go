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

// Info describes metadata about a btrfs subvolume.
type Info struct {
	ID         uint64 // subvolume id
	ParentID   uint64 // aka ref_tree
	TopLevelID uint64 // not actually clear what this is, not set for now.
	Offset     uint64 // key offset for root
	DirID      uint64

	Generation         uint64
	OriginalGeneration uint64

	UUID         string
	ParentUUID   string
	ReceivedUUID string

	Name string
	Path string // absolute path of subvolume
	Root string // path of root mount point

	Readonly bool // true if the snaps hot is readonly, extracted from flags
}

type infosByID []Info

func (b infosByID) Len() int           { return len(b) }
func (b infosByID) Less(i, j int) bool { return b[i].ID < b[j].ID }
func (b infosByID) Swap(i, j int)      { b[i], b[j] = b[j], b[i] }
