/*
Copyright 2019 The Kubernetes Authors.

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

package handlers

import (
	"net/http"

	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/apiserver/pkg/endpoints/request"
)

const (
	maxUserAgentLength      = 1024
	userAgentTruncateSuffix = "...TRUNCATED"
)

// lazyTruncatedUserAgent implements String() string and it will
// return user-agent which may be truncated.
type lazyTruncatedUserAgent struct {
	req *http.Request
}

func (lazy *lazyTruncatedUserAgent) String() string {
	ua := "unknown"
	if lazy.req != nil {
		ua = utilnet.GetHTTPClient(lazy.req)
		if len(ua) > maxUserAgentLength {
			ua = ua[:maxUserAgentLength] + userAgentTruncateSuffix
		}
	}
	return ua
}

// LazyClientIP implements String() string and it will
// calls GetClientIP() lazily only when required.
type lazyClientIP struct {
	req *http.Request
}

func (lazy *lazyClientIP) String() string {
	if lazy.req != nil {
		if ip := utilnet.GetClientIP(lazy.req); ip != nil {
			return ip.String()
		}
	}
	return "unknown"
}

// lazyAccept implements String() string and it will
// calls http.Request Header.Get() lazily only when required.
type lazyAccept struct {
	req *http.Request
}

func (lazy *lazyAccept) String() string {
	if lazy.req != nil {
		accept := lazy.req.Header.Get("Accept")
		return accept
	}

	return "unknown"
}

// lazyAuditID implements Stringer interface to lazily retrieve
// the audit ID associated with the request.
type lazyAuditID struct {
	req *http.Request
}

func (lazy *lazyAuditID) String() string {
	if lazy.req != nil {
		return request.GetAuditIDTruncated(lazy.req)
	}

	return "unknown"
}
