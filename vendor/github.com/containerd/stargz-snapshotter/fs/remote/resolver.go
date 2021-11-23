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

package remote

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/log"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/stargz-snapshotter/cache"
	"github.com/containerd/stargz-snapshotter/fs/config"
	commonmetrics "github.com/containerd/stargz-snapshotter/fs/metrics/common"
	"github.com/containerd/stargz-snapshotter/fs/source"
	"github.com/hashicorp/go-multierror"
	rhttp "github.com/hashicorp/go-retryablehttp"
	digest "github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	defaultChunkSize        = 50000
	defaultValidIntervalSec = 60
	defaultFetchTimeoutSec  = 300

	defaultMaxRetries  = 5
	defaultMinWaitMSec = 30
	defaultMaxWaitMSec = 300000
)

func NewResolver(cfg config.BlobConfig, handlers map[string]Handler) *Resolver {
	if cfg.ChunkSize == 0 { // zero means "use default chunk size"
		cfg.ChunkSize = defaultChunkSize
	}
	if cfg.ValidInterval == 0 { // zero means "use default interval"
		cfg.ValidInterval = defaultValidIntervalSec
	}
	if cfg.CheckAlways {
		cfg.ValidInterval = 0
	}
	if cfg.FetchTimeoutSec == 0 {
		cfg.FetchTimeoutSec = defaultFetchTimeoutSec
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = defaultMaxRetries
	}
	if cfg.MinWaitMSec == 0 {
		cfg.MinWaitMSec = defaultMinWaitMSec
	}
	if cfg.MaxWaitMSec == 0 {
		cfg.MaxWaitMSec = defaultMaxWaitMSec
	}

	return &Resolver{
		blobConfig: cfg,
		handlers:   handlers,
	}
}

type Resolver struct {
	blobConfig config.BlobConfig
	handlers   map[string]Handler
}

type fetcher interface {
	fetch(ctx context.Context, rs []region, retry bool) (multipartReadCloser, error)
	check() error
	genID(reg region) string
}

func (r *Resolver) Resolve(ctx context.Context, hosts source.RegistryHosts, refspec reference.Spec, desc ocispec.Descriptor, blobCache cache.BlobCache) (Blob, error) {
	f, size, err := r.resolveFetcher(ctx, hosts, refspec, desc)
	if err != nil {
		return nil, err
	}
	blobConfig := &r.blobConfig
	return makeBlob(f,
		size,
		blobConfig.ChunkSize,
		blobConfig.PrefetchChunkSize,
		blobCache,
		time.Now(),
		time.Duration(blobConfig.ValidInterval)*time.Second,
		r,
		time.Duration(blobConfig.FetchTimeoutSec)*time.Second), nil
}

func (r *Resolver) resolveFetcher(ctx context.Context, hosts source.RegistryHosts, refspec reference.Spec, desc ocispec.Descriptor) (f fetcher, size int64, err error) {
	blobConfig := &r.blobConfig
	fc := &fetcherConfig{
		hosts:       hosts,
		refspec:     refspec,
		desc:        desc,
		maxRetries:  blobConfig.MaxRetries,
		minWaitMSec: time.Duration(blobConfig.MinWaitMSec) * time.Millisecond,
		maxWaitMSec: time.Duration(blobConfig.MaxWaitMSec) * time.Millisecond,
	}
	var handlersErr error
	for name, p := range r.handlers {
		// TODO: allow to configure the selection of readers based on the hostname in refspec
		r, size, err := p.Handle(ctx, desc)
		if err != nil {
			handlersErr = multierror.Append(handlersErr, err)
			continue
		}
		log.G(ctx).WithField("handler name", name).WithField("ref", refspec.String()).WithField("digest", desc.Digest).
			Debugf("contents is provided by a handler")
		return &remoteFetcher{r}, size, nil
	}

	log.G(ctx).WithError(handlersErr).WithField("ref", refspec.String()).WithField("digest", desc.Digest).Debugf("using default handler")
	hf, size, err := newHTTPFetcher(ctx, fc)
	if err != nil {
		return nil, 0, err
	}
	if blobConfig.ForceSingleRangeMode {
		hf.singleRangeMode()
	}
	return hf, size, err
}

