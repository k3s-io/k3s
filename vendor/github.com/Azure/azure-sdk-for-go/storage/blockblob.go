package storage

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License. See License.txt in the project root for license information.

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// BlockListType is used to filter out types of blocks in a Get Blocks List call
// for a block blob.
//
// See https://msdn.microsoft.com/en-us/library/azure/dd179400.aspx for all
// block types.
type BlockListType string

// Filters for listing blocks in block blobs
const (
	BlockListTypeAll         BlockListType = "all"
	BlockListTypeCommitted   BlockListType = "committed"
	BlockListTypeUncommitted BlockListType = "uncommitted"
)

// Maximum sizes (per REST API) for various concepts
const (
	MaxBlobBlockSize = 100 * 1024 * 1024
	MaxBlobPageSize  = 4 * 1024 * 1024
)

// BlockStatus defines states a block for a block blob can
// be in.
type BlockStatus string

// List of statuses that can be used to refer to a block in a block list
const (
	BlockStatusUncommitted BlockStatus = "Uncommitted"
	BlockStatusCommitted   BlockStatus = "Committed"
	BlockStatusLatest      BlockStatus = "Latest"
)

// Block is used to create Block entities for Put Block List
// call.
type Block struct {
	ID     string
	Status BlockStatus
}

// BlockListResponse contains the response fields from Get Block List call.
//
// See https://msdn.microsoft.com/en-us/library/azure/dd179400.aspx
type BlockListResponse struct {
	XMLName           xml.Name        `xml:"BlockList"`
	CommittedBlocks   []BlockResponse `xml:"CommittedBlocks>Block"`
	UncommittedBlocks []BlockResponse `xml:"UncommittedBlocks>Block"`
}

// BlockResponse contains the block information returned
// in the GetBlockListCall.
type BlockResponse struct {
	Name string `xml:"Name"`
	Size int64  `xml:"Size"`
}

// CreateBlockBlob initializes an empty block blob with no blocks.
//
// See CreateBlockBlobFromReader for more info on creating blobs.
//
// See https://docs.microsoft.com/en-us/rest/api/storageservices/fileservices/Put-Blob
func (b *Blob) CreateBlockBlob(options *PutBlobOptions) error {
	return b.CreateBlockBlobFromReader(nil, options)
}

// CreateBlockBlobFromReader initializes a block blob using data from
// reader. Size must be the number of bytes read from reader. To
// create an empty blob, use size==0 and reader==nil.
//
// Any headers set in blob.Properties or metadata in blob.Metadata
// will be set on the blob.
//
// The API rejects requests with size > 256 MiB (but this limit is not
// checked by the SDK). To write a larger blob, use CreateBlockBlob,
// PutBlock, and PutBlockList.
//
// To create a blob from scratch, call container.GetBlobReference() to
// get an empty blob, fill in blob.Properties and blob.Metadata as
// appropriate then call this method.
//
// See https://docs.microsoft.com/en-us/rest/api/storageservices/fileservices/Put-Blob
func (b *Blob) CreateBlockBlobFromReader(blob io.Reader, options *PutBlobOptions) error {
	params := url.Values{}
	headers := b.Container.bsc.client.getStandardHeaders()
	headers["x-ms-blob-type"] = string(BlobTypeBlock)

	headers["Content-Length"] = "0"
	var n int64
	var err error
	if blob != nil {
		type lener interface {
			Len() int
		}
		// TODO(rjeczalik): handle io.ReadSeeker, in case blob is *os.File etc.
		if l, ok := blob.(lener); ok {
			n = int64(l.Len())
		} else {
			var buf bytes.Buffer
			n, err = io.Copy(&buf, blob)
			if err != nil {
				return err
			}
			blob = &buf
		}

		headers["Content-Length"] = strconv.FormatInt(n, 10)
	}
	b.Properties.ContentLength = n

	headers = mergeHeaders(headers, headersFromStruct(b.Properties))
	headers = b.Container.bsc.client.addMetadataToHeaders(headers, b.Metadata)

	if options != nil {
		params = addTimeout(params, options.Timeout)
		headers = mergeHeaders(headers, headersFromStruct(*options))
	}
	uri := b.Container.bsc.client.getEndpoint(blobServiceName, b.buildPath(), params)

	resp, err := b.Container.bsc.client.exec(http.MethodPut, uri, headers, blob, b.Container.bsc.auth)
	if err != nil {
		return err
	}
	return b.respondCreation(resp, BlobTypeBlock)
}

// PutBlockOptions includes the options for a put block operation
type PutBlockOptions struct {
	Timeout    uint
	LeaseID    string `header:"x-ms-lease-id"`
	ContentMD5 string `header:"Content-MD5"`
	RequestID  string `header:"x-ms-client-request-id"`
}

// PutBlock saves the given data chunk to the specified block blob with
// given ID.
//
// The API rejects chunks larger than 100 MiB (but this limit is not
// checked by the SDK).
//
// See https://docs.microsoft.com/en-us/rest/api/storageservices/fileservices/Put-Block
func (b *Blob) PutBlock(blockID string, chunk []byte, options *PutBlockOptions) error {
	return b.PutBlockWithLength(blockID, uint64(len(chunk)), bytes.NewReader(chunk), options)
}

