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

package memory

import (
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"time"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/containerd/stargz-snapshotter/metadata"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

type reader struct {
	r      *estargz.Reader
	rootID uint32

	idMap     map[uint32]*estargz.TOCEntry
	idOfEntry map[*estargz.TOCEntry]uint32
	mu        sync.Mutex

	curID   uint32
	curIDMu sync.Mutex

	opts *metadata.Options
}

func (r *reader) nextID() (uint32, error) {
	r.curIDMu.Lock()
	defer r.curIDMu.Unlock()
	if r.curID == math.MaxUint32 {
		return 0, fmt.Errorf("sequence id too large")
	}
	r.curID++
	return r.curID, nil
}

func NewReader(sr *io.SectionReader, opts ...metadata.Option) (metadata.Reader, error) {
	var rOpts metadata.Options
	for _, o := range opts {
		if err := o(&rOpts); err != nil {
			return nil, errors.Wrapf(err, "failed to apply option")
		}
	}

	telemetry := &estargz.Telemetry{}
	if rOpts.Telemetry != nil {
		telemetry.GetFooterLatency = estargz.MeasureLatencyHook(rOpts.Telemetry.GetFooterLatency)
		telemetry.GetTocLatency = estargz.MeasureLatencyHook(rOpts.Telemetry.GetTocLatency)
		telemetry.DeserializeTocLatency = estargz.MeasureLatencyHook(rOpts.Telemetry.DeserializeTocLatency)
	}
	var decompressors []estargz.Decompressor
	for _, d := range rOpts.Decompressors {
		decompressors = append(decompressors, d)
	}
	er, err := estargz.Open(sr,
		estargz.WithTOCOffset(rOpts.TOCOffset),
		estargz.WithTelemetry(telemetry),
		estargz.WithDecompressors(decompressors...),
	)
	if err != nil {
		return nil, err
	}

	root, ok := er.Lookup("")
	if !ok {
		return nil, fmt.Errorf("failed to get root node")
	}
	r := &reader{r: er, idMap: make(map[uint32]*estargz.TOCEntry), idOfEntry: make(map[*estargz.TOCEntry]uint32), opts: &rOpts}
	rootID, err := r.initID(root)
	if err != nil {
		return nil, err
	}
	r.rootID = rootID
	return r, nil
}

func (r *reader) initID(e *estargz.TOCEntry) (id uint32, err error) {
	var ok bool
	r.mu.Lock()
	id, ok = r.idOfEntry[e]
	if !ok {
		id, err = r.nextID()
		if err != nil {
			return 0, err
		}
		r.idMap[id] = e
		r.idOfEntry[e] = id
	}
	r.mu.Unlock()

	e.ForeachChild(func(_ string, ent *estargz.TOCEntry) bool {
		if ent.Type == "hardlink" {
			var ok bool
			ent, ok = r.r.Lookup(ent.Name)
			if !ok {
				return false
			}
		}
		_, err = r.initID(ent)
		return err == nil
	})
	return id, err
}

func (r *reader) RootID() uint32 {
	return r.rootID
}

func (r *reader) TOCDigest() digest.Digest {
	return r.r.TOCDigest()
}

func (r *reader) GetOffset(id uint32) (offset int64, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.idMap[id]
	if !ok {
		return 0, fmt.Errorf("entry %d not found", id)
	}
	return e.Offset, nil
}

func (r *reader) GetAttr(id uint32) (attr metadata.Attr, err error) {
	r.mu.Lock()
	e, ok := r.idMap[id]
	r.mu.Unlock()
	if !ok {
		err = fmt.Errorf("entry %d not found", id)
		return
	}
	// TODO: zero copy
	attrFromTOCEntry(e, &attr)
	return
}

func (r *reader) GetChild(pid uint32, base string) (id uint32, attr metadata.Attr, err error) {
	r.mu.Lock()
	e, ok := r.idMap[pid]
	r.mu.Unlock()
	if !ok {
		err = fmt.Errorf("parent entry %d not found", pid)
		return
	}
	child, ok := e.LookupChild(base)
	if !ok {
		err = fmt.Errorf("child %q of entry %d not found", base, pid)
		return
	}
	if child.Type == "hardlink" {
		child, ok = r.r.Lookup(child.Name)
		if !ok {
			err = fmt.Errorf("child %q ()hardlink of entry %d not found", base, pid)
			return
		}
	}
	cid, ok := r.idOfEntry[child]
	if !ok {
		err = fmt.Errorf("id of entry %q not found", base)
		return
	}
	// TODO: zero copy
	attrFromTOCEntry(child, &attr)
	return cid, attr, nil
}

func (r *reader) ForeachChild(id uint32, f func(name string, id uint32, mode os.FileMode) bool) error {
	r.mu.Lock()
	e, ok := r.idMap[id]
	r.mu.Unlock()
	if !ok {
		return fmt.Errorf("parent entry %d not found", id)
	}
	var err error
	e.ForeachChild(func(baseName string, ent *estargz.TOCEntry) bool {
		if ent.Type == "hardlink" {
			var ok bool
			ent, ok = r.r.Lookup(ent.Name)
			if !ok {
				return false
			}
		}
		r.mu.Lock()
		id, ok := r.idOfEntry[ent]
		r.mu.Unlock()
		if !ok {
			err = fmt.Errorf("id of child entry %q not found", baseName)
			return false
		}
		return f(baseName, id, ent.Stat().Mode())
	})
	return err
}

func (r *reader) OpenFile(id uint32) (metadata.File, error) {
	r.mu.Lock()
	e, ok := r.idMap[id]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("entry %d not found", id)
	}
	sr, err := r.r.OpenFile(e.Name)
	if err != nil {
		return nil, err
	}
	return &file{r, e, sr}, nil
}

func (r *reader) Clone(sr *io.SectionReader) (metadata.Reader, error) {
	return NewReader(sr,
		metadata.WithTOCOffset(r.opts.TOCOffset),
		metadata.WithTelemetry(r.opts.Telemetry),
		metadata.WithDecompressors(r.opts.Decompressors...),
	)
}

func (r *reader) Close() error {
	return nil
}

type file struct {
	r  *reader
	e  *estargz.TOCEntry
	sr *io.SectionReader
}

func (r *file) ChunkEntryForOffset(offset int64) (off int64, size int64, dgst string, ok bool) {
	e, ok := r.r.r.ChunkEntryForOffset(r.e.Name, offset)
	if !ok {
		return 0, 0, "", false
	}
	dgst = e.Digest
	if e.ChunkDigest != "" {
		// NOTE* "reg" also can contain ChunkDigest (e.g. when "reg" is the first entry of
		// chunked file)
		dgst = e.ChunkDigest
	}
	return e.ChunkOffset, e.ChunkSize, dgst, true
}

func (r *file) ReadAt(p []byte, off int64) (n int, err error) {
	return r.sr.ReadAt(p, off)
}

func (r *reader) NumOfNodes() (i int, _ error) {
	return len(r.idMap), nil
}

// TODO: share it with db pkg
func attrFromTOCEntry(src *estargz.TOCEntry, dst *metadata.Attr) *metadata.Attr {
	dst.Size = src.Size
	dst.ModTime, _ = time.Parse(time.RFC3339, src.ModTime3339)
	dst.LinkName = src.LinkName
	dst.Mode = src.Stat().Mode()
	dst.UID = src.UID
	dst.GID = src.GID
	dst.DevMajor = src.DevMajor
	dst.DevMinor = src.DevMinor
	dst.Xattrs = src.Xattrs
	dst.NumLink = src.NumLink
	return dst
}