type fetcherConfig struct {
	hosts       source.RegistryHosts
	refspec     reference.Spec
	desc        ocispec.Descriptor
	maxRetries  int
	minWaitMSec time.Duration
	maxWaitMSec time.Duration
}

func jitter(duration time.Duration) time.Duration {
	return time.Duration(rand.Int63n(int64(duration)) + int64(duration))
}

// backoffStrategy extends retryablehttp's DefaultBackoff to add a random jitter to avoid overwhelming the repository
// when it comes back online
// DefaultBackoff either tries to parse the 'Retry-After' header of the response; or, it uses an exponential backoff
// 2 ^ numAttempts, limited by max
func backoffStrategy(min, max time.Duration, attemptNum int, resp *http.Response) time.Duration {
	delayTime := rhttp.DefaultBackoff(min, max, attemptNum, resp)
	return jitter(delayTime)
}

// retryStrategy extends retryablehttp's DefaultRetryPolicy to log the error and response when retrying
// DefaultRetryPolicy retries whenever err is non-nil (except for some url errors) or if returned
// status code is 429 or 5xx (except 501)
func retryStrategy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	retry, err2 := rhttp.DefaultRetryPolicy(ctx, resp, err)
	if retry {
		log.G(ctx).WithFields(logrus.Fields{
			"error":    err,
			"response": resp,
		}).Infof("Retrying request")
	}
	return retry, err2
}

func newHTTPFetcher(ctx context.Context, fc *fetcherConfig) (*httpFetcher, int64, error) {
	reghosts, err := fc.hosts(fc.refspec)
	if err != nil {
		return nil, 0, err
	}
	desc := fc.desc
	if desc.Digest.String() == "" {
		return nil, 0, fmt.Errorf("Digest is mandatory in layer descriptor")
	}
	digest := desc.Digest
	pullScope, err := repositoryScope(fc.refspec, false)
	if err != nil {
		return nil, 0, err
	}

	// Try to create fetcher until succeeded
	rErr := fmt.Errorf("failed to resolve")
	for _, host := range reghosts {
		if host.Host == "" || strings.Contains(host.Host, "/") {
			rErr = errors.Wrapf(rErr, "invalid destination (host %q, ref:%q, digest:%q)",
				host.Host, fc.refspec, digest)
			continue // Try another

		}

		// Prepare transport with authorization functionality
		tr := host.Client.Transport

		if rt, ok := tr.(*rhttp.RoundTripper); ok {
			rt.Client.RetryMax = fc.maxRetries
			rt.Client.RetryWaitMin = fc.minWaitMSec
			rt.Client.RetryWaitMax = fc.maxWaitMSec
			rt.Client.Backoff = backoffStrategy
			rt.Client.CheckRetry = retryStrategy
		}

		timeout := host.Client.Timeout
		if host.Authorizer != nil {
			tr = &transport{
				inner: tr,
				auth:  host.Authorizer,
				scope: pullScope,
			}
		}

		// Resolve redirection and get blob URL
		blobURL := fmt.Sprintf("%s://%s/%s/blobs/%s",
			host.Scheme,
			path.Join(host.Host, host.Path),
			strings.TrimPrefix(fc.refspec.Locator, fc.refspec.Hostname()+"/"),
			digest)
		url, err := redirect(ctx, blobURL, tr, timeout)
		if err != nil {
			rErr = errors.Wrapf(rErr, "failed to redirect (host %q, ref:%q, digest:%q): %v",
				host.Host, fc.refspec, digest, err)
			continue // Try another
		}

		// Get size information
		// TODO: we should try to use the Size field in the descriptor here.
		start := time.Now() // start time before getting layer header
		size, err := getSize(ctx, url, tr, timeout)
		commonmetrics.MeasureLatencyInMilliseconds(commonmetrics.StargzHeaderGet, digest, start) // time to get layer header
		if err != nil {
			rErr = errors.Wrapf(rErr, "failed to get size (host %q, ref:%q, digest:%q): %v",
				host.Host, fc.refspec, digest, err)
			continue // Try another
		}

		// Hit one destination
		return &httpFetcher{
			url:     url,
			tr:      tr,
			blobURL: blobURL,
			digest:  digest,
			timeout: timeout,
		}, size, nil
	}

	return nil, 0, errors.Wrapf(rErr, "cannot resolve layer")
}

type transport struct {
	inner http.RoundTripper
	auth  docker.Authorizer
	scope string
}

