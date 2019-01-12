package parse

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"sort"

	"github.com/rancher/norman/api/builtin"
	"github.com/rancher/norman/httperror"
	"github.com/rancher/norman/types"
	"github.com/rancher/norman/urlbuilder"
)

const (
	maxFormSize = 2 * 1 << 20
)

var (
	multiSlashRegexp = regexp.MustCompile("//+")
	allowedFormats   = map[string]bool{
		"html": true,
		"json": true,
		"yaml": true,
	}
)

type ParsedURL struct {
	Version          *types.APIVersion
	SchemasVersion   *types.APIVersion
	Type             string
	ID               string
	Link             string
	Method           string
	Action           string
	SubContext       map[string]string
	SubContextPrefix string
	Query            url.Values
}

type ResolverFunc func(typeName string, context *types.APIContext) error

type URLParser func(schema *types.Schemas, url *url.URL) (ParsedURL, error)

func DefaultURLParser(schemas *types.Schemas, url *url.URL) (ParsedURL, error) {
	result := ParsedURL{}

	path := url.EscapedPath()
	path = multiSlashRegexp.ReplaceAllString(path, "/")
	schemaVersion, version, prefix, parts, subContext := parseVersionAndSubContext(schemas, path)

	if version == nil {
		return result, nil
	}

	result.Version = version
	result.SchemasVersion = schemaVersion
	result.SubContext = subContext
	result.SubContextPrefix = prefix
	result.Action, result.Method = parseAction(url)
	result.Query = url.Query()

	result.Type = safeIndex(parts, 0)
	result.ID = safeIndex(parts, 1)
	result.Link = safeIndex(parts, 2)

	return result, nil
}

func Parse(rw http.ResponseWriter, req *http.Request, schemas *types.Schemas, urlParser URLParser, resolverFunc ResolverFunc) (*types.APIContext, error) {
	var err error

	result := types.NewAPIContext(req, rw, schemas)
	result.Method = parseMethod(req)
	result.ResponseFormat = parseResponseFormat(req)
	result.URLBuilder, _ = urlbuilder.New(req, types.APIVersion{}, schemas)

	// The response format is guarenteed to be set even in the event of an error
	parsedURL, err := urlParser(schemas, req.URL)
	// wait to check error, want to set as much as possible

	result.SubContext = parsedURL.SubContext
	result.Type = parsedURL.Type
	result.ID = parsedURL.ID
	result.Link = parsedURL.Link
	result.Action = parsedURL.Action
	result.Query = parsedURL.Query
	if parsedURL.Method != "" {
		result.Method = parsedURL.Method
	}

	result.Version = parsedURL.Version
	result.SchemasVersion = parsedURL.SchemasVersion

	if err != nil {
		return result, err
	}

	if result.Version == nil {
		result.Method = http.MethodGet
		result.URLBuilder, err = urlbuilder.New(req, types.APIVersion{}, result.Schemas)
		result.Type = "apiRoot"
		result.Schema = result.Schemas.Schema(&builtin.Version, "apiRoot")
		return result, nil
	}

	result.URLBuilder, err = urlbuilder.New(req, *result.Version, result.Schemas)
	if err != nil {
		return result, err
	}

	if parsedURL.SubContextPrefix != "" {
		result.URLBuilder.SetSubContext(parsedURL.SubContextPrefix)
	}

	if err := resolverFunc(result.Type, result); err != nil {
		return result, err
	}

	if result.Schema == nil {
		if result.Type != "" {
			err = httperror.NewAPIError(httperror.NotFound, "failed to find schema "+result.Type)
		}
		result.Method = http.MethodGet
		result.Type = "apiRoot"
		result.Schema = result.Schemas.Schema(&builtin.Version, "apiRoot")
		result.ID = result.Version.Path
		return result, err
	}

	result.Type = result.Schema.ID

	if err := ValidateMethod(result); err != nil {
		return result, err
	}

	return result, nil
}

func versionsForPath(schemas *types.Schemas, path string) []types.APIVersion {
	var matchedVersion []types.APIVersion
	for _, version := range schemas.Versions() {
		if strings.HasPrefix(path, version.Path) {
			afterPath := path[len(version.Path):]
			// if version.Path is /v3/cluster allow /v3/clusters but not /v3/clusterstuff
			if len(afterPath) < 3 || strings.Contains(afterPath[:3], "/") {
				matchedVersion = append(matchedVersion, version)
			}
		}
	}
	sort.Slice(matchedVersion, func(i, j int) bool {
		return len(matchedVersion[i].Path) > len(matchedVersion[j].Path)
	})
	return matchedVersion
}

