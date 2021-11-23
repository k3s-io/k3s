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
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/containerd/stargz-snapshotter/estargz/zstdchunked"
	"github.com/containerd/stargz-snapshotter/util/testutil"
	"github.com/hashicorp/go-multierror"
	"github.com/klauspost/compress/zstd"
)

var allowedPrefix = [4]string{"", "./", "/", "../"}

type compression interface {
	estargz.Compressor
	Decompressor
}

var srcCompressions = map[string]compression{
	"zstd-fastest":            zstdCompressionWithLevel(zstd.SpeedFastest),
	"zstd-default":            zstdCompressionWithLevel(zstd.SpeedDefault),
	"zstd-bettercompression":  zstdCompressionWithLevel(zstd.SpeedBetterCompression),
	"gzip-nocompression":      gzipCompressionWithLevel(gzip.NoCompression),
	"gzip-bestspeed":          gzipCompressionWithLevel(gzip.BestSpeed),
	"gzip-bestcompression":    gzipCompressionWithLevel(gzip.BestCompression),
	"gzip-defaultcompression": gzipCompressionWithLevel(gzip.DefaultCompression),
	"gzip-huffmanonly":        gzipCompressionWithLevel(gzip.HuffmanOnly),
}

type zstdCompression struct {
	*zstdchunked.Compressor
	*zstdchunked.Decompressor
}

func zstdCompressionWithLevel(compressionLevel zstd.EncoderLevel) compression {
	return &zstdCompression{&zstdchunked.Compressor{CompressionLevel: compressionLevel}, &zstdchunked.Decompressor{}}
}

type gzipCompression struct {
	*estargz.GzipCompressor
	*estargz.GzipDecompressor
}

func gzipCompressionWithLevel(compressionLevel int) compression {
	return gzipCompression{estargz.NewGzipCompressorWithLevel(compressionLevel), &estargz.GzipDecompressor{}}
}

type ReaderFactory func(sr *io.SectionReader, opts ...Option) (r TestableReader, err error)

type TestableReader interface {
	Reader
	NumOfNodes() (i int, _ error)
}

