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

/*
   Copyright 2019 The Go Authors. All rights reserved.
   Use of this source code is governed by a BSD-style
   license that can be found in the NOTICE.md file.
*/

package reader

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/containerd/stargz-snapshotter/cache"
	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/containerd/stargz-snapshotter/metadata"
	"github.com/containerd/stargz-snapshotter/util/testutil"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type region struct{ b, e int64 }

const (
	sampleChunkSize    = 3
	sampleMiddleOffset = sampleChunkSize / 2
	sampleData1        = "0123456789"
	lastChunkOffset1   = sampleChunkSize * (int64(len(sampleData1)) / sampleChunkSize)
)

func TestSuiteReader(t *testing.T, store metadata.Store) {
	testFileReadAt(t, store)
	testCacheVerify(t, store)
	testFailReader(t, store)
}

func testFileReadAt(t *testing.T, factory metadata.Store) {
	sizeCond := map[string]int64{
		"single_chunk": sampleChunkSize - sampleMiddleOffset,
		"multi_chunks": sampleChunkSize + sampleMiddleOffset,
	}
	innerOffsetCond := map[string]int64{
		"at_top":    0,
		"at_middle": sampleMiddleOffset,
	}
	baseOffsetCond := map[string]int64{
		"of_1st_chunk":  sampleChunkSize * 0,
		"of_2nd_chunk":  sampleChunkSize * 1,
		"of_last_chunk": lastChunkOffset1,
	}
	fileSizeCond := map[string]int64{
		"in_1_chunk_file":  sampleChunkSize * 1,
		"in_2_chunks_file": sampleChunkSize * 2,
		"in_max_size_file": int64(len(sampleData1)),
	}
	cacheCond := map[string][]region{
		"with_clean_cache": nil,
		"with_edge_filled_cache": {
			region{0, sampleChunkSize - 1},
			region{lastChunkOffset1, int64(len(sampleData1)) - 1},
		},
		"with_sparse_cache": {
			region{0, sampleChunkSize - 1},
			region{2 * sampleChunkSize, 3*sampleChunkSize - 1},
		},
	}
	for sn, size := range sizeCond {
		for in, innero := range innerOffsetCond {
			for bo, baseo := range baseOffsetCond {
				for fn, filesize := range fileSizeCond {
					for cc, cacheExcept := range cacheCond {
						t.Run(fmt.Sprintf("reading_%s_%s_%s_%s_%s", sn, in, bo, fn, cc), func(t *testing.T) {
							if filesize > int64(len(sampleData1)) {
								t.Fatal("sample file size is larger than sample data")
							}

							wantN := size
							offset := baseo + innero
							if remain := filesize - offset; remain < wantN {
								if wantN = remain; wantN < 0 {
									wantN = 0
								}
							}

							// use constant string value as a data source.
							want := strings.NewReader(sampleData1)

							// data we want to get.
							wantData := make([]byte, wantN)
							_, err := want.ReadAt(wantData, offset)
							if err != nil && err != io.EOF {
								t.Fatalf("want.ReadAt (offset=%d,size=%d): %v", offset, wantN, err)
							}

							// data we get through a file.
							f, closeFn := makeFile(t, []byte(sampleData1)[:filesize], sampleChunkSize, factory)
							defer closeFn()
							f.fr = newExceptFile(t, f.fr, cacheExcept...)
							for _, reg := range cacheExcept {
								id := genID(f.id, reg.b, reg.e-reg.b+1)
								w, err := f.gr.cache.Add(id)
								if err != nil {
									w.Close()
									t.Fatalf("failed to add cache %v: %v", id, err)
								}
								if _, err := w.Write([]byte(sampleData1[reg.b : reg.e+1])); err != nil {
									w.Close()
									t.Fatalf("failed to write cache %v: %v", id, err)
								}
								if err := w.Commit(); err != nil {
									w.Close()
									t.Fatalf("failed to commit cache %v: %v", id, err)
								}
								w.Close()
							}
							respData := make([]byte, size)
							n, err := f.ReadAt(respData, offset)
							if err != nil {
								t.Errorf("failed to read off=%d, size=%d, filesize=%d: %v", offset, size, filesize, err)
								return
							}
							respData = respData[:n]

							if !bytes.Equal(wantData, respData) {
								t.Errorf("off=%d, filesize=%d; read data{size=%d,data=%q}; want (size=%d,data=%q)",
									offset, filesize, len(respData), string(respData), wantN, string(wantData))
								return
							}

							// check cache has valid contents.
							cn := 0
							nr := 0
							for int64(nr) < wantN {
								chunkOffset, chunkSize, _, ok := f.fr.ChunkEntryForOffset(offset + int64(nr))
								if !ok {
									break
								}
								data := make([]byte, chunkSize)
								id := genID(f.id, chunkOffset, chunkSize)
								r, err := f.gr.cache.Get(id)
								if err != nil {
									t.Errorf("missed cache of offset=%d, size=%d: %v(got size=%d)", chunkOffset, chunkSize, err, n)
									return
								}
								defer r.Close()
								if n, err := r.ReadAt(data, 0); (err != nil && err != io.EOF) || n != int(chunkSize) {
									t.Errorf("failed to read cache of offset=%d, size=%d: %v(got size=%d)", chunkOffset, chunkSize, err, n)
									return
								}
								nr += n
								cn++
							}
						})
					}
				}
			}
		}
	}
}

