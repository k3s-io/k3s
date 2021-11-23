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

package metadata

import (
	"io"
	"os"
	"time"

	"github.com/containerd/stargz-snapshotter/estargz"
	digest "github.com/opencontainers/go-digest"
)

// Attr reprensents the attributes of a node.
type Attr struct {
	// Size, for regular files, is the logical size of the file.
	Size int64

	// ModTime is the modification time of the node.
	ModTime time.Time

	// LinkName, for symlinks, is the link target.
	LinkName string

	// Mode is the permission and mode bits.
	Mode os.FileMode

	// UID is the user ID of the owner.
	UID int

	// GID is the group ID of the owner.
	GID int

	// DevMajor is the major device number for device.
	DevMajor int

	// DevMinor is the major device number for device.
	DevMinor int

	// Xattrs are the extended attribute for the node.
	Xattrs map[string][]byte

	// NumLink is the number of names pointing to this node.
	NumLink int
}

// Store reads the provided eStargz blob and creates a metadata reader.
type Store func(sr *io.SectionReader, opts ...Option) (Reader, error)

// Reader provides access to file metadata of a blob.
type Reader interface {
	RootID() uint32
	TOCDigest() digest.Digest

	GetOffset(id uint32) (offset int64, err error)
	GetAttr(id uint32) (attr Attr, err error)
	GetChild(pid uint32, base string) (id uint32, attr Attr, err error)
	ForeachChild(id uint32, f func(name string, id uint32, mode os.FileMode) bool) error
	OpenFile(id uint32) (File, error)

	Clone(sr *io.SectionReader) (Reader, error)
	Close() error
}

type File interface {
	ChunkEntryForOffset(offset int64) (off int64, size int64, dgst string, ok bool)
	ReadAt(p []byte, off int64) (n int, err error)
}

type Decompressor interface {
	estargz.Decompressor

	// DecompressTOC decompresses the passed blob and returns a reader of TOC JSON.
	DecompressTOC(io.Reader) (tocJSON io.ReadCloser, err error)
}

type Options struct {
	TOCOffset     int64
	Telemetry     *Telemetry
	Decompressors []Decompressor
}

// Option is an option to configure the behaviour of reader.
type Option func(o *Options) error

// WithTOCOffset option specifies the offset of TOC
func WithTOCOffset(tocOffset int64) Option {
	return func(o *Options) error {
		o.TOCOffset = tocOffset
		return nil
	}
}

// WithTelemetry option specifies the telemetry hooks
func WithTelemetry(telemetry *Telemetry) Option {
	return func(o *Options) error {
		o.Telemetry = telemetry
		return nil
	}
}

// WithDecompressors option specifies decompressors to use.
// Default is gzip-based decompressor.
func WithDecompressors(decompressors ...Decompressor) Option {
	return func(o *Options) error {
		o.Decompressors = decompressors
		return nil
	}
}

// A func which takes start time and records the diff
type MeasureLatencyHook func(time.Time)

// A struct which defines telemetry hooks. By implementing these hooks you should be able to record
// the latency metrics of the respective steps of estargz open operation.
type Telemetry struct {
	GetFooterLatency      MeasureLatencyHook // measure time to get stargz footer (in milliseconds)
	GetTocLatency         MeasureLatencyHook // measure time to GET TOC JSON (in milliseconds)
	DeserializeTocLatency MeasureLatencyHook // measure time to deserialize TOC JSON (in milliseconds)
}
