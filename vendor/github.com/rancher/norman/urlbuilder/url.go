package urlbuilder

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/rancher/norman/name"
	"github.com/rancher/norman/types"
)

const (
	PrefixHeader         = "X-API-URL-Prefix"
	ForwardedHostHeader  = "X-Forwarded-Host"
	ForwardedProtoHeader = "X-Forwarded-Proto"
	ForwardedPortHeader  = "X-Forwarded-Port"
)

func New(r *http.Request, version types.APIVersion, schemas *types.Schemas) (types.URLBuilder, error) {
	requestURL := parseRequestURL(r)
	responseURLBase, err := parseResponseURLBase(requestURL, r)
	if err != nil {
		return nil, err
	}

	builder := &urlBuilder{
		schemas:         schemas,
		requestURL:      requestURL,
		responseURLBase: responseURLBase,
		apiVersion:      version,
		query:           r.URL.Query(),
	}

	return builder, nil
}

type urlBuilder struct {
	schemas         *types.Schemas
	requestURL      string
	responseURLBase string
	apiVersion      types.APIVersion
	subContext      string
	query           url.Values
}

func (u *urlBuilder) SetSubContext(subContext string) {
	u.subContext = subContext
}

func (u *urlBuilder) SchemaLink(schema *types.Schema) string {
	return u.constructBasicURL(schema.Version, "schemas", schema.ID)
}

func (u *urlBuilder) Link(linkName string, resource *types.RawResource) string {
	if resource.ID == "" || linkName == "" {
		return ""
	}

	if self, ok := resource.Links["self"]; ok {
		return self + "/" + strings.ToLower(linkName)
	}

	return u.constructBasicURL(resource.Schema.Version, resource.Schema.PluralName, resource.ID, strings.ToLower(linkName))
}

func (u *urlBuilder) ResourceLink(resource *types.RawResource) string {
	if resource.ID == "" {
		return ""
	}

	return u.constructBasicURL(resource.Schema.Version, resource.Schema.PluralName, resource.ID)
}

func (u *urlBuilder) Marker(marker string) string {
	newValues := url.Values{}
	for k, v := range u.query {
		newValues[k] = v
	}
	newValues.Set("marker", marker)
	return u.requestURL + "?" + newValues.Encode()
}

func (u *urlBuilder) ReverseSort(order types.SortOrder) string {
	newValues := url.Values{}
	for k, v := range u.query {
		newValues[k] = v
	}
	newValues.Del("order")
	newValues.Del("marker")
	if order == types.ASC {
		newValues.Add("order", string(types.DESC))
	} else {
		newValues.Add("order", string(types.ASC))
	}

	return u.requestURL + "?" + newValues.Encode()
}

func (u *urlBuilder) Current() string {
	return u.requestURL
}

func (u *urlBuilder) RelativeToRoot(path string) string {
	return u.responseURLBase + path
}

func (u *urlBuilder) Sort(field string) string {
	newValues := url.Values{}
	for k, v := range u.query {
		newValues[k] = v
	}
	newValues.Del("order")
	newValues.Del("marker")
	newValues.Set("sort", field)
	return u.requestURL + "?" + newValues.Encode()
}

func (u *urlBuilder) Collection(schema *types.Schema, versionOverride *types.APIVersion) string {
	plural := u.getPluralName(schema)
	if versionOverride == nil {
		return u.constructBasicURL(schema.Version, plural)
	}
	return u.constructBasicURL(*versionOverride, plural)
}

func (u *urlBuilder) SubContextCollection(subContext *types.Schema, contextName string, schema *types.Schema) string {
	return u.constructBasicURL(subContext.Version, subContext.PluralName, contextName, u.getPluralName(schema))
}

func (u *urlBuilder) Version(version types.APIVersion) string {
	return u.constructBasicURL(version)
}

func (u *urlBuilder) FilterLink(schema *types.Schema, fieldName string, value string) string {
	return u.constructBasicURL(schema.Version, schema.PluralName) + "?" +
		url.QueryEscape(fieldName) + "=" + url.QueryEscape(value)
}

func (u *urlBuilder) ResourceLinkByID(schema *types.Schema, id string) string {
	return u.constructBasicURL(schema.Version, schema.PluralName, id)
}

