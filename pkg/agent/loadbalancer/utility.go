package loadbalancer

import (
	"errors"
	"net/url"
	"sort"
	"strings"
)

func parseURL(serverURL, newHost string) (string, string, error) {
	var address strings.Builder
	parsedURL, err := url.Parse(serverURL)
	if err != nil {
		return "", "", err
	}
	if parsedURL.Host == "" {
		return "", "", errors.New("Initial server URL host is not defined for load balancer")
	}
	address.WriteString(parsedURL.Host)
	if parsedURL.Port() == "" {
		if strings.ToLower(parsedURL.Scheme) == "http" {
			address.WriteString(":80")
		}
		if strings.ToLower(parsedURL.Scheme) == "https" {
			address.WriteString(":443")
		}
	}
	parsedURL.Host = newHost
	return address.String(), parsedURL.String(), nil
}

func sortServers(input []string, search string) ([]string, bool) {
	result := []string{}
	found := false
	skip := map[string]bool{"": true}

	for _, entry := range input {
		if skip[entry] {
			continue
		}
		if search == entry {
			found = true
		}
		skip[entry] = true
		result = append(result, entry)
	}

	sort.Strings(result)
	return result, found
}