func parseVersionAndSubContext(schemas *types.Schemas, escapedPath string) (*types.APIVersion, *types.APIVersion, string, []string, map[string]string) {
	versions := versionsForPath(schemas, escapedPath)
	if len(versions) == 0 {
		return nil, nil, "", nil, nil
	}
	version := &versions[0]

	if strings.HasSuffix(escapedPath, "/") {
		escapedPath = escapedPath[:len(escapedPath)-1]
	}

	versionParts := strings.Split(version.Path, "/")
	pp := strings.Split(escapedPath, "/")
	var pathParts []string
	for _, p := range pp {
		part, err := url.PathUnescape(p)
		if err == nil {
			pathParts = append(pathParts, part)
		} else {
			pathParts = append(pathParts, p)
		}
	}

	paths := pathParts[len(versionParts):]

	if !version.SubContext || len(versions) < 2 {
		return nil, version, "", paths, nil
	}

	// Handle the special case of /v3/clusters/schema(s)
	if len(paths) >= 1 && (paths[0] == "schema" || paths[0] == "schemas") {
		return nil, version, "", paths, nil
	}

	if len(paths) < 2 {
		// Handle case like /v3/clusters/foo where /v3 and /v3/clusters are API versions.
		// In this situation you want the version to be /v3 and the path "clusters", "foo"
		newVersion := versions[0]
		if len(paths) > 0 {
			newVersion.Path = newVersion.Path + "/" + paths[0]
		}
		return &newVersion, &versions[1], "", pathParts[len(versionParts)-1:], nil
	}

	// Length is always >= 3

	attrs := map[string]string{
		version.SubContextSchema: paths[0],
	}

	for i, version := range versions {
		schema := schemas.Schema(&version, paths[1])
		if schema != nil {
			if i == 0 {
				break
			}
			return nil, &version, "", paths[1:], attrs
		}
	}

	return nil, version, "/" + paths[0], paths[1:], attrs
}

func DefaultResolver(typeName string, apiContext *types.APIContext) error {
	if typeName == "" {
		return nil
	}

	schema := apiContext.Schemas.Schema(apiContext.Version, typeName)
	if schema == nil && (typeName == builtin.Schema.ID || typeName == builtin.Schema.PluralName) {
		// Schemas are special, we include it as though part of the API request version
		schema = apiContext.Schemas.Schema(&builtin.Version, typeName)
	}
	if schema == nil {
		return nil
	}

	apiContext.Schema = schema
	return nil
}

func safeIndex(slice []string, index int) string {
	if index >= len(slice) {
		return ""
	}
	return slice[index]
}

func parseResponseFormat(req *http.Request) string {
	format := req.URL.Query().Get("_format")

	if format != "" {
		format = strings.TrimSpace(strings.ToLower(format))
	}

	/* Format specified */
	if allowedFormats[format] {
		return format
	}

	// User agent has Mozilla and browser accepts */*
	if IsBrowser(req, true) {
		return "html"
	}

	if isYaml(req) {
		return "yaml"
	}
	return "json"
}

func isYaml(req *http.Request) bool {
	return strings.Contains(req.Header.Get("Accept"), "application/yaml")
}

func parseMethod(req *http.Request) string {
	method := req.URL.Query().Get("_method")
	if method == "" {
		method = req.Method
	}
	return method
}

func parseAction(url *url.URL) (string, string) {
	action := url.Query().Get("action")
	if action == "remove" {
		return "", http.MethodDelete
	}

	return action, ""
}

func Body(req *http.Request) (map[string]interface{}, error) {
	req.ParseMultipartForm(maxFormSize)
	if req.MultipartForm != nil {
		return valuesToBody(req.MultipartForm.Value), nil
	}

	if req.PostForm != nil && len(req.PostForm) > 0 {
		return valuesToBody(map[string][]string(req.Form)), nil
	}

	return ReadBody(req)
}

func valuesToBody(input map[string][]string) map[string]interface{} {
	result := map[string]interface{}{}
	for k, v := range input {
		result[k] = v
	}
	return result
}