func (u *urlBuilder) constructBasicURL(version types.APIVersion, parts ...string) string {
	buffer := bytes.Buffer{}

	buffer.WriteString(u.responseURLBase)
	if version.Path == "" {
		buffer.WriteString(u.apiVersion.Path)
	} else {
		buffer.WriteString(version.Path)
	}
	buffer.WriteString(u.subContext)

	for _, part := range parts {
		if part == "" {
			return ""
		}
		buffer.WriteString("/")
		buffer.WriteString(part)
	}

	return buffer.String()
}

func (u *urlBuilder) getPluralName(schema *types.Schema) string {
	if schema.PluralName == "" {
		return strings.ToLower(name.GuessPluralName(schema.ID))
	}
	return strings.ToLower(schema.PluralName)
}

// Constructs the request URL based off of standard headers in the request, falling back to the HttpServletRequest.getRequestURL()
// if the headers aren't available. Here is the ordered list of how we'll attempt to construct the URL:
//  - x-forwarded-proto://x-forwarded-host:x-forwarded-port/HttpServletRequest.getRequestURI()
//  - x-forwarded-proto://x-forwarded-host/HttpServletRequest.getRequestURI()
//  - x-forwarded-proto://host:x-forwarded-port/HttpServletRequest.getRequestURI()
//  - x-forwarded-proto://host/HttpServletRequest.getRequestURI() request.getRequestURL()
//
// Additional notes:
//  - If the x-forwarded-host/host header has a port and x-forwarded-port has been passed, x-forwarded-port will be used.
func parseRequestURL(r *http.Request) string {
	// Get url from standard headers
	requestURL := getURLFromStandardHeaders(r)
	if requestURL != "" {
		return requestURL
	}

	// Use incoming url
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s%s%s", scheme, r.Host, r.Header.Get(PrefixHeader), r.URL.Path)
}

func getURLFromStandardHeaders(r *http.Request) string {
	xForwardedProto := getOverrideHeader(r, ForwardedProtoHeader, "")
	if xForwardedProto == "" {
		return ""
	}

	host := getOverrideHeader(r, ForwardedHostHeader, "")
	if host == "" {
		host = r.Host
	}

	if host == "" {
		return ""
	}

	port := getOverrideHeader(r, ForwardedPortHeader, "")
	if port == "443" || port == "80" {
		port = "" // Don't include default ports in url
	}

	if port != "" && strings.Contains(host, ":") {
		// Have to strip the port that is in the host. Handle IPv6, which has this format: [::1]:8080
		if (strings.HasPrefix(host, "[") && strings.Contains(host, "]:")) || !strings.HasPrefix(host, "[") {
			host = host[0:strings.LastIndex(host, ":")]
		}
	}

	if port != "" {
		port = ":" + port
	}

	return fmt.Sprintf("%s://%s%s%s%s", xForwardedProto, host, port, r.Header.Get(PrefixHeader), r.URL.Path)
}

func getOverrideHeader(r *http.Request, header string, defaultValue string) string {
	// Need to handle comma separated hosts in X-Forwarded-For
	value := r.Header.Get(header)
	if value != "" {
		return strings.TrimSpace(strings.Split(value, ",")[0])
	}
	return defaultValue
}

func parseResponseURLBase(requestURL string, r *http.Request) (string, error) {
	path := r.URL.Path

	index := strings.LastIndex(requestURL, path)
	if index == -1 {
		// Fallback, if we can't find path in requestURL, then we just assume the base is the root of the web request
		u, err := url.Parse(requestURL)
		if err != nil {
			return "", err
		}

		buffer := bytes.Buffer{}
		buffer.WriteString(u.Scheme)
		buffer.WriteString("://")
		buffer.WriteString(u.Host)
		return buffer.String(), nil
	}

	return requestURL[0:index], nil
}

func (u *urlBuilder) Action(action string, resource *types.RawResource) string {
	return u.constructBasicURL(resource.Schema.Version, resource.Schema.PluralName, resource.ID) + "?action=" + url.QueryEscape(action)
}

func (u *urlBuilder) CollectionAction(schema *types.Schema, versionOverride *types.APIVersion, action string) string {
	collectionURL := u.Collection(schema, versionOverride)
	return collectionURL + "?action=" + url.QueryEscape(action)
}

func (u *urlBuilder) ActionLinkByID(schema *types.Schema, id string, action string) string {
	return u.constructBasicURL(schema.Version, schema.PluralName, id) + "?action=" + url.QueryEscape(action)
}
