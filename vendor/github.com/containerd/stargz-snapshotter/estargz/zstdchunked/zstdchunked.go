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

package zstdchunked

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"sync"

	"github.com/containerd/stargz-snapshotter/estargz"
	"github.com/klauspost/compress/zstd"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

const (
	// ManifestChecksumAnnotation is an annotation that contains the compressed TOC Digset
	ManifestChecksumAnnotation = "io.containers.zstd-chunked.manifest-checksum"

	// ManifestPositionAnnotation is an annotation that contains the offset to the TOC.
	ManifestPositionAnnotation = "io.containers.zstd-chunked.manifest-position"

	// FooterSize is the size of the footer
	FooterSize = 40

	manifestTypeCRFS = 1
)

var (
	skippableFrameMagic   = []byte{0x50, 0x2a, 0x4d, 0x18}
	zstdFrameMagic        = []byte{0x28, 0xb5, 0x2f, 0xfd}
	zstdChunkedFrameMagic = []byte{0x47, 0x6e, 0x55, 0x6c, 0x49, 0x6e, 0x55, 0x78}
)

type Decompressor struct{}

func (zz *Decompressor) Reader(r io.Reader) (io.ReadCloser, error) {
	decoder, err := zstd.NewReader(r)
	if err != nil {
		return nil, err
	}
	return &zstdReadCloser{decoder}, nil
}

func (zz *Decompressor) ParseTOC(r io.Reader) (toc *estargz.JTOC, tocDgst digest.Digest, err error) {
	zr, err := zstd.NewReader(r)
	if err != nil {
		return nil, "", err
	}
	defer zr.Close()
	dgstr := digest.Canonical.Digester()
	toc = new(estargz.JTOC)
	if err := json.NewDecoder(io.TeeReader(zr, dgstr.Hash())).Decode(&toc); err != nil {
		return nil, "", errors.Wrap(err, "error decoding TOC JSON")
	}
	return toc, dgstr.Digest(), nil
}

func (zz *Decompressor) ParseFooter(p []byte) (blobPayloadSize, tocOffset, tocSize int64, err error) {
	offset := binary.LittleEndian.Uint64(p[0:8])
	compressedLength := binary.LittleEndian.Uint64(p[8:16])
	if !bytes.Equal(zstdChunkedFrameMagic, p[32:40]) {
		return 0, 0, 0, fmt.Errorf("invalid magic number")
	}
	// 8 is the size of the zstd skippable frame header + the frame size (see WriteTOCAndFooter)
	return int64(offset - 8), int64(offset), int64(compressedLength), nil
}

func (zz *Decompressor) FooterSize() int64 {
	return FooterSize
}

type zstdReadCloser struct{ *zstd.Decoder }

func (z *zstdReadCloser) Close() error {
	z.Decoder.Close()
	return nil
}

type Compressor struct {
	CompressionLevel zstd.EncoderLevel
	Metadata         map[string]string

	pool sync.Pool
}

func (zc *Compressor) Writer(w io.Writer) (io.WriteCloser, error) {
	if wc := zc.pool.Get(); wc != nil {
		ec := wc.(*zstd.Encoder)
		ec.Reset(w)
		return &poolEncoder{ec, zc}, nil
	}
	ec, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zc.CompressionLevel), zstd.WithLowerEncoderMem(true))
	if err != nil {
		return nil, err
	}
	return &poolEncoder{ec, zc}, nil
}

type poolEncoder struct {
	*zstd.Encoder
	zc *Compressor
}

func (w *poolEncoder) Close() error {
	if err := w.Encoder.Close(); err != nil {
		return err
	}
	w.zc.pool.Put(w.Encoder)
	return nil
}

func (zc *Compressor) WriteTOCAndFooter(w io.Writer, off int64, toc *estargz.JTOC, diffHash hash.Hash) (digest.Digest, error) {
	tocJSON, err := json.MarshalIndent(toc, "", "\t")
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	encoder, err := zstd.NewWriter(buf, zstd.WithEncoderLevel(zc.CompressionLevel))
	if err != nil {
		return "", err
	}
	if _, err := encoder.Write(tocJSON); err != nil {
		return "", err
	}
	if err := encoder.Close(); err != nil {
		return "", err
	}
	compressedTOC := buf.Bytes()
	_, err = io.Copy(w, bytes.NewReader(appendSkippableFrameMagic(compressedTOC)))

	// 8 is the size of the zstd skippable frame header + the frame size
	tocOff := uint64(off) + 8
	if _, err := w.Write(appendSkippableFrameMagic(
		zstdFooterBytes(tocOff, uint64(len(tocJSON)), uint64(len(compressedTOC)))),
	); err != nil {
		return "", err
	}

	if zc.Metadata != nil {
		zc.Metadata[ManifestChecksumAnnotation] = digest.FromBytes(compressedTOC).String()
		zc.Metadata[ManifestPositionAnnotation] = fmt.Sprintf("%d:%d:%d:%d",
			tocOff, len(compressedTOC), len(tocJSON), manifestTypeCRFS)
	}

	return digest.FromBytes(tocJSON), err
}

// zstdFooterBytes returns the 40 bytes footer.
func zstdFooterBytes(tocOff, tocRawSize, tocCompressedSize uint64) []byte {
	footer := make([]byte, FooterSize)
	binary.LittleEndian.PutUint64(footer, tocOff)
	binary.LittleEndian.PutUint64(footer[8:], tocCompressedSize)
	binary.LittleEndian.PutUint64(footer[16:], tocRawSize)
	binary.LittleEndian.PutUint64(footer[24:], manifestTypeCRFS)
	copy(footer[32:40], zstdChunkedFrameMagic)
	return footer
}

func appendSkippableFrameMagic(b []byte) []byte {
	size := make([]byte, 4)
	binary.LittleEndian.PutUint32(size, uint32(len(b)))
	return append(append(skippableFrameMagic, size...), b...)
}