// TestReader tests Reader returns correct file metadata.
func TestReader(t *testing.T, factory ReaderFactory) {
	sampleTime := time.Now().Truncate(time.Second)
	sampleText := "qwer" + "tyui" + "opas" + "dfgh" + "jk"
	tests := []struct {
		name      string
		chunkSize int
		in        []testutil.TarEntry
		want      []check
	}{
		{
			name: "empty",
			in:   []testutil.TarEntry{},
			want: []check{
				numOfNodes(2), // root dir + prefetch landmark
			},
		},
		{
			name: "files",
			in: []testutil.TarEntry{
				testutil.File("foo", "foofoo", testutil.WithFileMode(0644|os.ModeSetuid)),
				testutil.Dir("bar/"),
				testutil.File("bar/baz.txt", "bazbazbaz", testutil.WithFileOwner(1000, 1000)),
				testutil.File("xxx.txt", "xxxxx", testutil.WithFileModTime(sampleTime)),
				testutil.File("y.txt", "", testutil.WithFileXattrs(map[string]string{"testkey": "testval"})),
			},
			want: []check{
				numOfNodes(7), // root dir + prefetch landmark + 1 dir + 4 files
				hasFile("foo", "foofoo", 6),
				hasMode("foo", 0644|os.ModeSetuid),
				hasFile("bar/baz.txt", "bazbazbaz", 9),
				hasOwner("bar/baz.txt", 1000, 1000),
				hasFile("xxx.txt", "xxxxx", 5),
				hasModTime("xxx.txt", sampleTime),
				hasFile("y.txt", "", 0),
				hasXattrs("y.txt", map[string]string{"testkey": "testval"}),
			},
		},
		{
			name: "dirs",
			in: []testutil.TarEntry{
				testutil.Dir("foo/", testutil.WithDirMode(os.ModeDir|0600|os.ModeSticky)),
				testutil.Dir("foo/bar/", testutil.WithDirOwner(1000, 1000)),
				testutil.File("foo/bar/baz.txt", "testtest"),
				testutil.File("foo/bar/xxxx", "x"),
				testutil.File("foo/bar/yyy", "yyy"),
				testutil.Dir("foo/a/", testutil.WithDirModTime(sampleTime)),
				testutil.Dir("foo/a/1/", testutil.WithDirXattrs(map[string]string{"testkey": "testval"})),
				testutil.File("foo/a/1/2", "1111111111"),
			},
			want: []check{
				numOfNodes(10), // root dir + prefetch landmark + 4 dirs + 4 files
				hasDirChildren("foo", "bar", "a"),
				hasDirChildren("foo/bar", "baz.txt", "xxxx", "yyy"),
				hasDirChildren("foo/a", "1"),
				hasDirChildren("foo/a/1", "2"),
				hasMode("foo", os.ModeDir|0600|os.ModeSticky),
				hasOwner("foo/bar", 1000, 1000),
				hasModTime("foo/a", sampleTime),
				hasXattrs("foo/a/1", map[string]string{"testkey": "testval"}),
				hasFile("foo/bar/baz.txt", "testtest", 8),
				hasFile("foo/bar/xxxx", "x", 1),
				hasFile("foo/bar/yyy", "yyy", 3),
				hasFile("foo/a/1/2", "1111111111", 10),
			},
		},
		{
			name: "hardlinks",
			in: []testutil.TarEntry{
				testutil.File("foo", "foofoo", testutil.WithFileOwner(1000, 1000)),
				testutil.Dir("bar/"),
				testutil.Link("bar/foolink", "foo"),
				testutil.Link("bar/foolink2", "bar/foolink"),
				testutil.Dir("bar/1/"),
				testutil.File("bar/1/baz.txt", "testtest"),
				testutil.Link("barlink", "bar/1/baz.txt"),
				testutil.Symlink("foosym", "bar/foolink2"),
			},
			want: []check{
				numOfNodes(7), // root dir + prefetch landmark + 2 dirs + 1 flie(linked) + 1 file(linked) + 1 symlink
				hasFile("foo", "foofoo", 6),
				hasOwner("foo", 1000, 1000),
				hasFile("bar/foolink", "foofoo", 6),
				hasOwner("bar/foolink", 1000, 1000),
				hasFile("bar/foolink2", "foofoo", 6),
				hasOwner("bar/foolink2", 1000, 1000),
				hasFile("bar/1/baz.txt", "testtest", 8),
				hasFile("barlink", "testtest", 8),
				hasDirChildren("bar", "foolink", "foolink2", "1"),
				hasDirChildren("bar/1", "baz.txt"),
				sameNodes("foo", "bar/foolink", "bar/foolink2"),
				sameNodes("bar/1/baz.txt", "barlink"),
				linkName("foosym", "bar/foolink2"),
				hasNumLink("foo", 3),     // parent dir + 2 links
				hasNumLink("barlink", 2), // parent dir + 1 link
				hasNumLink("bar", 3),     // parent + "." + child's ".."
			},
		},
		{
			name: "various files",
			in: []testutil.TarEntry{
				testutil.Dir("bar/"),
				testutil.File("bar/../bar///////////////////foo", ""),
				testutil.Chardev("bar/cdev", 10, 11),
				testutil.Blockdev("bar/bdev", 100, 101),
				testutil.Fifo("bar/fifo"),
			},
			want: []check{
				numOfNodes(7), // root dir + prefetch landmark + 1 file + 1 dir + 1 cdev + 1 bdev + 1 fifo
				hasFile("bar/foo", "", 0),
				hasChardev("bar/cdev", 10, 11),
				hasBlockdev("bar/bdev", 100, 101),
				hasFifo("bar/fifo"),
			},
		},
		{
			name:      "chunks",
			chunkSize: 4,
			in: []testutil.TarEntry{
				testutil.Dir("foo/"),
				testutil.File("foo/small", sampleText[:2]),
				testutil.File("foo/large", sampleText),
			},
			want: []check{
				numOfNodes(5), // root dir + prefetch landmark + 1 dir + 2 files
				numOfChunks("foo/large", 1+(len(sampleText)/4)),
				hasFileContentsOffset("foo/small", 0, sampleText[:2]),
				hasFileContentsOffset("foo/large", 0, sampleText[0:]),
				hasFileContentsOffset("foo/large", 1, sampleText[1:]),
				hasFileContentsOffset("foo/large", 2, sampleText[2:]),
				hasFileContentsOffset("foo/large", 3, sampleText[3:]),
				hasFileContentsOffset("foo/large", 4, sampleText[4:]),
				hasFileContentsOffset("foo/large", 5, sampleText[5:]),
				hasFileContentsOffset("foo/large", 6, sampleText[6:]),
				hasFileContentsOffset("foo/large", 7, sampleText[7:]),
				hasFileContentsOffset("foo/large", 8, sampleText[8:]),
				hasFileContentsOffset("foo/large", 9, sampleText[9:]),
				hasFileContentsOffset("foo/large", 10, sampleText[10:]),
				hasFileContentsOffset("foo/large", 11, sampleText[11:]),
				hasFileContentsOffset("foo/large", 12, sampleText[12:]),
				hasFileContentsOffset("foo/large", int64(len(sampleText)-1), ""),
			},
		},
	}
	for _, tt := range tests {
		for _, prefix := range allowedPrefix {
			prefix := prefix
			for srcCompresionName, srcCompression := range srcCompressions {
				srcCompression := srcCompression
				t.Run(tt.name+"-"+srcCompresionName, func(t *testing.T) {
					opts := []testutil.BuildEStargzOption{
						testutil.WithBuildTarOptions(testutil.WithPrefix(prefix)),
						testutil.WithEStargzOptions(estargz.WithCompression(srcCompression)),
					}
					if tt.chunkSize > 0 {
						opts = append(opts, testutil.WithEStargzOptions(estargz.WithChunkSize(tt.chunkSize)))
					}
					esgz, _, err := testutil.BuildEStargz(tt.in, opts...)
					if err != nil {
						t.Fatalf("failed to build sample eStargz: %v", err)
					}

					telemetry, checkCalled := newCalledTelemetry()
					r, err := factory(esgz,
						WithDecompressors(new(zstdchunked.Decompressor)), WithTelemetry(telemetry))
					if err != nil {
						t.Fatalf("failed to create new reader: %v", err)
					}
					defer r.Close()
					t.Logf("vvvvv Node tree vvvvv")
					t.Logf("[%d] ROOT", r.RootID())
					dumpNodes(t, r, r.RootID(), 1)
					t.Logf("^^^^^^^^^^^^^^^^^^^^^")
					for _, want := range tt.want {
						want(t, r)
					}
					if err := checkCalled(); err != nil {
						t.Errorf("telemetry failure: %v", err)
					}
				})
			}
		}
	}
}