func (tr *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := docker.WithScope(req.Context(), tr.scope)
	roundTrip := func(req *http.Request) (*http.Response, error) {
		// authorize the request using docker.Authorizer
		if err := tr.auth.Authorize(ctx, req); err != nil {
			return nil, err
		}

		// send the request
		return tr.inner.RoundTrip(req)
	}

	resp, err := roundTrip(req)
	if err != nil {
		return nil, err
	}

	// TODO: support more status codes and retries
	if resp.StatusCode == http.StatusUnauthorized {
		log.G(ctx).Infof("Received status code: %v. Refreshing creds...", resp.Status)

		// prepare authorization for the target host using docker.Authorizer
		if err := tr.auth.AddResponses(ctx, []*http.Response{resp}); err != nil {
			if errdefs.IsNotImplemented(err) {
				return resp, nil
			}
			return nil, err
		}

		// re-authorize and send the request
		return roundTrip(req.Clone(ctx))
	}

	return resp, nil
}

func redirect(ctx context.Context, blobURL string, tr http.RoundTripper, timeout time.Duration) (url string, err error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	// We use GET request for redirect.
	// gcr.io returns 200 on HEAD without Location header (2020).
	// ghcr.io returns 200 on HEAD without Location header (2020).
	req, err := http.NewRequestWithContext(ctx, "GET", blobURL, nil)
	if err != nil {
		return "", errors.Wrapf(err, "failed to make request to the registry")
	}
	req.Close = false
	req.Header.Set("Range", "bytes=0-1")
	res, err := tr.RoundTrip(req)
	if err != nil {
		return "", errors.Wrapf(err, "failed to request")
	}
	defer func() {
		io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
	}()

	if res.StatusCode/100 == 2 {
		url = blobURL
	} else if redir := res.Header.Get("Location"); redir != "" && res.StatusCode/100 == 3 {
		// TODO: Support nested redirection
		url = redir
	} else {
		return "", fmt.Errorf("failed to access to the registry with code %v", res.StatusCode)
	}

	return
}

func getSize(ctx context.Context, url string, tr http.RoundTripper, timeout time.Duration) (int64, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return 0, err
	}
	req.Close = false
	res, err := tr.RoundTrip(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusOK {
		return strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
	}
	headStatusCode := res.StatusCode

	// Failed to do HEAD request. Fall back to GET.
	// ghcr.io (https://github-production-container-registry.s3.amazonaws.com) doesn't allow
	// HEAD request (2020).
	req, err = http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to make request to the registry")
	}
	req.Close = false
	req.Header.Set("Range", "bytes=0-1")
	res, err = tr.RoundTrip(req)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to request")
	}
	defer func() {
		io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
	}()

	if res.StatusCode == http.StatusOK {
		return strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
	} else if res.StatusCode == http.StatusPartialContent {
		_, size, err := parseRange(res.Header.Get("Content-Range"))
		return size, err
	}

	return 0, fmt.Errorf("failed to get size with code (HEAD=%v, GET=%v)",
		headStatusCode, res.StatusCode)
}

type httpFetcher struct {
	url           string
	urlMu         sync.Mutex
	tr            http.RoundTripper
	blobURL       string
	digest        digest.Digest
	singleRange   bool
	singleRangeMu sync.Mutex
	timeout       time.Duration
}

type multipartReadCloser interface {
	Next() (region, io.Reader, error)
	Close() error
}