func newExceptFile(t *testing.T, fr metadata.File, except ...region) metadata.File {
	er := exceptFile{fr: fr, t: t}
	er.except = map[region]bool{}
	for _, reg := range except {
		er.except[reg] = true
	}
	return &er
}

type exceptFile struct {
	fr     metadata.File
	except map[region]bool
	t      *testing.T
}

func (er *exceptFile) ReadAt(p []byte, offset int64) (int, error) {
	if er.except[region{offset, offset + int64(len(p)) - 1}] {
		er.t.Fatalf("Requested prohibited region of chunk: (%d, %d)", offset, offset+int64(len(p))-1)
	}
	return er.fr.ReadAt(p, offset)
}

func (er *exceptFile) ChunkEntryForOffset(offset int64) (off int64, size int64, dgst string, ok bool) {
	return er.fr.ChunkEntryForOffset(offset)
}

func makeFile(t *testing.T, contents []byte, chunkSize int, factory metadata.Store) (*file, func() error) {
	testName := "test"
	sr, dgst, err := testutil.BuildEStargz([]testutil.TarEntry{
		testutil.File(testName, string(contents)),
	}, testutil.WithEStargzOptions(estargz.WithChunkSize(chunkSize)))
	if err != nil {
		t.Fatalf("failed to build sample estargz")
	}
	mr, err := factory(sr)
	if err != nil {
		t.Fatalf("failed to create reader: %v", err)
	}
	vr, err := NewReader(mr, cache.NewMemoryCache(), digest.FromString(""))
	if err != nil {
		mr.Close()
		t.Fatalf("failed to make new reader: %v", err)
	}
	r, err := vr.VerifyTOC(dgst)
	if err != nil {
		vr.Close()
		t.Fatalf("failed to verify TOC: %v", err)
	}
	tid, _, err := r.Metadata().GetChild(r.Metadata().RootID(), testName)
	if err != nil {
		vr.Close()
		t.Fatalf("failed to get %q: %v", testName, err)
	}
	ra, err := r.OpenFile(tid)
	if err != nil {
		vr.Close()
		t.Fatalf("Failed to open testing file: %v", err)
	}
	f, ok := ra.(*file)
	if !ok {
		vr.Close()
		t.Fatalf("invalid type of file %q", tid)
	}
	return f, vr.Close
}