func newCalledTelemetry() (telemetry *Telemetry, check func() error) {
	var getFooterLatencyCalled bool
	var getTocLatencyCalled bool
	var deserializeTocLatencyCalled bool
	return &Telemetry{
			func(time.Time) { getFooterLatencyCalled = true },
			func(time.Time) { getTocLatencyCalled = true },
			func(time.Time) { deserializeTocLatencyCalled = true },
		}, func() error {
			var allErr error
			if !getFooterLatencyCalled {
				allErr = multierror.Append(allErr, fmt.Errorf("metrics GetFooterLatency isn't called"))
			}
			if !getTocLatencyCalled {
				allErr = multierror.Append(allErr, fmt.Errorf("metrics GetTocLatency isn't called"))
			}
			if !deserializeTocLatencyCalled {
				allErr = multierror.Append(allErr, fmt.Errorf("metrics DeserializeTocLatency isn't called"))
			}
			return allErr
		}
}

func dumpNodes(t *testing.T, r TestableReader, id uint32, level int) {
	if err := r.ForeachChild(id, func(name string, id uint32, mode os.FileMode) bool {
		ind := ""
		for i := 0; i < level; i++ {
			ind += " "
		}
		t.Logf("%v+- [%d] %q : %v", ind, id, name, mode)
		dumpNodes(t, r, id, level+1)
		return true
	}); err != nil {
		t.Errorf("failed to dump nodes %v", err)
	}
}