func (f *httpFetcher) fetch(ctx context.Context, rs []region, retry bool) (multipartReadCloser, error) {
	if len(rs) == 0 {
		return nil, fmt.Errorf("no request queried")
	}

	var (
		tr              = f.tr
		singleRangeMode = f.isSingleRangeMode()
	)

	// squash requesting chunks for reducing the total size of request header
	// (servers generally have limits for the size of headers)
	// TODO: when our request has too many ranges, we need to divide it into
	//       multiple requests to avoid huge header.
	var s regionSet
	for _, reg := range rs {
		s.add(reg)
	}
	requests := s.rs
	if singleRangeMode {
		// Squash requests if the layer doesn't support multi range.
		requests = []region{superRegion(requests)}
	}

	// Request to the registry
	f.urlMu.Lock()
	url := f.url
	f.urlMu.Unlock()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	var ranges string
	for _, reg := range requests {
		ranges += fmt.Sprintf("%d-%d,", reg.b, reg.e)
	}
	req.Header.Add("Range", fmt.Sprintf("bytes=%s", ranges[:len(ranges)-1]))
	req.Header.Add("Accept-Encoding", "identity")
	req.Close = false

	// Recording the roundtrip latency for remote registry GET operation.
	start := time.Now()
	res, err := tr.RoundTrip(req) // NOT DefaultClient; don't want redirects
	commonmetrics.MeasureLatencyInMilliseconds(commonmetrics.RemoteRegistryGet, f.digest, start)
	if err != nil {
		return nil, err
	}
	if res.StatusCode == http.StatusOK {
		// We are getting the whole blob in one part (= status 200)
		size, err := strconv.ParseInt(res.Header.Get("Content-Length"), 10, 64)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse Content-Length")
		}
		return newSinglePartReader(region{0, size - 1}, res.Body), nil
	} else if res.StatusCode == http.StatusPartialContent {
		mediaType, params, err := mime.ParseMediaType(res.Header.Get("Content-Type"))
		if err != nil {
			return nil, errors.Wrapf(err, "invalid media type %q", mediaType)
		}
		if strings.HasPrefix(mediaType, "multipart/") {
			// We are getting a set of chunks as a multipart body.
			return newMultiPartReader(res.Body, params["boundary"]), nil
		}

		// We are getting single range
		reg, _, err := parseRange(res.Header.Get("Content-Range"))
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse Content-Range")
		}
		return newSinglePartReader(reg, res.Body), nil
	} else if retry && res.StatusCode == http.StatusForbidden {
		log.G(ctx).Infof("Received status code: %v. Refreshing URL and retrying...", res.Status)

		// re-redirect and retry this once.
		if err := f.refreshURL(ctx); err != nil {
			return nil, errors.Wrapf(err, "failed to refresh URL on %v", res.Status)
		}
		return f.fetch(ctx, rs, false)
	} else if retry && res.StatusCode == http.StatusBadRequest && !singleRangeMode {
		log.G(ctx).Infof("Received status code: %v. Setting single range mode and retrying...", res.Status)

		// gcr.io (https://storage.googleapis.com) returns 400 on multi-range request (2020 #81)
		f.singleRangeMode()            // fallbacks to singe range request mode
		return f.fetch(ctx, rs, false) // retries with the single range mode
	}

	return nil, fmt.Errorf("unexpected status code: %v", res.Status)
}

func (f *httpFetcher) check() error {
	ctx := context.Background()
	if f.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.timeout)
		defer cancel()
	}
	f.urlMu.Lock()
	url := f.url
	f.urlMu.Unlock()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return errors.Wrapf(err, "check failed: failed to make request")
	}
	req.Close = false
	req.Header.Set("Range", "bytes=0-1")
	res, err := f.tr.RoundTrip(req)
	if err != nil {
		return errors.Wrapf(err, "check failed: failed to request to registry")
	}
	defer func() {
		io.Copy(ioutil.Discard, res.Body)
		res.Body.Close()
	}()
	if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusPartialContent {
		return nil
	} else if res.StatusCode == http.StatusForbidden {
		// Try to re-redirect this blob
		rCtx := context.Background()
		if f.timeout > 0 {
			var rCancel context.CancelFunc
			rCtx, rCancel = context.WithTimeout(rCtx, f.timeout)
			defer rCancel()
		}
		if err := f.refreshURL(rCtx); err == nil {
			return nil
		}
		return fmt.Errorf("failed to refresh URL on status %v", res.Status)
	}

	return fmt.Errorf("unexpected status code %v", res.StatusCode)
}

func (f *httpFetcher) refreshURL(ctx context.Context) error {
	newURL, err := redirect(ctx, f.blobURL, f.tr, f.timeout)
	if err != nil {
		return err
	}
	f.urlMu.Lock()
	f.url = newURL
	f.urlMu.Unlock()
	return nil
}

func (f *httpFetcher) genID(reg region) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s-%d-%d", f.blobURL, reg.b, reg.e)))
	return fmt.Sprintf("%x", sum)
}

func (f *httpFetcher) singleRangeMode() {
	f.singleRangeMu.Lock()
	f.singleRange = true
	f.singleRangeMu.Unlock()
}