func testCacheVerify(t *testing.T, factory metadata.Store) {
	sr, tocDgst, err := testutil.BuildEStargz([]testutil.TarEntry{
		testutil.File("a", sampleData1+"a"),
		testutil.File("b", sampleData1+"b"),
	}, testutil.WithEStargzOptions(estargz.WithChunkSize(sampleChunkSize)))
	if err != nil {
		t.Fatalf("failed to build sample estargz")
	}
	for _, skipVerify := range [2]bool{true, false} {
		for _, invalidChunkBeforeVerify := range [2]bool{true, false} {
			for _, invalidChunkAfterVerify := range [2]bool{true, false} {
				name := fmt.Sprintf("test_cache_verify_%v_%v_%v",
					skipVerify, invalidChunkBeforeVerify, invalidChunkAfterVerify)
				t.Run(name, func(t *testing.T) {

					// Determine the expected behaviour
					var wantVerifyFail, wantCacheFail, wantCacheFail2 bool
					if skipVerify {
						// always no error if verification is disabled
						wantVerifyFail, wantCacheFail, wantCacheFail2 = false, false, false
					} else if invalidChunkBeforeVerify {
						// errors occurred before verifying TOC must be reported via VerifyTOC()
						wantVerifyFail = true
					} else if invalidChunkAfterVerify {
						// errors occurred after verifying TOC must be reported via Cache()
						wantVerifyFail, wantCacheFail, wantCacheFail2 = false, true, true
					} else {
						// otherwise no verification error
						wantVerifyFail, wantCacheFail, wantCacheFail2 = false, false, false
					}

					// Prepare reader
					verifier := &failIDVerifier{}
					mr, err := factory(sr)
					if err != nil {
						t.Fatalf("failed to prepare reader %v", err)
					}
					defer mr.Close()
					vr, err := NewReader(mr, cache.NewMemoryCache(), digest.FromString(""))
					if err != nil {
						t.Fatalf("failed to make new reader: %v", err)
					}
					if verifier != nil {
						vr.verifier = verifier.verifier
						vr.r.verifier = verifier.verifier
					}

					off2id, id2path, err := prepareMap(vr.Metadata(), vr.Metadata().RootID(), "")
					if err != nil || off2id == nil || id2path == nil {
						t.Fatalf("failed to prepare offset map %v, off2id = %+v, id2path = %+v", err, off2id, id2path)
					}

					// Perform Cache() before verification
					// 1. Either of "a" or "b" is read and verified
					// 2. VerifyTOC/SkipVerify is called
					// 3. Another entry ("a" or "b") is called
					verifyDone := make(chan struct{})
					var firstEntryCalled bool
					var eg errgroup.Group
					eg.Go(func() error {
						return vr.Cache(WithFilter(func(off int64) bool {
							id, ok := off2id[off]
							if !ok {
								t.Fatalf("no ID is assigned to offset %d", off)
							}
							name, ok := id2path[id]
							if !ok {
								t.Fatalf("no name is assigned to id %d", id)
							}
							if name == "a" || name == "b" {
								if !firstEntryCalled {
									firstEntryCalled = true
									if invalidChunkBeforeVerify {
										verifier.registerFails([]uint32{id})
									}
									return true
								}
								<-verifyDone
								if invalidChunkAfterVerify {
									verifier.registerFails([]uint32{id})
								}
								return true
							}
							return false
						}))
					})
					time.Sleep(10 * time.Millisecond)

					// Perform verification
					if skipVerify {
						vr.SkipVerify()
					} else {
						_, err = vr.VerifyTOC(tocDgst)
					}
					if checkErr := checkError(wantVerifyFail, err); checkErr != nil {
						t.Errorf("verify: %v", checkErr)
						return
					}
					if err != nil {
						return
					}
					close(verifyDone)

					// Check the result of Cache()
					if checkErr := checkError(wantCacheFail, eg.Wait()); checkErr != nil {
						t.Errorf("cache: %v", checkErr)
						return
					}

					// Call Cache() again and check the result
					if checkErr := checkError(wantCacheFail2, vr.Cache()); checkErr != nil {
						t.Errorf("cache(2): %v", checkErr)
						return
					}
				})
			}
		}
	}
}

type failIDVerifier struct {
	fails   []uint32
	failsMu sync.Mutex
}

func (f *failIDVerifier) registerFails(fails []uint32) {
	f.failsMu.Lock()
	defer f.failsMu.Unlock()
	f.fails = fails

}

func (f *failIDVerifier) verifier(id uint32, chunkDigest string) (digest.Verifier, error) {
	f.failsMu.Lock()
	defer f.failsMu.Unlock()
	success := true
	for _, n := range f.fails {
		if n == id {
			success = false
			break
		}
	}
	return &testVerifier{success}, nil
}

type testVerifier struct {
	success bool
}

func (bv *testVerifier) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func (bv *testVerifier) Verified() bool {
	return bv.success
}

func checkError(wantFail bool, err error) error {
	if wantFail && err == nil {
		return fmt.Errorf("wanted to fail but succeeded")
	} else if !wantFail && err != nil {
		return errors.Wrapf(err, "wanted to succeed verification but failed")
	}
	return nil
}