// PutBlockWithLength saves the given data stream of exactly specified size to
// the block blob with given ID. It is an alternative to PutBlocks where data
// comes as stream but the length is known in advance.
//
// The API rejects requests with size > 100 MiB (but this limit is not
// checked by the SDK).
//
// See https://docs.microsoft.com/en-us/rest/api/storageservices/fileservices/Put-Block
func (b *Blob) PutBlockWithLength(blockID string, size uint64, blob io.Reader, options *PutBlockOptions) error {
	query := url.Values{
		"comp":    {"block"},
		"blockid": {blockID},
	}
	headers := b.Container.bsc.client.getStandardHeaders()
	headers["Content-Length"] = fmt.Sprintf("%v", size)

	if options != nil {
		query = addTimeout(query, options.Timeout)
		headers = mergeHeaders(headers, headersFromStruct(*options))
	}
	uri := b.Container.bsc.client.getEndpoint(blobServiceName, b.buildPath(), query)

	resp, err := b.Container.bsc.client.exec(http.MethodPut, uri, headers, blob, b.Container.bsc.auth)
	if err != nil {
		return err
	}
	return b.respondCreation(resp, BlobTypeBlock)
}

// PutBlockFromURLOptions includes the options for a put block from URL operation
type PutBlockFromURLOptions struct {
	PutBlockOptions

	SourceContentMD5   string `header:"x-ms-source-content-md5"`
	SourceContentCRC64 string `header:"x-ms-source-content-crc64"`
}

// PutBlockFromURL copy data of exactly specified size from specified URL to
// the block blob with given ID. It is an alternative to PutBlocks where data
// comes from a remote URL and the offset and length is known in advance.
//
// The API rejects requests with size > 100 MiB (but this limit is not
// checked by the SDK).
//
// See https://docs.microsoft.com/en-us/rest/api/storageservices/put-block-from-url
func (b *Blob) PutBlockFromURL(blockID string, blobURL string, offset int64, size uint64, options *PutBlockFromURLOptions) error {
	query := url.Values{
		"comp":    {"block"},
		"blockid": {blockID},
	}
	headers := b.Container.bsc.client.getStandardHeaders()
	// The value of this header must be set to zero.
	// When the length is not zero, the operation will fail with the status code 400 (Bad Request).
	headers["Content-Length"] = "0"
	headers["x-ms-copy-source"] = blobURL
	headers["x-ms-source-range"] = fmt.Sprintf("bytes=%d-%d", offset, uint64(offset)+size-1)

	if options != nil {
		query = addTimeout(query, options.Timeout)
		headers = mergeHeaders(headers, headersFromStruct(*options))
	}
	uri := b.Container.bsc.client.getEndpoint(blobServiceName, b.buildPath(), query)

	resp, err := b.Container.bsc.client.exec(http.MethodPut, uri, headers, nil, b.Container.bsc.auth)
	if err != nil {
		return err
	}
	return b.respondCreation(resp, BlobTypeBlock)
}

// PutBlockListOptions includes the options for a put block list operation
type PutBlockListOptions struct {
	Timeout           uint
	LeaseID           string     `header:"x-ms-lease-id"`
	IfModifiedSince   *time.Time `header:"If-Modified-Since"`
	IfUnmodifiedSince *time.Time `header:"If-Unmodified-Since"`
	IfMatch           string     `header:"If-Match"`
	IfNoneMatch       string     `header:"If-None-Match"`
	RequestID         string     `header:"x-ms-client-request-id"`
}

// PutBlockList saves list of blocks to the specified block blob.
//
// See https://docs.microsoft.com/en-us/rest/api/storageservices/fileservices/Put-Block-List
func (b *Blob) PutBlockList(blocks []Block, options *PutBlockListOptions) error {
	params := url.Values{"comp": {"blocklist"}}
	blockListXML := prepareBlockListRequest(blocks)
	headers := b.Container.bsc.client.getStandardHeaders()
	headers["Content-Length"] = fmt.Sprintf("%v", len(blockListXML))
	headers = mergeHeaders(headers, headersFromStruct(b.Properties))
	headers = b.Container.bsc.client.addMetadataToHeaders(headers, b.Metadata)

	if options != nil {
		params = addTimeout(params, options.Timeout)
		headers = mergeHeaders(headers, headersFromStruct(*options))
	}
	uri := b.Container.bsc.client.getEndpoint(blobServiceName, b.buildPath(), params)

	resp, err := b.Container.bsc.client.exec(http.MethodPut, uri, headers, strings.NewReader(blockListXML), b.Container.bsc.auth)
	if err != nil {
		return err
	}
	defer drainRespBody(resp)
	return checkRespCode(resp, []int{http.StatusCreated})
}

// GetBlockListOptions includes the options for a get block list operation
type GetBlockListOptions struct {
	Timeout   uint
	Snapshot  *time.Time
	LeaseID   string `header:"x-ms-lease-id"`
	RequestID string `header:"x-ms-client-request-id"`
}

// GetBlockList retrieves list of blocks in the specified block blob.
//
// See https://docs.microsoft.com/en-us/rest/api/storageservices/fileservices/Get-Block-List
func (b *Blob) GetBlockList(blockType BlockListType, options *GetBlockListOptions) (BlockListResponse, error) {
	params := url.Values{
		"comp":          {"blocklist"},
		"blocklisttype": {string(blockType)},
	}
	headers := b.Container.bsc.client.getStandardHeaders()

	if options != nil {
		params = addTimeout(params, options.Timeout)
		params = addSnapshot(params, options.Snapshot)
		headers = mergeHeaders(headers, headersFromStruct(*options))
	}
	uri := b.Container.bsc.client.getEndpoint(blobServiceName, b.buildPath(), params)

	var out BlockListResponse
	resp, err := b.Container.bsc.client.exec(http.MethodGet, uri, headers, nil, b.Container.bsc.auth)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()

	err = xmlUnmarshal(resp.Body, &out)
	return out, err
}