type check func(*testing.T, TestableReader)

func numOfNodes(want int) check {
	return func(t *testing.T, r TestableReader) {
		i, err := r.NumOfNodes()
		if err != nil {
			t.Errorf("num of nodes: %v", err)
		}
		if want != i {
			t.Errorf("unexpected num of nodes %d; want %d", i, want)
		}
	}
}

func numOfChunks(name string, num int) check {
	return func(t *testing.T, r TestableReader) {
		nr, ok := r.(interface {
			NumOfChunks(id uint32) (i int, _ error)
		})
		if !ok {
			return // skip
		}
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("failed to lookup %q: %v", name, err)
			return
		}
		i, err := nr.NumOfChunks(id)
		if err != nil {
			t.Errorf("failed to get num of chunks of %q: %v", name, err)
			return
		}
		if i != num {
			t.Errorf("unexpected num of chunk of %q : %d want %d", name, i, num)
		}
	}
}

func sameNodes(n string, nodes ...string) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, n)
		if err != nil {
			t.Errorf("failed to lookup %q: %v", n, err)
			return
		}
		for _, en := range nodes {
			eid, err := lookup(r, en)
			if err != nil {
				t.Errorf("failed to lookup %q: %v", en, err)
				return
			}
			if eid != id {
				t.Errorf("unexpected ID of %q: %d want %d", en, eid, id)
			}
		}
	}
}

func linkName(name string, linkName string) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("failed to lookup %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("failed to get attr of %q: %v", name, err)
			return
		}
		if attr.Mode&os.ModeSymlink == 0 {
			t.Errorf("%q is not a symlink: %v", name, attr.Mode)
			return
		}
		if attr.LinkName != linkName {
			t.Errorf("unexpected link name of %q : %q want %q", name, attr.LinkName, linkName)
			return
		}
	}
}

func hasNumLink(name string, numLink int) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("failed to lookup %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("failed to get attr of %q: %v", name, err)
			return
		}
		if attr.NumLink != numLink {
			t.Errorf("unexpected numLink of %q: %d want %d", name, attr.NumLink, numLink)
			return
		}
	}
}

func hasDirChildren(name string, children ...string) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("failed to lookup %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("failed to get attr of %q: %v", name, err)
			return
		}
		if !attr.Mode.IsDir() {
			t.Errorf("%q is not directory: %v", name, attr.Mode)
			return
		}
		found := map[string]struct{}{}
		if err := r.ForeachChild(id, func(name string, id uint32, mode os.FileMode) bool {
			found[name] = struct{}{}
			return true
		}); err != nil {
			t.Errorf("failed to see children %v", err)
			return
		}
		if len(found) != len(children) {
			t.Errorf("unexpected number of children of %q : %d want %d", name, len(found), len(children))
		}
		for _, want := range children {
			if _, ok := found[want]; !ok {
				t.Errorf("expected child %q not found in %q", want, name)
			}
		}
	}
}

func hasChardev(name string, maj, min int) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find chardev %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of chardev %q: %v", name, err)
			return
		}
		if attr.Mode&os.ModeDevice == 0 || attr.Mode&os.ModeCharDevice == 0 {
			t.Errorf("file %q is not a chardev: %v", name, attr.Mode)
			return
		}
		if attr.DevMajor != maj || attr.DevMinor != min {
			t.Errorf("unexpected major/minor of chardev %q: %d/%d want %d/%d", name, attr.DevMajor, attr.DevMinor, maj, min)
			return
		}
	}
}

func hasBlockdev(name string, maj, min int) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find blockdev %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of blockdev %q: %v", name, err)
			return
		}
		if attr.Mode&os.ModeDevice == 0 || attr.Mode&os.ModeCharDevice != 0 {
			t.Errorf("file %q is not a blockdev: %v", name, attr.Mode)
			return
		}
		if attr.DevMajor != maj || attr.DevMinor != min {
			t.Errorf("unexpected major/minor of blockdev %q: %d/%d want %d/%d", name, attr.DevMajor, attr.DevMinor, maj, min)
			return
		}
	}
}

