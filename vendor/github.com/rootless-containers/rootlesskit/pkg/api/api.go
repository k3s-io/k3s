package api

// ErrorJSON is returned with "application/json" content type and non-2XX status code
type ErrorJSON struct {
	Message string `json:"message"`
}
