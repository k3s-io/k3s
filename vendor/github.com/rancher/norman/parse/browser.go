package parse

import (
	"net/http"
	"strings"
)

func IsBrowser(req *http.Request, checkAccepts bool) bool {
	accepts := strings.ToLower(req.Header.Get("Accept"))
	userAgent := strings.ToLower(req.Header.Get("User-Agent"))

	if accepts == "" || !checkAccepts {
		accepts = "*/*"
	}

	// User agent has Mozilla and browser accepts */*
	return strings.Contains(userAgent, "mozilla") && strings.Contains(accepts, "*/*")
}