func prepareMap(mr metadata.Reader, id uint32, p string) (off2id map[int64]uint32, id2path map[uint32]string, _ error) {
	attr, err := mr.GetAttr(id)
	if err != nil {
		return nil, nil, err
	}
	id2path = map[uint32]string{id: p}
	off2id = make(map[int64]uint32)
	if attr.Mode.IsRegular() {
		off, err := mr.GetOffset(id)
		if err != nil {
			return nil, nil, err
		}
		off2id[off] = id
	}
	var retErr error
	mr.ForeachChild(id, func(name string, id uint32, mode os.FileMode) bool {
		o2i, i2p, err := prepareMap(mr, id, path.Join(p, name))
		if err != nil {
			retErr = err
			return false
		}
		for k, v := range o2i {
			off2id[k] = v
		}
		for k, v := range i2p {
			id2path[k] = v
		}
		return true
	})
	if retErr != nil {
		return nil, nil, retErr
	}
	return off2id, id2path, nil
}

func testFailReader(t *testing.T, factory metadata.Store) {
	testFileName := "test"
	stargzFile, tocDigest, err := testutil.BuildEStargz([]testutil.TarEntry{
		testutil.File(testFileName, sampleData1),
	}, testutil.WithEStargzOptions(estargz.WithChunkSize(sampleChunkSize)))
	if err != nil {
		t.Fatalf("failed to build sample estargz")
	}

	for _, rs := range []bool{true, false} {
		for _, vs := range []bool{true, false} {
			br := &breakReaderAt{
				ReaderAt: stargzFile,
				success:  true,
			}
			bev := &testChunkVerifier{true}
			mcache := cache.NewMemoryCache()
			mr, err := factory(io.NewSectionReader(br, 0, stargzFile.Size()))
			if err != nil {
				t.Fatalf("failed to prepare metadata reader")
			}
			defer mr.Close()
			vr, err := NewReader(mr, mcache, digest.FromString(""))
			if err != nil {
				t.Fatalf("failed to make new reader: %v", err)
			}
			defer vr.Close()
			vr.verifier = bev.verifier
			vr.r.verifier = bev.verifier
			gr, err := vr.VerifyTOC(tocDigest)
			if err != nil {
				t.Fatalf("failed to verify TOC: %v", err)
			}

			notexist := uint32(0)
			found := false
			for i := uint32(0); i < 1000000; i++ {
				if _, err := gr.Metadata().GetAttr(i); err != nil {
					notexist, found = i, true
					break
				}
			}
			if !found {
				t.Fatalf("free ID not found")
			}

			// tests for opening non-existing file
			_, err = gr.OpenFile(notexist)
			if err == nil {
				t.Errorf("succeeded to open file but wanted to fail")
				return
			}

			// tests failure behaviour of a file read
			tid, _, err := gr.Metadata().GetChild(gr.Metadata().RootID(), testFileName)
			if err != nil {
				t.Errorf("failed to get %q: %v", testFileName, err)
				return
			}
			fr, err := gr.OpenFile(tid)
			if err != nil {
				t.Errorf("failed to open file but wanted to succeed: %v", err)
				return
			}

			mcache.(*cache.MemoryCache).Membuf = map[string]*bytes.Buffer{}
			br.success = rs
			bev.success = vs

			// tests for reading file
			p := make([]byte, len(sampleData1))
			n, err := fr.ReadAt(p, 0)
			if rs && vs {
				if err != nil || n != len(sampleData1) || !bytes.Equal([]byte(sampleData1), p) {
					t.Errorf("failed to read data but wanted to succeed: %v", err)
					return
				}
			} else {
				if err == nil {
					t.Errorf("succeeded to read data but wanted to fail (reader:%v,verify:%v)", rs, vs)
					return
				}
			}
		}
	}
}

type breakReaderAt struct {
	io.ReaderAt
	success bool
}

func (br *breakReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if br.success {
		return br.ReaderAt.ReadAt(p, off)
	}
	return 0, fmt.Errorf("failed")
}

type testChunkVerifier struct {
	success bool
}

func (bev *testChunkVerifier) verifier(id uint32, chunkDigest string) (digest.Verifier, error) {
	return &testVerifier{bev.success}, nil
}