func hasFifo(name string) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find blockdev %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of blockdev %q: %v", name, err)
			return
		}
		if attr.Mode&os.ModeNamedPipe == 0 {
			t.Errorf("file %q is not a fifo: %v", name, attr.Mode)
			return
		}
	}
}

func hasFile(name, content string, size int64) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find file %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of file %q: %v", name, err)
			return
		}
		if !attr.Mode.IsRegular() {
			t.Errorf("file %q is not a regular file: %v", name, attr.Mode)
			return
		}
		sr, err := r.OpenFile(id)
		if err != nil {
			t.Errorf("cannot open file %q: %v", name, err)
			return
		}
		data, err := ioutil.ReadAll(io.NewSectionReader(sr, 0, attr.Size))
		if err != nil {
			t.Errorf("cannot read file %q: %v", name, err)
			return
		}
		if attr.Size != size {
			t.Errorf("unexpected size of file %q : %d (%q) want %d (%q)", name, attr.Size, string(data), size, content)
			return
		}
		if string(data) != content {
			t.Errorf("unexpected content of %q: %q want %q", name, string(data), content)
			return
		}
	}
}

func hasFileContentsOffset(name string, off int64, contents string) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("failed to lookup %q: %v", name, err)
			return
		}
		fr, err := r.OpenFile(id)
		if err != nil {
			t.Errorf("failed to open file %q: %v", name, err)
			return
		}
		buf := make([]byte, len(contents))
		n, err := fr.ReadAt(buf, off)
		if err != nil && err != io.EOF {
			t.Errorf("failed to read file %q (off:%d, want:%q): %v", name, off, contents, err)
			return
		}
		if n != len(contents) {
			t.Errorf("failed to read contents %q (off:%d, want:%q) got %q", name, off, contents, string(buf))
			return
		}
	}
}

func hasMode(name string, mode os.FileMode) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find file %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of file %q: %v", name, err)
			return
		}
		if attr.Mode != mode {
			t.Errorf("unexpected mode of %q: %v want %v", name, attr.Mode, mode)
			return
		}
	}
}

func hasOwner(name string, uid, gid int) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find file %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of file %q: %v", name, err)
			return
		}
		if attr.UID != uid || attr.GID != gid {
			t.Errorf("unexpected owner of %q: (%d:%d) want (%d:%d)", name, attr.UID, attr.GID, uid, gid)
			return
		}
	}
}

func hasModTime(name string, modTime time.Time) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find file %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of file %q: %v", name, err)
			return
		}
		attrModTime := attr.ModTime
		if attrModTime.Before(modTime) || attrModTime.After(modTime) {
			t.Errorf("unexpected time of %q: %v; want %v", name, attrModTime, modTime)
			return
		}
	}
}

func hasXattrs(name string, xattrs map[string]string) check {
	return func(t *testing.T, r TestableReader) {
		id, err := lookup(r, name)
		if err != nil {
			t.Errorf("cannot find file %q: %v", name, err)
			return
		}
		attr, err := r.GetAttr(id)
		if err != nil {
			t.Errorf("cannot get attr of file %q: %v", name, err)
			return
		}
		if len(attr.Xattrs) != len(xattrs) {
			t.Errorf("unexpected size of xattr of %q: %d want %d", name, len(attr.Xattrs), len(xattrs))
			return
		}
		for k, v := range attr.Xattrs {
			if xattrs[k] != string(v) {
				t.Errorf("unexpected xattr of %q: %q=%q want %q=%q", name, k, string(v), k, xattrs[k])
			}
		}
	}
}

func lookup(r TestableReader, name string) (uint32, error) {
	name = strings.TrimPrefix(path.Clean("/"+name), "/")
	if name == "" {
		return r.RootID(), nil
	}
	dir, base := filepath.Split(name)
	pid, err := lookup(r, dir)
	if err != nil {
		return 0, err
	}
	id, _, err := r.GetChild(pid, base)
	return id, err
}