func (f *httpFetcher) isSingleRangeMode() bool {
	f.singleRangeMu.Lock()
	r := f.singleRange
	f.singleRangeMu.Unlock()
	return r
}

func newSinglePartReader(reg region, rc io.ReadCloser) multipartReadCloser {
	return &singlepartReader{
		r:      rc,
		Closer: rc,
		reg:    reg,
	}
}

type singlepartReader struct {
	io.Closer
	r      io.Reader
	reg    region
	called bool
}

func (sr *singlepartReader) Next() (region, io.Reader, error) {
	if !sr.called {
		sr.called = true
		return sr.reg, sr.r, nil
	}
	return region{}, nil, io.EOF
}

func newMultiPartReader(rc io.ReadCloser, boundary string) multipartReadCloser {
	return &multipartReader{
		m:      multipart.NewReader(rc, boundary),
		Closer: rc,
	}
}

type multipartReader struct {
	io.Closer
	m *multipart.Reader
}

func (sr *multipartReader) Next() (region, io.Reader, error) {
	p, err := sr.m.NextPart()
	if err != nil {
		return region{}, nil, err
	}
	reg, _, err := parseRange(p.Header.Get("Content-Range"))
	if err != nil {
		return region{}, nil, errors.Wrapf(err, "failed to parse Content-Range")
	}
	return reg, p, nil
}

func parseRange(header string) (region, int64, error) {
	submatches := contentRangeRegexp.FindStringSubmatch(header)
	if len(submatches) < 4 {
		return region{}, 0, fmt.Errorf("Content-Range %q doesn't have enough information", header)
	}
	begin, err := strconv.ParseInt(submatches[1], 10, 64)
	if err != nil {
		return region{}, 0, errors.Wrapf(err, "failed to parse beginning offset %q", submatches[1])
	}
	end, err := strconv.ParseInt(submatches[2], 10, 64)
	if err != nil {
		return region{}, 0, errors.Wrapf(err, "failed to parse end offset %q", submatches[2])
	}
	blobSize, err := strconv.ParseInt(submatches[3], 10, 64)
	if err != nil {
		return region{}, 0, errors.Wrapf(err, "failed to parse blob size %q", submatches[3])
	}

	return region{begin, end}, blobSize, nil
}

type Option func(*options)

type options struct {
	ctx       context.Context
	cacheOpts []cache.Option
}

func WithContext(ctx context.Context) Option {
	return func(opts *options) {
		opts.ctx = ctx
	}
}

func WithCacheOpts(cacheOpts ...cache.Option) Option {
	return func(opts *options) {
		opts.cacheOpts = cacheOpts
	}
}

// NOTE: ported from https://github.com/containerd/containerd/blob/v1.5.2/remotes/docker/scope.go#L29-L42
// TODO: import this from containerd package once we drop support to continerd v1.4.x
//
// repositoryScope returns a repository scope string such as "repository:foo/bar:pull"
// for "host/foo/bar:baz".
// When push is true, both pull and push are added to the scope.
func repositoryScope(refspec reference.Spec, push bool) (string, error) {
	u, err := url.Parse("dummy://" + refspec.Locator)
	if err != nil {
		return "", err
	}
	s := "repository:" + strings.TrimPrefix(u.Path, "/") + ":pull"
	if push {
		s += ",push"
	}
	return s, nil
}

type remoteFetcher struct {
	r Fetcher
}

func (r *remoteFetcher) fetch(ctx context.Context, rs []region, retry bool) (multipartReadCloser, error) {
	var s regionSet
	for _, reg := range rs {
		s.add(reg)
	}
	reg := superRegion(s.rs)
	rc, err := r.r.Fetch(ctx, reg.b, reg.size())
	if err != nil {
		return nil, err
	}
	return newSinglePartReader(reg, rc), nil
}

func (r *remoteFetcher) check() error {
	return r.r.Check()
}

func (r *remoteFetcher) genID(reg region) string {
	return r.r.GenID(reg.b, reg.size())
}

type Handler interface {
	Handle(ctx context.Context, desc ocispec.Descriptor) (fetcher Fetcher, size int64, err error)
}

type Fetcher interface {
	Fetch(ctx context.Context, off int64, size int64) (io.ReadCloser, error)
	Check() error
	GenID(off int64, size int64) string
}
